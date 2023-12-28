package makegenesis

import (
	"encoding/hex"

	"github.com/Fantom-foundation/go-opera/inter"
	"github.com/Fantom-foundation/go-opera/inter/validatorpk"
	"github.com/Fantom-foundation/go-opera/opera/genesis/gpos"
	"github.com/Fantom-foundation/lachesis-base/inter/idx"
	"github.com/ethereum/go-ethereum/crypto"
)

func GetValidators(validatorPKs []string, creationTime inter.Timestamp) gpos.Validators {
	validators := make(gpos.Validators, 0, len(validatorPKs))

	for i := idx.ValidatorID(1); i <= idx.ValidatorID(len(validatorPKs)); i++ {
		//key := FakeKey(i)
		pubkeyHex, err := hex.DecodeString(validatorPKs[i-1])
		if err != nil {
			panic(err)
		}

		pubkey, err := crypto.UnmarshalPubkey(pubkeyHex)
		if err != nil {
			panic(err)
		}

		validators = append(validators, gpos.Validator{
			ID:      i,
			Address: crypto.PubkeyToAddress(*pubkey),
			PubKey: validatorpk.PubKey{
				Raw:  crypto.FromECDSAPub(pubkey),
				Type: validatorpk.Types.Secp256k1,
			},
			CreationTime:     creationTime,
			CreationEpoch:    0,
			DeactivatedTime:  0,
			DeactivatedEpoch: 0,
			Status:           0,
		})
	}

	return validators
}
