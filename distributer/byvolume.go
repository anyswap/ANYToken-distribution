package distributer

import (
	"github.com/anyswap/ANYToken-distribution/log"
)

// ByVolume distribute rewards by vloume
func ByVolume(opt *Option) error {
	opt.byWhat = byVolumeMethod
	if opt.TotalValue == nil || opt.TotalValue.Sign() <= 0 {
		log.Warn("no volume rewards", "option", opt.String())
		return errTotalRewardsIsZero
	}
	err := opt.checkAndInit()
	defer opt.deinit()
	if err != nil {
		log.Error("[byvolume] check option error", "option", opt.String(), "err", err)
		return errCheckOptionFailed
	}
	accounts, volumes, err := opt.GetAccountsAndVolumes()
	if err != nil {
		log.Error("[byvolume] GetAccountsAndVolumes error", "err", err)
		return errGetAccountsVolumeFailed
	}
	if len(accounts) == 0 {
		log.Warn("[byvolume] no accounts. " + opt.String())
		return errNoAccountSatisfied
	}
	return dispatchRewards(opt, accounts, volumes)
}
