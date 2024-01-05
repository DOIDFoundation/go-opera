package testnet

import (
	"math/big"
	"time"

	"github.com/Fantom-foundation/go-opera/integration/makefakegenesis"
	"github.com/Fantom-foundation/go-opera/integration/makegenesis"
	"github.com/Fantom-foundation/go-opera/inter"
	"github.com/Fantom-foundation/go-opera/inter/drivertype"
	"github.com/Fantom-foundation/go-opera/inter/iblockproc"
	"github.com/Fantom-foundation/go-opera/inter/ier"
	"github.com/Fantom-foundation/go-opera/opera"
	"github.com/Fantom-foundation/go-opera/opera/contracts/driver"
	"github.com/Fantom-foundation/go-opera/opera/contracts/driver/drivercall"
	"github.com/Fantom-foundation/go-opera/opera/contracts/driverauth"
	"github.com/Fantom-foundation/go-opera/opera/contracts/evmwriter"
	"github.com/Fantom-foundation/go-opera/opera/contracts/netinit"
	"github.com/Fantom-foundation/go-opera/opera/contracts/sfc"
	"github.com/Fantom-foundation/go-opera/opera/contracts/sfclib"
	"github.com/Fantom-foundation/go-opera/opera/genesis"
	"github.com/Fantom-foundation/go-opera/opera/genesisstore"
	"github.com/Fantom-foundation/go-opera/utils"
	"github.com/Fantom-foundation/lachesis-base/hash"
	"github.com/Fantom-foundation/lachesis-base/inter/idx"
	"github.com/Fantom-foundation/lachesis-base/inter/pos"
	"github.com/Fantom-foundation/lachesis-base/kvdb/memorydb"
	"github.com/Fantom-foundation/lachesis-base/lachesis"
	"github.com/ethereum/go-ethereum/common"
)

var testnetValidators = []string{
	"04cbcc8aa3c7314d5cc58cc44d4160658479c9b73764880b949bbc1d156eb95b0b3ddf23d326bed2982a844c6fbd563cc27f6009fe96e0637f184260b51e0f5d7c",
	"04ed0273361cec3adbeb45b89f645c9c0fd2e4230fcb285a33d289816d12dbf8b6472587d290dc9598206d6a6a1f7f8f25b7c516f4e563dd3867571dc8a57bb7ff",
	"041ad375419a202e5ec885db53447291ac0bc3eba1a5743c709359c9db3479b6d792776e6e48dc75a2ad922c73d56d05a8a94168a9307974ed207aa301fcdea282",
}
var TestNetGenesisTime = inter.Timestamp(1703800000 * time.Second)

func TestNetGenesisStore() *genesisstore.Store {
	return TestNetGenesisStoreWithRulesAndStart(opera.TestNetRules(), 2, 1)
}

func TestNetGenesisStoreWithRulesAndStart(rules opera.Rules, epoch idx.Epoch, block idx.Block) *genesisstore.Store {
	builder := makegenesis.NewGenesisBuilder(memorydb.NewProducer(""))

	validators := makegenesis.GetValidators(testnetValidators, TestNetGenesisTime)

	// add balances to validators
	var delegations []drivercall.Delegation
	for _, val := range validators {
		builder.AddBalance(val.Address, utils.ToFtm(10_000_000))
		delegations = append(delegations, drivercall.Delegation{
			Address:            val.Address,
			ValidatorID:        val.ID,
			Stake:              utils.ToFtm(5_000_000),
			LockedStake:        new(big.Int),
			LockupFromEpoch:    0,
			LockupEndTime:      0,
			LockupDuration:     0,
			EarlyUnlockPenalty: new(big.Int),
			Rewards:            new(big.Int),
		})
	}

	// deploy essential contracts
	// pre deploy NetworkInitializer
	builder.SetCode(netinit.ContractAddress, netinit.GetContractBin())
	// pre deploy NodeDriver
	builder.SetCode(driver.ContractAddress, driver.GetContractBin())
	// pre deploy NodeDriverAuth
	builder.SetCode(driverauth.ContractAddress, driverauth.GetContractBin())
	// pre deploy SFC
	builder.SetCode(sfc.ContractAddress, sfc.GetContractBin())
	// pre deploy SFCLib
	builder.SetCode(sfclib.ContractAddress, sfclib.GetContractBin())
	// set non-zero code for pre-compiled contracts
	builder.SetCode(evmwriter.ContractAddress, []byte{0})

	builder.SetCurrentEpoch(ier.LlrIdxFullEpochRecord{
		LlrFullEpochRecord: ier.LlrFullEpochRecord{
			BlockState: iblockproc.BlockState{
				LastBlock: iblockproc.BlockCtx{
					Idx:     block - 1,
					Time:    TestNetGenesisTime,
					Atropos: hash.Event{},
				},
				FinalizedStateRoot:    hash.Hash{},
				EpochGas:              0,
				EpochCheaters:         lachesis.Cheaters{},
				CheatersWritten:       0,
				ValidatorStates:       make([]iblockproc.ValidatorBlockState, 0),
				NextValidatorProfiles: make(map[idx.ValidatorID]drivertype.Validator),
				DirtyRules:            nil,
				AdvanceEpochs:         0,
			},
			EpochState: iblockproc.EpochState{
				Epoch:             epoch - 1,
				EpochStart:        TestNetGenesisTime,
				PrevEpochStart:    TestNetGenesisTime - 1,
				EpochStateRoot:    hash.Zero,
				Validators:        pos.NewBuilder().Build(),
				ValidatorStates:   make([]iblockproc.ValidatorEpochState, 0),
				ValidatorProfiles: make(map[idx.ValidatorID]drivertype.Validator),
				Rules:             rules,
			},
		},
		Idx: epoch - 1,
	})

	var owner = common.Address{175, 178, 225, 20, 95, 26, 136, 206, 72, 157, 34, 66, 90, 200, 64, 3, 254, 80, 179, 190}
	builder.AddBalance(owner, utils.ToFtm(1_000_000_000))

	blockProc := makegenesis.DefaultBlockProc()
	genesisTxs := makefakegenesis.GetGenesisTxs(epoch-2, validators, builder.TotalSupply(), delegations, owner)
	err := builder.ExecuteGenesisTxs(blockProc, genesisTxs)
	if err != nil {
		panic(err)
	}

	return builder.Build(genesis.Header{
		GenesisID:   builder.CurrentHash(),
		NetworkID:   rules.NetworkID,
		NetworkName: rules.Name,
	})
}
