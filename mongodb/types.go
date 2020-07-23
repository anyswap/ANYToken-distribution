package mongodb

import (
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
	if s[i].Reward != nil && s[j].Reward != nil {
		return s[i].Reward.Cmp(s[j].Reward) > 0
	}
	return s[i].Share.Cmp(s[j].Share) > 0
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
	totalShare := s.CalcTotalShare()
	if totalShare.Sign() <= 0 {
		return
	}

	sum := big.NewInt(0)
	for _, stat := range s {
		reward := new(big.Int).Mul(totalReward, stat.Share)
		reward.Div(reward, totalShare)
		stat.Reward = reward
		sum.Add(sum, reward)
	}
	if sum.Cmp(totalReward) < 0 { // ensure zero rewards left
		left := new(big.Int).Sub(totalReward, sum)
		if left.Int64() >= int64(len(s)) {
			log.Errorf("please check CalcRewards, left rewards %v is not lower than count of acounts %v", left, len(s))
		}
		for i := int64(0); i < left.Int64(); i++ {
			s[i].Reward.Add(s[i].Reward, big.NewInt(1))
		}
	}
}
