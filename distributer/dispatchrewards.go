package distributer

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
)

func dispatchRewards(opt *Option, accounts []common.Address, shares []*big.Int) {
	if len(accounts) != len(shares) {
		log.Error("number of accounts %v is not equal to shares %v", len(accounts), len(shares))
		return
	}
	totalShare := calcTotalShare(shares)
	if totalShare.Sign() <= 0 {
		log.Error("sum shares is zero")
		return
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

	sendRewards(accounts, rewards, shares, common.HexToAddress(opt.RewardToken), opt.DryRun)
}

func sendRewards(accounts []common.Address, rewards, shares []*big.Int, rewardToken common.Address, dryRun bool) {
	if len(accounts) != len(rewards) || len(accounts) != len(shares) {
		log.Error("number of accounts %v, rewards %v, and shares %v are not equal", len(accounts), len(rewards), len(shares))
		return
	}
	var reward, share *big.Int
	for i, account := range accounts {
		reward = rewards[i]
		share = shares[i]
		if reward == nil || reward.Sign() <= 0 {
			continue
		}
		log.Info("sendRewards begin", "account", account.String(), "reward", reward, "share", share, "dryrun", dryRun)
		txHash, err := commonTxArgs.sendRewardsTransaction(account, reward, rewardToken, dryRun)
		if err != nil {
			log.Info("sendRewards failed", "account", account.String(), "reward", reward, "share", share, "dryrun", dryRun, "err", err)
		}
		err = writeOutput(strings.ToLower(account.String()), reward.String(), txHash.String())
		if err != nil {
			log.Info("sendRewards write output error", "err", err)
		}
	}
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

func writeOutput(account, reward, txHash string) error {
	if outputFile == nil {
		return nil
	}
	msg := fmt.Sprintf("%s %s %s\n", account, reward, txHash)
	_, err := outputFile.Write([]byte(msg))
	return err
}
