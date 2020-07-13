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
	if !config.Distribute.Enable {
		log.Info("[distribute] function is not enabled")
		return
	}
	log.Info("[distribute] start job")
}
