package distributer

import (
	"math/big"

	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
)

// CalcTotalValue calc the summary
func CalcTotalValue(shares []*big.Int) *big.Int {
	sum := big.NewInt(0)
	for _, share := range shares {
		if share == nil || share.Sign() <= 0 {
			continue
		}
		sum.Add(sum, share)
	}
	return sum
}

// CalcRewardsByShares calc rewards by shares
func CalcRewardsByShares(totalReward *big.Int, accounts []common.Address, shares []*big.Int) []*big.Int {
	if len(accounts) != len(shares) {
		log.Error("number of accounts %v and shares %v are not equal", len(accounts), len(shares))
		panic("number of accounts and shares are not equal")
	}
	totalShare := CalcTotalValue(shares)
	if totalShare.Sign() <= 0 {
		return nil
	}

	rewards := make([]*big.Int, len(accounts))

	var reward *big.Int
	sum := big.NewInt(0)
	for i, share := range shares {
		if share == nil || share.Sign() <= 0 {
			continue
		}
		reward = new(big.Int)
		reward.Mul(totalReward, share)
		reward.Div(reward, totalShare)
		rewards[i] = reward
		sum.Add(sum, reward)
	}
	if sum.Cmp(totalReward) < 0 { // ensure zero rewards left
		left := new(big.Int).Sub(totalReward, sum)
		count := int64(len(shares))
		avg := new(big.Int).Div(left, big.NewInt(count))
		mod := new(big.Int).Mod(left, big.NewInt(count)).Int64()
		for i := int64(0); i < count; i++ {
			rewards[i].Add(rewards[i], avg)
			if i < mod {
				rewards[i].Add(rewards[i], big.NewInt(1))
			}
		}
	}
	return rewards
}
