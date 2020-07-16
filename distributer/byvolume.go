package distributer

import (
	"github.com/anyswap/ANYToken-distribution/log"
)

// ByVolume distribute rewards by vloume
func ByVolume(opt *Option, args *BuildTxArgs) error {
	if opt.TotalValue == nil || opt.TotalValue.Sign() <= 0 {
		log.Warn("no volume rewards", "option", opt.String())
		return errTotalRewardsIsZero
	}
	opt.byWhat = byVolumeMethod
	opt.buildTxArgs = args
	err := opt.checkAndInit()
	defer opt.deinit()
	if err != nil {
		log.Error("[byvolume] check option error", "option", opt.String(), "err", err)
		return errCheckOptionFailed
	}
	accounts, volumes, err := opt.getAccountsAndVolumes()
	if err != nil {
		log.Error("[byvolume] getAccountsAndVolumes error", "err", err)
		return errGetAccountsVolumeFailed
	}
	if len(accounts) == 0 {
		log.Warn("[byvolume] no accounts. " + opt.String())
		return errNoAccountSatisfied
	}
	return dispatchRewards(opt, accounts, volumes)
}
