package distribute

import (
	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/params"
)

// Start start distribute
func Start() {
	config := params.GetConfig()
	if !config.Distribute.Enable {
		log.Info("[distribute] function is not enabled")
		return
	}
	log.Info("[distribute] start job")
}
