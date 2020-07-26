package distributer

import (
	"math/big"

	"github.com/anyswap/ANYToken-distribution/log"
)

// ByVolume distribute rewards by vloume
func ByVolume(opt *Option) error {
	opt.byWhat = byVolumeMethodID
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
	if opt.noVolumes > 0 && opt.StepReward.Sign() > 0 {
		subReward := new(big.Int).Mul(opt.StepReward, new(big.Int).SetUint64(opt.noVolumes))
		opt.TotalValue.Sub(opt.TotalValue, subReward)
	}
	return opt.dispatchRewards(accountStats)
}
