package distributer

import (
	"math/big"
	"strings"

	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/mongodb"
	"github.com/anyswap/ANYToken-distribution/params"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
)

func dispatchRewards(opt *Option, accounts []common.Address, rewards []*big.Int) error {
	rewardsSended, err := sendRewards(accounts, rewards, opt)

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

func sendRewards(accounts []common.Address, rewards []*big.Int, opt *Option) (*big.Int, error) {
	rewardsSended := big.NewInt(0)
	if len(accounts) != len(rewards) {
		log.Fatalf("number of accounts %v and rewards %v are not equal", len(accounts), len(rewards))
	}
	dryRun := opt.DryRun
	var reward *big.Int
	for i, account := range accounts {
		reward = rewards[i]
		if reward == nil || reward.Sign() <= 0 {
			continue
		}
		log.Info("sendRewards begin", "account", account.String(), "reward", reward, "dryrun", dryRun)
		txHash, err := opt.SendRewardsTransaction(account, reward)
		if err != nil {
			log.Info("sendRewards failed", "account", account.String(), "reward", reward, "dryrun", dryRun, "err", err)
			return rewardsSended, errSendTransactionFailed
		}
		rewardsSended.Add(rewardsSended, reward)
		_ = opt.WriteSendRewardResult(account, reward, txHash)
	}
	return rewardsSended, nil
}
