package distributer

import (
	"github.com/anyswap/ANYToken-distribution/callapi"
	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/params"
)

var capi *callapi.APICaller

// SetAPICaller set API caller
func SetAPICaller(apiCaller *callapi.APICaller) {
	capi = apiCaller
}

// Start start distribute
func Start(apiCaller *callapi.APICaller) {
	SetAPICaller(apiCaller)
	config := params.GetConfig()
	for _, distCfg := range config.Distribute {
		if !distCfg.Enable {
			log.Warn("[distribute] ignore disabled config", "config", distCfg)
			continue
		}
		go startDistributeJob(distCfg)
	}
}

func startDistributeJob(distCfg *params.DistributeConfig) {
	log.Info("[distribute] start job", "config", distCfg)
}
