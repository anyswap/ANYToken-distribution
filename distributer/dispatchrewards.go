package distributer

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/mongodb"
	"github.com/anyswap/ANYToken-distribution/params"
)

func (opt *Option) dispatchRewards(accountStats mongodb.AccountStatSlice) error {
	rewardsSended, err := opt.sendRewards(accountStats)

	hasSendedReward := rewardsSended.Sign() > 0

	if !opt.DryRun && hasSendedReward {
		mdist := &mongodb.MgoDistributeInfo{
			Exchange:     strings.ToLower(opt.Exchange),
			Pairs:        params.GetExchangePairs(opt.Exchange),
			ByWhat:       opt.byWhat,
			Start:        opt.StartHeight,
			End:          opt.EndHeight,
			RewardToken:  opt.RewardToken,
			Rewards:      rewardsSended.String(),
			SampleHeigts: opt.Heights,
		}
		_ = mongodb.TryDoTimes("AddDistributeInfo "+mdist.Pairs, func() error {
			return mongodb.AddDistributeInfo(mdist)
		})
	}
	if hasSendedReward {
		// treat this situation as success
		// and resolve partly failed manually if have
		// don't retry send rewards with return nil here
		return nil
	}
	return err
}

func (opt *Option) sendRewards(accountStats mongodb.AccountStatSlice) (*big.Int, error) {
	var keyShare, keyNumber, noVolumeInfo string
	switch opt.byWhat {
	case byLiquidMethod:
		keyShare = "liquidity"
		keyNumber = "height"
	case byVolumeMethod:
		keyShare = "volume"
		keyNumber = "txcount"
		noVolumeInfo = fmt.Sprintf("novolumes=%d", opt.noVolumes)
	}
	// write title
	if opt.DryRun {
		if noVolumeInfo != "" {
			_ = opt.WriteOutput("#account", "reward", keyShare, keyNumber, noVolumeInfo)
		} else {
			_ = opt.WriteOutput("#account", "reward", keyShare, keyNumber)
		}
	} else {
		_ = opt.WriteOutput("#account", "reward", keyShare, keyNumber, "txhash")
	}

	rewardsSended := big.NewInt(0)
	for _, stat := range accountStats {
		if stat.Reward == nil || stat.Reward.Sign() <= 0 {
			log.Warn("empty reward stat exist", "stat", stat.String())
			continue
		}
		log.Info("sendRewards begin", "account", stat.Account.String(), "reward", stat.Reward, keyShare, stat.Share, keyNumber, stat.Number, "dryrun", opt.DryRun)
		txHash, err := opt.SendRewardsTransaction(stat.Account, stat.Reward)
		if err != nil {
			log.Info("sendRewards failed", "account", stat.Account.String(), "reward", stat.Reward, "dryrun", opt.DryRun, "err", err)
			return rewardsSended, errSendTransactionFailed
		}
		rewardsSended.Add(rewardsSended, stat.Reward)
		// write body
		_ = opt.WriteSendRewardResult(stat.Account, stat.Reward, stat.Share, stat.Number, txHash)
	}
	return rewardsSended, nil
}
