package distributer

import (
	"math/big"
	"strings"

	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/mongodb"
	"github.com/anyswap/ANYToken-distribution/params"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
)

func dispatchRewards(opt *Option, accounts []common.Address, shares []*big.Int) error {
	if len(accounts) != len(shares) {
		log.Error("number of accounts %v is not equal to shares %v", len(accounts), len(shares))
		return errAccountsLengthMismatch
	}
	totalShare := calcTotalShare(shares)
	if totalShare.Sign() <= 0 {
		log.Error("sum shares is zero")
		return errNoAccountSatisfied
	}
	rewards := make([]*big.Int, len(accounts))
	totalReward := opt.TotalValue
	log.Info("dispatchRewards", "option", opt, "totalShare", totalShare)

	var reward *big.Int
	for i, share := range shares {
		if share == nil || share.Sign() <= 0 {
			continue
		}
		reward = new(big.Int)
		reward.Mul(totalReward, share)
		reward.Div(reward, totalShare)
		rewards[i] = reward
	}

	rewardsSended, err := sendRewards(accounts, rewards, shares, opt)

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

func sendRewards(accounts []common.Address, rewards, shares []*big.Int, opt *Option) (*big.Int, error) {
	rewardsSended := big.NewInt(0)
	if len(accounts) != len(rewards) || len(accounts) != len(shares) {
		log.Error("number of accounts %v, rewards %v, and shares %v are not equal", len(accounts), len(rewards), len(shares))
		return rewardsSended, errAccountsLengthMismatch
	}
	rewardToken := common.HexToAddress(opt.RewardToken)
	dryRun := opt.DryRun
	var reward, share *big.Int
	for i, account := range accounts {
		reward = rewards[i]
		share = shares[i]
		if reward == nil || reward.Sign() <= 0 {
			continue
		}
		log.Info("sendRewards begin", "account", account.String(), "reward", reward, "share", share, "dryrun", dryRun)
		txHash, err := opt.sendRewardsTransaction(account, reward, rewardToken, dryRun)
		if err != nil {
			log.Info("sendRewards failed", "account", account.String(), "reward", reward, "share", share, "dryrun", dryRun, "err", err)
			return rewardsSended, errSendTransactionFailed
		}
		rewardsSended.Add(rewardsSended, reward)
		_ = opt.writeOutput(strings.ToLower(account.String()), reward.String(), txHash.String())
	}
	return rewardsSended, nil
}

func calcTotalShare(shares []*big.Int) *big.Int {
	sum := big.NewInt(0)
	for _, share := range shares {
		if share == nil || share.Sign() <= 0 {
			continue
		}
		sum.Add(sum, share)
	}
	return sum
}
