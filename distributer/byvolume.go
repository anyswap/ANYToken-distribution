package distributer

import (
	"math/big"

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
	accounts, volumes, missSteps, err := opt.GetAccountsAndVolumes()
	if err != nil {
		log.Error("[byvolume] GetAccountsAndVolumes error", "err", err)
		return errGetAccountsVolumeFailed
	}
	if len(accounts) == 0 {
		log.Warn("[byvolume] no accounts. " + opt.String())
		return errNoAccountSatisfied
	}
	if missSteps > 0 {
		steps := (opt.EndHeight - opt.StartHeight) / opt.StepCount
		missRewards := new(big.Int)
		missRewards.Mul(opt.TotalValue, new(big.Int).SetUint64(missSteps))
		missRewards.Div(missRewards, new(big.Int).SetUint64(steps))
		opt.TotalValue.Sub(opt.TotalValue, missRewards)
		log.Info("[byvolume] find miss volume", "missSteps", missSteps, "missRewards", missRewards)
		if opt.TotalValue.Sign() <= 0 {
			return nil
		}
	}
	return dispatchRewards(opt, accounts, volumes)
}
