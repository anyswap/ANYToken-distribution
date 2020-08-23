package distributer

import (
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/mongodb"
	"github.com/anyswap/ANYToken-distribution/params"
)

func (opt *Option) dispatchRewards(accountStats []mongodb.AccountStatSlice) error {
	for i, exchange := range opt.Exchanges {
		rewardsSended, err := opt.sendRewards(i, exchange, accountStats[i])
		if err != nil {
			return err
		}

		hasSendedReward := rewardsSended.Sign() > 0

		if !opt.DryRun && hasSendedReward {
			mdist := &mongodb.MgoDistributeInfo{
				Exchange:     strings.ToLower(exchange),
				Pairs:        params.GetExchangePairs(exchange),
				ByWhat:       opt.byWhat,
				Start:        opt.StartHeight,
				End:          opt.EndHeight,
				RewardToken:  opt.RewardToken,
				Rewards:      rewardsSended.String(),
				SampleHeigts: opt.Heights,
				Timestamp:    uint64(time.Now().Unix()),
			}
			_ = mongodb.TryDoTimes("AddDistributeInfo "+mdist.Pairs, func() error {
				return mongodb.AddDistributeInfo(mdist)
			})
		}
	}
	return nil
}

func (opt *Option) getSampleHeightsInfo() string {
	if len(opt.Heights) == 0 {
		return ""
	}
	info := "sampleHeights="
	for i, height := range opt.Heights {
		info += fmt.Sprintf("%d", height)
		if i < len(opt.Heights)-1 {
			info += "-"
		}
	}
	return info
}

func (opt *Option) sendRewards(idx int, exchange string, accountStats mongodb.AccountStatSlice) (*big.Int, error) {
	outputFile, err := opt.getOutputFile(idx)
	if err != nil {
		return nil, err
	}

	var keyShare, keyNumber, extraInfo string
	switch opt.byWhat {
	case byLiquidMethodID:
		keyShare = byLiquidMethodID
		keyNumber = "height"
		extraInfo = opt.getSampleHeightsInfo()
	case byVolumeMethodID:
		keyShare = byVolumeMethodID
		keyNumber = "txcount"
		extraInfo = fmt.Sprintf("novolumes=%d", opt.noVolumes)
	default:
		return nil, fmt.Errorf("unknown byWhat '%v'", opt.byWhat)
	}
	// plus common extra info
	extraInfo += fmt.Sprintf(
		"&&start=%v&&end=%v&&totalReward=%v&&exchange=%v&&rewardToken=%v",
		opt.StartHeight, opt.EndHeight, opt.TotalValue,
		strings.ToLower(exchange), strings.ToLower(opt.RewardToken))
	// write title
	if opt.DryRun {
		_ = WriteOutput(outputFile, "#account", "reward", keyShare, keyNumber, extraInfo)
	} else {
		_ = WriteOutput(outputFile, "#account", "reward", keyShare, keyNumber, "txhash", extraInfo)
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
			log.Warn("sendRewards failed", "account", stat.Account.String(), "reward", stat.Reward, "dryrun", opt.DryRun, "err", err)
			return rewardsSended, errSendTransactionFailed
		}
		rewardsSended.Add(rewardsSended, stat.Reward)
		// write body
		_ = opt.WriteSendRewardResult(outputFile, exchange, stat, txHash)
	}
	return rewardsSended, nil
}
