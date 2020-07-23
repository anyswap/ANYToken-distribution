package distributer

import (
	"crypto/rand"
	"math/big"
	"strings"
	"time"

	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/mongodb"
	"github.com/anyswap/ANYToken-distribution/params"
	"github.com/anyswap/ANYToken-distribution/tools"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
)

func getRandNumber(max uint64) uint64 {
	for i := 0; i < 3; i++ {
		randInt, err := rand.Int(rand.Reader, new(big.Int).SetUint64(max))
		if err == nil {
			return randInt.Uint64()
		}
	}
	return uint64(time.Now().Unix()) % max
}

// ByLiquidity distribute by liquidity
func ByLiquidity(opt *Option) error {
	log.Info("[byliquid] start", "option", opt.String())
	opt.byWhat = byLiquidMethod
	if opt.TotalValue == nil || opt.TotalValue.Sign() <= 0 {
		log.Warn("no liquidity rewards", "option", opt.String())
		return errTotalRewardsIsZero
	}
	err := opt.checkAndInit()
	defer opt.deinit()
	if err != nil {
		log.Error("[byliquid] check option error", "option", opt.String(), "err", err)
		return errCheckOptionFailed
	}
	accounts, err := opt.getAccounts()
	if err != nil {
		log.Error("[byliquid] get accounts error", "err", err)
		return errGetAccountListFailed
	}
	if len(accounts) == 0 {
		log.Warn("[byliquid] no accounts. " + opt.String())
		return errNoAccountSatisfied
	}
	accountStats := opt.getLiquidityBalances(accounts)
	if len(accountStats) == 0 {
		log.Warn("[byliquid] no account satisfied. " + opt.String())
		return errNoAccountSatisfied
	}
	accountStats.CalcRewards(opt.TotalValue)
	return opt.dispatchRewards(accountStats)
}

func (opt *Option) getLiquidityBalances(accounts []common.Address) (accountStats mongodb.AccountStatSlice) {
	_ = opt.WriteLiquiditySubject(opt.Exchange, opt.StartHeight, opt.EndHeight, len(accounts))

	if len(opt.Heights) == 0 {
		countOfBlocks := opt.EndHeight - opt.StartHeight
		// randomly pick smpale blocks to query liquidity balance, and keep the minimumn
		quarterCount := countOfBlocks/sampleCount + 1
		for i := uint64(0); i < sampleCount; i++ {
			height := opt.StartHeight + i*quarterCount + getRandNumber(quarterCount)
			if height >= opt.EndHeight {
				break
			}
			opt.Heights = append(opt.Heights, height)
		}
	}

	accountStats = opt.updateLiquidityBalance(accounts)

	totalLiquids := accountStats.CalcTotalShare()
	_ = opt.WriteLiquiditySummary(opt.Exchange, opt.StartHeight, opt.EndHeight, len(accountStats), totalLiquids, opt.TotalValue)
	for _, stat := range accountStats {
		_ = opt.WriteLiquidityBalance(stat.Account, stat.Share, stat.Number)
	}
	return accountStats
}

func (opt *Option) updateLiquidityBalance(accounts []common.Address) (accountStats mongodb.AccountStatSlice) {
	exchange := opt.Exchange
	exchangeAddr := common.HexToAddress(exchange)

	finStatMap := make(map[common.Address]*mongodb.AccountStat)

	for _, height := range opt.Heights {
		blockNumber := new(big.Int).SetUint64(height)
		totalLiquid := big.NewInt(0)
		for _, account := range accounts {
			finStat, exist := finStatMap[account]
			// if jump here, verifyTotalLiquidity will warn total value not equal
			// still jump here for saving performance
			if exist && finStat.Share.Sign() == 0 {
				continue // find zero liquidity then no need to query anymore
			}
			var value *big.Int
			accoutStr := strings.ToLower(account.String())
			liquid, err := mongodb.FindLiquidityBalance(exchange, accoutStr, height)
			if err == nil {
				value, _ = tools.GetBigIntFromString(liquid)
			}
			for value == nil {
				value, err = capi.GetLiquidityBalance(exchangeAddr, account, blockNumber)
				if err != nil {
					log.Warn("[byliquid] GetLiquidityBalance error", "err", err)
					time.Sleep(time.Second)
					continue
				}
			}
			_ = opt.WriteLiquidityBalance(account, value, height)
			if !opt.DryRun || opt.SaveDB {
				mliq := &mongodb.MgoLiquidityBalance{
					Key:         mongodb.GetKeyOfLiquidityBalance(exchange, accoutStr, height),
					Exchange:    strings.ToLower(exchange),
					Pairs:       params.GetExchangePairs(exchange),
					Account:     accoutStr,
					BlockNumber: height,
					Liquidity:   value.String(),
				}
				_ = mongodb.TryDoTimes("AddLiquidityBalance "+mliq.Key, func() error {
					return mongodb.AddLiquidityBalance(mliq)
				})
			}
			totalLiquid.Add(totalLiquid, value)
			if exist {
				if finStat.Share.Cmp(value) > 0 { // get minimumn liquidity balance
					finStat.Share = value
					finStat.Number = height
				}
			} else {
				finStatMap[account] = &mongodb.AccountStat{
					Account: account,
					Share:   value,
					Number:  height,
				}
			}
		}
		err := verifyTotalLiquidity(exchangeAddr, blockNumber, totalLiquid)
		if err != nil {
			log.Warn("[byliquid] " + err.Error())
		}
	}

	return mongodb.ConvertToSortedSlice(finStatMap)
}

func verifyTotalLiquidity(exchangeAddr common.Address, blockNumber, totalLiquid *big.Int) error {
	for {
		totalSupply, err := capi.GetExchangeLiquidity(exchangeAddr, blockNumber)
		if err == nil {
			if totalLiquid.Cmp(totalSupply) != 0 {
				//return fmt.Errorf("account list is not complete at height %v. total liqudity %v is not equal to total supply %v", blockNumber, totalLiquid, totalSupply)
			} else {
				log.Info("[byliquid] account list is complete", "height", blockNumber, "totalsupply", totalSupply)
			}
			return nil
		}
		log.Warn("[byliquid] GetExchangeLiquidity error", "exchange", exchangeAddr.String(), "blockNumber", blockNumber, "err", err)
		time.Sleep(time.Second)
	}
}
