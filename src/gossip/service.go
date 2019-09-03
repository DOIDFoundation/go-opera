package gossip

import (
	"crypto/ecdsa"
	"fmt"
	"math/rand"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	notify "github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/discv5"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/Fantom-foundation/go-lachesis/src/evm_core"
	"github.com/Fantom-foundation/go-lachesis/src/inter"
	"github.com/Fantom-foundation/go-lachesis/src/inter/idx"
	"github.com/Fantom-foundation/go-lachesis/src/logger"
)

type ServiceFeed struct {
	newEpoch        notify.Feed
	newPack         notify.Feed
	newEmittedEvent notify.Feed
	newBlock        notify.Feed
	scope           notify.SubscriptionScope
}

func (f *ServiceFeed) SubscribeNewEpoch(ch chan<- idx.Epoch) notify.Subscription {
	return f.scope.Track(f.newEpoch.Subscribe(ch))
}

func (f *ServiceFeed) SubscribeNewPack(ch chan<- idx.Pack) notify.Subscription {
	return f.scope.Track(f.newPack.Subscribe(ch))
}

func (f *ServiceFeed) SubscribeNewBlock(ch chan<- evm_core.ChainHeadNotify) notify.Subscription {
	return f.scope.Track(f.newBlock.Subscribe(ch))
}

func (f *ServiceFeed) SubscribeNewEmitted(ch chan<- *inter.Event) notify.Subscription {
	return f.scope.Track(f.newEmittedEvent.Subscribe(ch))
}

// Service implements go-ethereum/node.Service interface.
type Service struct {
	config Config

	wg   sync.WaitGroup
	done chan struct{}

	// server
	Name   string
	Topics []discv5.Topic

	serverPool *serverPool

	// my identity
	me         common.Address
	privateKey *ecdsa.PrivateKey

	// application
	store    *Store
	engine   Consensus
	engineMu *sync.RWMutex
	emitter  *Emitter
	txpool   txPool

	feed ServiceFeed

	// application protocol
	pm *ProtocolManager

	logger.Instance
}

func NewService(config Config, store *Store, engine Consensus) (*Service, error) {
	svc := &Service{
		config: config,

		done: make(chan struct{}),

		Name: fmt.Sprintf("Node-%d", rand.Int()),

		store: store,

		engineMu: new(sync.RWMutex),

		Instance: logger.MakeInstance(),
	}

	engine = &HookedEngine{
		engine:       engine,
		processEvent: svc.processEvent,
	}
	svc.engine = engine

	engine.Bootstrap(svc.ApplyBlock)

	trustedNodes := []string{}

	svc.serverPool = newServerPool(store.table.Peers, svc.done, &svc.wg, trustedNodes)

	svc.txpool = evm_core.NewTxPool(config.Net.TxPool, params.AllEthashProtocolChanges, svc.GetEvmStateReader())

	var err error
	svc.pm, err = NewProtocolManager(&config, &svc.feed, svc.txpool, svc.engineMu, store, engine)

	return svc, err
}

func (s *Service) processEvent(realEngine Consensus, e *inter.Event) error {
	// s.engineMu is locked here

	if s.store.HasEvent(e.Hash()) { // sanity check
		s.store.Fatalf("ProcessEvent: event is already processed %s", e.Hash().String())
	}

	oldEpoch := e.Epoch

	s.store.SetEvent(e)
	if realEngine != nil {
		err := realEngine.ProcessEvent(e)
		if err != nil { // TODO make it possible to write only on success
			s.store.DeleteEvent(e.Epoch, e.Hash())
			return err
		}
	}
	// set member's last event. we don't care about forks, because this index is used only for emitter
	s.store.SetLastEvent(e.Epoch, e.Creator, e.Hash())

	// track events with no descendants, i.e. heads
	for _, parent := range e.Parents {
		if s.store.IsHead(e.Epoch, parent) {
			s.store.DelHead(e.Epoch, parent)
		}
	}
	s.store.AddHead(e.Epoch, e.Hash())

	s.packs_onNewEvent(e, e.Epoch)

	newEpoch := realEngine.GetEpoch()
	if newEpoch != oldEpoch {
		s.packs_onNewEpoch(oldEpoch, newEpoch)
		s.store.delEpochStore(oldEpoch)
		s.feed.newEpoch.Send(newEpoch)
	}

	return nil
}

func (s *Service) makeEmitter() *Emitter {
	return NewEmitter(&s.config, s.me, s.privateKey, s.engineMu, s.store, s.txpool, s.engine, func(emitted *inter.Event) {
		// s.engineMu is locked here

		err := s.engine.ProcessEvent(emitted)
		if err != nil {
			s.Fatalf("Self-event connection failed: %s", err.Error())
		}

		s.feed.newEmittedEvent.Send(emitted) // PM listens and will broadcast it
		if err != nil {
			s.Fatalf("Failed to post self-event: %s", err.Error())
		}
	},
	)
}

// Protocols returns protocols the service can communicate on.
func (s *Service) Protocols() []p2p.Protocol {
	protos := make([]p2p.Protocol, len(ProtocolVersions))
	for i, vsn := range ProtocolVersions {
		protos[i] = s.pm.makeProtocol(vsn)
		protos[i].Attributes = []enr.Entry{s.currentEnr()}
	}
	return protos
}

// APIs returns api methods the service wants to expose on rpc channels.
func (s *Service) APIs() []rpc.API {
	return []rpc.API{}
}

// Start method invoked when the node is ready to start the service.
func (s *Service) Start(srv *p2p.Server) error {

	var genesis common.Hash
	genesis = s.engine.GetGenesisHash()
	s.Topics = []discv5.Topic{
		discv5.Topic("lachesis@" + genesis.Hex()),
	}

	if srv.DiscV5 != nil {
		for _, topic := range s.Topics {
			topic := topic
			go func() {
				s.Info("Starting topic registration")
				defer s.Info("Terminated topic registration")

				srv.DiscV5.RegisterTopic(topic, s.done)
			}()
		}
	}
	s.privateKey = srv.PrivateKey
	s.me = crypto.PubkeyToAddress(s.privateKey.PublicKey)

	s.pm.Start(srv.MaxPeers)

	s.emitter = s.makeEmitter()

	return nil
}

// Stop method invoked when the node terminates the service.
func (s *Service) Stop() error {
	fmt.Println("Service stopping...")
	s.pm.Stop()
	s.wg.Wait()
	s.feed.scope.Close()
	return nil
}
