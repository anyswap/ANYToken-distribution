package distributer

import (
	"fmt"
	"io"
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

		if opt.SaveDB && hasSendedReward {
			mdist := &mongodb.MgoDistributeInfo{
				Exchange:     strings.ToLower(exchange),
				Pairs:        params.GetExchangePairs(exchange),
				ByWhat:       opt.byWhat,
				Start:        opt.StartHeight,
				End:          opt.EndHeight,
				RewardToken:  opt.RewardToken,
				Rewards:      rewardsSended.String(),
				SampleHeight: opt.SampleHeight,
				Timestamp:    uint64(time.Now().Unix()),
			}
			_ = mongodb.TryDoTimes("AddDistributeInfo "+mdist.Pairs, func() error {
				return mongodb.AddDistributeInfo(mdist)
			})
		}
	}
	return nil
}

func (opt *Option) writeSendRewardTitleLine(outputFile io.Writer, exchange string) (keyShare, keyNumber string, err error) {
	var extraInfo string
	switch opt.byWhat {
	case byLiquidMethodID:
		keyShare = byLiquidMethodID
		keyNumber = "height"
		extraInfo = fmt.Sprintf("sampleHeight=%v", opt.SampleHeight)
	case byVolumeMethodID:
		keyShare = byVolumeMethodID
		keyNumber = "txcount"
		extraInfo = fmt.Sprintf("novolumes=%d", opt.noVolumes)
	default:
		err = fmt.Errorf("unknown byWhat '%v'", opt.byWhat)
		return
	}
	// plus common extra info
	extraInfo += fmt.Sprintf(
		"&&start=%v&&end=%v&&totalReward=%v&&exchange=%v&&rewardToken=%v",
		opt.StartHeight, opt.EndHeight, opt.TotalValue,
		strings.ToLower(exchange), strings.ToLower(opt.RewardToken))
	// write title
	if opt.DryRun {
		err = WriteOutput(outputFile, "#account", "reward", keyShare, keyNumber, extraInfo)
	} else {
		err = WriteOutput(outputFile, "#account", "reward", keyShare, keyNumber, "txhash", extraInfo)
	}
	return
}

func (opt *Option) sendRewards(idx int, exchange string, accountStats mongodb.AccountStatSlice) (*big.Int, error) {
	outputFile, err := opt.getOutputFile(idx)
	if err != nil {
		return nil, err
	}

	keyShare, keyNumber, err := opt.writeSendRewardTitleLine(outputFile, exchange)
	if err != nil {
		return nil, err
	}

	rewardsSended := big.NewInt(0)
	totalDustReward := big.NewInt(0)
	totalDustRewardCount := 0
	i := uint64(0)
	for _, stat := range accountStats {
		if stat.Reward == nil || stat.Reward.Sign() <= 0 {
			log.Warn("empty reward stat exist", "stat", stat.String())
			continue
		}
		log.Info("sendRewards begin", "account", stat.Account.String(), "reward", stat.Reward, keyShare, stat.Share, keyNumber, stat.Number, "dryrun", opt.DryRun)
		txHash, err := opt.SendRewardsTransaction(stat.Account, stat.Reward)
		switch err {
		case nil:
		case errDustReward:
			totalDustReward.Add(totalDustReward, stat.Reward)
			totalDustRewardCount++
		default:
			log.Error("[sendRewards] send tx failed", "account", stat.Account.String(), "reward", stat.Reward, "dryrun", opt.DryRun, "err", err)
			return rewardsSended, errSendTransactionFailed
		}
		rewardsSended.Add(rewardsSended, stat.Reward)
		if opt.DryRun || txHash != nil {
			// write body
			_ = opt.WriteSendRewardResult(outputFile, exchange, stat, txHash)
			i++
		}
		if !opt.DryRun && opt.BatchCount > 0 && i%opt.BatchCount == 0 {
			time.Sleep(time.Duration(opt.BatchInterval) * time.Millisecond)
		}
	}

	log.Info("[sendRewards] rewards sended",
		"exchange", exchange,
		"totalRewards", opt.TotalValue,
		"rewardsSended", rewardsSended,
		"allRewardsSended", opt.TotalValue == nil || rewardsSended.Cmp(opt.TotalValue) == 0,
		"totalDustReward", totalDustReward,
		"totalDustRewardCount", totalDustRewardCount,
	)
	return rewardsSended, nil
}
