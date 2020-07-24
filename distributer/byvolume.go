package distributer

import (
	"github.com/anyswap/ANYToken-distribution/log"
)

// ByVolume distribute rewards by vloume
func ByVolume(opt *Option) error {
	opt.byWhat = byVolumeMethod
	log.Info("[byvolume] start", "option", opt.String())
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
	accountStats, err := opt.GetAccountsAndRewards()
	if err != nil {
		log.Error("[byvolume] GetAccountsAndRewards error", "err", err)
		return errGetAccountsRewardsFailed
	}
	if len(accountStats) == 0 {
		log.Warn("[byvolume] no accounts. " + opt.String())
		return errNoAccountSatisfied
	}
	return opt.dispatchRewards(accountStats)
}
