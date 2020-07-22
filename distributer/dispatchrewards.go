package distributer

import (
	"math/big"
	"strings"

	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/mongodb"
	"github.com/anyswap/ANYToken-distribution/params"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
)

func dispatchLiquidityRewards(opt *Option, accounts []common.Address, rewards, liquids []*big.Int, minHeights, sampleHeights []uint64) error {
	return dispatchRewards(opt, accounts, rewards, liquids, minHeights, sampleHeights)
}

func dispatchVolumeRewards(opt *Option, accounts []common.Address, rewards, shares []*big.Int, numbers []uint64) error {
	return dispatchRewards(opt, accounts, rewards, shares, numbers, nil)
}

func dispatchRewards(opt *Option, accounts []common.Address, rewards, shares []*big.Int, numbers, sampleHeights []uint64) error {
	rewardsSended, err := sendRewards(accounts, rewards, shares, numbers, opt)

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
			SampleHeigts: sampleHeights,
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

func sendRewards(accounts []common.Address, rewards, shares []*big.Int, numbers []uint64, opt *Option) (*big.Int, error) {
	if len(accounts) != len(rewards) {
		log.Fatalf("number of accounts %v and rewards %v are not equal", len(accounts), len(rewards))
	}
	if len(accounts) != len(shares) {
		log.Fatalf("number of accounts %v and shares %v are not equal", len(accounts), len(shares))
	}
	if len(accounts) != len(numbers) {
		log.Fatalf("number of accounts %v and numbers %v are not equal", len(accounts), len(numbers))
	}
	var keyShare, keyNumber string
	switch opt.ByWhat() {
	case byLiquidMethod:
		keyShare = "liquid"
		keyNumber = "height"
	case byVolumeMethod:
		keyShare = "volume"
		keyNumber = "txcount"
	}
	dryRun := opt.DryRun
	rewardsSended := big.NewInt(0)
	var reward *big.Int
	for i, account := range accounts {
		reward = rewards[i]
		if reward == nil || reward.Sign() <= 0 {
			continue
		}
		log.Info("sendRewards begin", "account", account.String(), "reward", reward, keyShare, shares[i], keyNumber, numbers[i], "dryrun", dryRun)
		txHash, err := opt.SendRewardsTransaction(account, reward)
		if err != nil {
			log.Info("sendRewards failed", "account", account.String(), "reward", reward, "dryrun", dryRun, "err", err)
			return rewardsSended, errSendTransactionFailed
		}
		rewardsSended.Add(rewardsSended, reward)
		_ = opt.WriteSendRewardResult(account, reward, shares[i], numbers[i], txHash)
	}
	return rewardsSended, nil
}
