package distributer

import (
	"math/big"
	"strings"

	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/mongodb"
	"github.com/anyswap/ANYToken-distribution/params"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
)

func dispatchLiquidityRewards(opt *Option, accounts []common.Address, rewards, liquids []*big.Int) error {
	return dispatchRewards(opt, accounts, rewards, liquids, nil)
}

func dispatchVolumeRewards(opt *Option, accounts []common.Address, rewards, volumes []*big.Int, txcounts []int) error {
	return dispatchRewards(opt, accounts, rewards, volumes, txcounts)
}

func dispatchRewards(opt *Option, accounts []common.Address, rewards, volumes []*big.Int, txcounts []int) error {
	rewardsSended, err := sendRewards(accounts, rewards, volumes, txcounts, opt)

	hasSendedReward := rewardsSended.Sign() > 0

	if !opt.DryRun && hasSendedReward {
		mdist := &mongodb.MgoDistributeInfo{
			Exchange:    strings.ToLower(opt.Exchange),
			Pairs:       params.GetExchangePairs(opt.Exchange),
			ByWhat:      opt.byWhat,
			Start:       opt.StartHeight,
			End:         opt.EndHeight,
			RewardToken: opt.RewardToken,
			Rewards:     rewardsSended.String(),
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

func sendRewards(accounts []common.Address, rewards, volumes []*big.Int, txcounts []int, opt *Option) (*big.Int, error) {
	if len(accounts) != len(rewards) {
		log.Fatalf("number of accounts %v and rewards %v are not equal", len(accounts), len(rewards))
	}
	if len(accounts) != len(volumes) {
		log.Fatalf("number of accounts %v and volumes %v are not equal", len(accounts), len(volumes))
	}
	hasTxCounts := len(txcounts) != 0
	if hasTxCounts && len(accounts) != len(txcounts) {
		log.Fatalf("number of accounts %v and txcounts %v are not equal", len(accounts), len(txcounts))
	}
	dryRun := opt.DryRun
	rewardsSended := big.NewInt(0)
	var reward *big.Int
	for i, account := range accounts {
		reward = rewards[i]
		if reward == nil || reward.Sign() <= 0 {
			continue
		}
		if hasTxCounts {
			log.Info("sendRewards begin", "account", account.String(), "reward", reward, "volume", volumes[i], "txcounts", txcounts[i], "dryrun", dryRun)
		} else {
			log.Info("sendRewards begin", "account", account.String(), "reward", reward, "liquid", volumes[i], "dryrun", dryRun)
		}
		txHash, err := opt.SendRewardsTransaction(account, reward)
		if err != nil {
			log.Info("sendRewards failed", "account", account.String(), "reward", reward, "dryrun", dryRun, "err", err)
			return rewardsSended, errSendTransactionFailed
		}
		rewardsSended.Add(rewardsSended, reward)
		if hasTxCounts {
			_ = opt.WriteSendRewardWithVolumeResult(account, reward, volumes[i], txcounts[i], txHash)
		} else {
			_ = opt.WriteSendRewardResult(account, reward, volumes[i], txHash)
		}
	}
	return rewardsSended, nil
}
