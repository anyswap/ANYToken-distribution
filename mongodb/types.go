package mongodb

import (
	"bytes"
	"fmt"
	"math/big"
	"sort"

	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
)

// AccountStat account statistics
type AccountStat struct {
	Account common.Address
	Reward  *big.Int
	Share   *big.Int // volume or liquidity
	Number  uint64   // txcount or height
}

func (s *AccountStat) String() string {
	return fmt.Sprintf("Account %v Reward %v Share %v Number %v", s.Account.String(), s.Reward, s.Share, s.Number)
}

// ConvertToSortedSlice convert to sorted slice
func ConvertToSortedSlice(statMap map[common.Address]*AccountStat) AccountStatSlice {
	accountStats := make(AccountStatSlice, 0, len(statMap))
	for _, stat := range statMap {
		if stat.Share == nil || stat.Share.Sign() <= 0 {
			continue
		}
		accountStats = append(accountStats, stat)
	}
	sort.Sort(accountStats)
	return accountStats
}

// AccountStatSlice slice sort by reward in reverse order
type AccountStatSlice []*AccountStat

func (s AccountStatSlice) Len() int {
	return len(s)
}

func (s AccountStatSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s AccountStatSlice) Less(i, j int) bool {
	var cmp int
	if s[i].Reward != nil && s[j].Reward != nil {
		cmp = s[i].Reward.Cmp(s[j].Reward)
	} else {
		cmp = s[i].Share.Cmp(s[j].Share)
	}
	if cmp != 0 {
		return cmp > 0
	}
	return bytes.Compare(s[i].Account[:], s[j].Account[:]) < 0
}

// IsAccountExist is account exist in slice
func (s AccountStatSlice) IsAccountExist(account common.Address) bool {
	for _, stat := range s {
		if stat.Account == account {
			return true
		}
	}
	return false
}

// CalcTotalShare calc the summary
func (s AccountStatSlice) CalcTotalShare() *big.Int {
	sum := big.NewInt(0)
	for _, stat := range s {
		sum.Add(sum, stat.Share)
	}
	return sum
}

// CalcTotalReward calc the summary
func (s AccountStatSlice) CalcTotalReward() *big.Int {
	sum := big.NewInt(0)
	for _, stat := range s {
		sum.Add(sum, stat.Reward)
	}
	return sum
}

// CalcRewards calc rewards by shares
func (s AccountStatSlice) CalcRewards(totalReward *big.Int) {
	if len(s) == 0 {
		return
	}
	// if has already calced reward before, use these values to
	// redispatch rewards when total rewards is changed.
	hasRewardValue := s[0].Reward != nil
	totalShareSlice := make([]*big.Int, len(s))
	for i, stat := range s {
		if hasRewardValue {
			totalShareSlice[i] = stat.Reward
		} else {
			totalShareSlice[i] = stat.Share
		}
	}
	rewards := DivideRewards(totalReward, totalShareSlice)
	for i, stat := range s {
		stat.Reward = rewards[i]
	}
}

// SumWeightShares sum weight shares (do not change slice itself)
func (s AccountStatSlice) SumWeightShares(weight uint64) (totalWeightShare *big.Int) {
	totalWeightShare = big.NewInt(0)
	biWeight := new(big.Int).SetUint64(weight)
	for _, stat := range s {
		weightShare := new(big.Int).Mul(stat.Share, biWeight)
		totalWeightShare.Add(totalWeightShare, weightShare)
	}
	return totalWeightShare
}

// CalcWeightedRewards calc weighted rewards
func CalcWeightedRewards(stats []AccountStatSlice, totalReward *big.Int, weights []uint64) {
	if weights != nil && len(weights) != len(stats) {
		log.Error("calc weighted rewards with not equal number of stats and weights")
		return
	}
	if len(stats) <= 1 {
		if len(stats) == 1 {
			stats[0].CalcRewards(totalReward)
		}
		return
	}
	weight := uint64(1)
	totalShareSlice := make([]*big.Int, len(stats))
	for i, stat := range stats {
		if weights != nil {
			weight = weights[i]
		}
		totalShareSlice[i] = stat.SumWeightShares(weight)
	}
	rewards := DivideRewards(totalReward, totalShareSlice)
	if len(rewards) != len(stats) {
		return
	}
	for i, stat := range stats {
		stat.CalcRewards(rewards[i])
	}
}

// DivideRewards divide rewards
func DivideRewards(totalReward *big.Int, totalShareSlice []*big.Int) (rewards []*big.Int) {
	sumTotalShare := big.NewInt(0)
	for _, share := range totalShareSlice {
		sumTotalShare.Add(sumTotalShare, share)
	}
	if sumTotalShare.Sign() <= 0 {
		return nil
	}
	sumReward := big.NewInt(0)
	for _, share := range totalShareSlice {
		reward := new(big.Int).Mul(totalReward, share)
		reward.Div(reward, sumTotalShare)
		sumReward.Add(sumReward, reward)
		rewards = append(rewards, reward)
	}
	if sumReward.Cmp(totalReward) < 0 {
		left := new(big.Int).Sub(totalReward, sumReward)
		count := big.NewInt(int64(len(rewards)))
		avg := new(big.Int).Div(left, count)
		mod := new(big.Int).Mod(left, count).Uint64()
		for i, reward := range rewards {
			if avg.Sign() > 0 {
				reward.Add(reward, avg)
			}
			if uint64(i) < mod {
				reward.Add(reward, big.NewInt(1))
			}
		}
	}

	// verify sum
	sumReward = big.NewInt(0)
	for _, reward := range rewards {
		sumReward.Add(sumReward, reward)
	}
	if sumReward.Cmp(totalReward) != 0 {
		log.Error("call DivideRewards verify sum failed", "sumReward", sumReward, "totalReward", totalReward)
	}
	return rewards
}
