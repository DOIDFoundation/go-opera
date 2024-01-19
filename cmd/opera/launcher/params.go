package launcher

import (
	"github.com/ethereum/go-ethereum/params"
)

var (
	Bootnodes = map[string][]string{
		"doid": {
			"enode://5208615fbfc2b38a49fc6fcf773c500c212637bda220b885e28d1eadeab0c4ba3df6036268117bf32b84c6203aa976e349641b291c5d58295b2cf3721c5c0f53@13.234.216.14:5050",
			"enode://574617a92ab3341f8a09fa07e0ccdb0e1cf1af498b7ae9721a3e122ee5d4fa8c5b749972a0f15a10b165abc8c282c35e821f68eb9e6bb59abbba91604de7cb64@35.237.88.8:5051",
		},
		"test": {
			"enode://7f4c2fb6fad64fb96cae8632209a5753ea8e52908842acfaa389847f4dfa9037d68e42ce87b8ef4c550303afd37079fe4b10fc438e68fe18905c754a7244a95f@35.237.88.8:5050",
		},
	}

	AllowedOperaGenesis = []GenesisTemplate{}
)

func overrideParams() {
	params.MainnetBootnodes = []string{}
	params.RopstenBootnodes = []string{}
	params.RinkebyBootnodes = []string{}
	params.GoerliBootnodes = []string{}
}
