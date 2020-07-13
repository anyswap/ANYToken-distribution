package distributer

import (
	"github.com/anyswap/ANYToken-distribution/log"
)

// ByVolume ditribute rewards by vloume
func ByVolume(opt *Option) {
	var err error
	err = commonTxArgs.Check() // check args before opts
	if err != nil {
		log.Error("[ditribute] Check commonTxArgs error", "args", commonTxArgs, "err", err)
		return
	}
	err = opt.checkAndInit()
	defer opt.deinit()
	if err != nil {
		log.Error("[ditribute] check option error", "option", opt, "err", err)
		return
	}
	accounts, volumes, err := opt.getAccountsAndVolumes()
	if err != nil {
		log.Error("[distribute] getAccountsAndVolumes error", "err", err)
		return
	}
	if len(accounts) == 0 {
		log.Warn("[ditribute] no accounts")
		return
	}
	dispatchRewards(opt, accounts, volumes)
}
