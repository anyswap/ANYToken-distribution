package distributer

import (
	"math/big"
	"math/rand"
	"strings"
	"time"

	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/mongodb"
	"github.com/anyswap/ANYToken-distribution/params"
	"github.com/anyswap/ANYToken-distribution/tools"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
)

// ByLiquidity distribute by liquidity
func ByLiquidity(opt *Option) error {
	opt.byWhat = byLiquidMethodID
	log.Info("[byliquid] start", "option", opt.String())
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
	accountStats := opt.getLiquidityBalances(accounts)
	if len(accountStats) != len(opt.Exchanges) {
		log.Warn("[byliquid] account list is not complete. " + opt.String())
		return errAccountsNotComplete
	}
	mongodb.CalcWeightedRewards(accountStats, opt.TotalValue, opt.LiquidWeights)
	return opt.dispatchRewards(accountStats)
}

func (opt *Option) getLiquidityBalances(accounts [][]common.Address) (accountStats []mongodb.AccountStatSlice) {
	if len(opt.Heights) == 0 {
		opt.calcSampleHeights()
	}
	accountStats = opt.updateLiquidityBalances(accounts)
	return accountStats
}

func (opt *Option) updateLiquidityBalances(accountsSlice [][]common.Address) (accountStats []mongodb.AccountStatSlice) {
	accountStats = make([]mongodb.AccountStatSlice, len(opt.Exchanges))
	for i, exchange := range opt.Exchanges {
		accounts := accountsSlice[i]
		WriteLiquiditySubject(exchange, opt.StartHeight, opt.EndHeight, len(accounts))
		stats, complete := opt.updateLiquidityBalance(exchange, accounts)
		if !complete {
			log.Error("[byliquid] account list is not complete", "exchange", exchange)
			break
		}
		totalLiquids := stats.CalcTotalShare()
		WriteLiquiditySummary(exchange, opt.StartHeight, opt.EndHeight, len(stats), totalLiquids, opt.TotalValue)
		for _, stat := range stats {
			WriteLiquidityBalance(stat.Account, stat.Share, stat.Number)
		}
		accountStats[i] = stats
	}
	return accountStats
}

func (opt *Option) updateLiquidityBalance(exchange string, accounts []common.Address) (accountStats mongodb.AccountStatSlice, complete bool) {
	exchangeAddr := common.HexToAddress(exchange)

	finStatMap := make(map[common.Address]*mongodb.AccountStat)

	// pick smpale blocks to query liquidity balance, and keep the minimumn
	for _, height := range opt.Heights {
		blockNumber := new(big.Int).SetUint64(height)
		totalSupply := capi.LoopGetExchangeLiquidity(exchangeAddr, blockNumber)
		exCoinBalance := capi.LoopGetCoinBalance(exchangeAddr, blockNumber)
		log.Info("get exchange liquidity and coin balance", "totalSupply", totalSupply, "exCoinBalance", exCoinBalance, "blockNumber", blockNumber)
		totalLiquid := big.NewInt(0)
		totalCoinBalance := big.NewInt(0)
		for _, account := range accounts {
			finStat, exist := finStatMap[account]
			// if jump here, verifyTotalLiquidity will warn total value not equal
			// still jump here for saving performance
			if exist && finStat.Share.Sign() == 0 {
				continue // find zero liquidity then no need to query anymore
			}
			var value *big.Int
			accoutStr := strings.ToLower(account.String())
			liquidStr, err := mongodb.FindLiquidityBalance(exchange, accoutStr, height)
			if err == nil {
				value, _ = tools.GetBigIntFromString(liquidStr)
			}
			for value == nil {
				value = capi.LoopGetLiquidityBalance(exchangeAddr, account, blockNumber)
			}
			totalLiquid.Add(totalLiquid, value)
			// convert liquid balance to coin balance
			coinBalance := new(big.Int).Mul(value, exCoinBalance)
			coinBalance.Div(coinBalance, totalSupply)
			totalCoinBalance.Add(totalCoinBalance, coinBalance)
			WriteLiquidityBalance(account, value, height)
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
			if exist {
				if finStat.Share.Cmp(value) > 0 { // get minimumn liquidity balance
					finStat.Share = coinBalance
					finStat.Number = height
				}
			} else {
				finStatMap[account] = &mongodb.AccountStat{
					Account: account,
					Share:   coinBalance,
					Number:  height,
				}
			}
		}
		if totalLiquid.Cmp(totalSupply) == 0 {
			log.Info("[byliquid] account list is complete", "exchange", exchange, "height", blockNumber, "totalsupply", totalSupply)
			complete = true
		} else if !complete {
			log.Warn("[byliquid] account list is not complete", "exchange", exchange, "height", blockNumber, "totalsupply", totalSupply, "totalLiquid", totalLiquid)
			time.Sleep(time.Second)
		}
		leftValue := new(big.Int).Sub(exCoinBalance, totalCoinBalance)
		distLeftValue(finStatMap, leftValue)
	}

	return mongodb.ConvertToSortedSlice(finStatMap), complete
}

func distLeftValue(finStatMap map[common.Address]*mongodb.AccountStat, leftValue *big.Int) {
	if leftValue.Sign() <= 0 {
		return
	}
	stats := mongodb.ConvertToSortedSlice(finStatMap)
	numAccounts := big.NewInt(int64(len(stats)))
	avg := new(big.Int).Div(leftValue, numAccounts)
	mod := new(big.Int).Mod(leftValue, numAccounts).Uint64()
	for i, stat := range stats {
		share := stat.Share
		if avg.Sign() > 0 {
			share.Add(share, avg)
		}
		if uint64(i) < mod {
			share.Add(share, big.NewInt(1))
		}
	}
}

// nolint:gosec // use of weak random number generator math/rand intentionally
func getRandNumbers(seedBlock, max, count uint64) (numbers []uint64) {
	log.Info("start get random numbers", "seedBlock", seedBlock, "max", max, "count", count)
	header := capi.LoopGetBlockHeader(new(big.Int).SetUint64(seedBlock))
	log.Info("get seed block hash success", "hash", header.Hash().String())
	dhash := common.Keccak256Hash(header.Hash().Bytes(), header.Number.Bytes())
	for i := uint64(1); i <= count; i++ {
		rehash := common.Keccak256Hash(dhash.Bytes(), new(big.Int).SetUint64(i).Bytes())
		rand.Seed(new(big.Int).SetBytes(rehash.Bytes()).Int64())
		number := uint64(rand.Intn(int(max)))
		numbers = append(numbers, number)
	}
	log.Info("get random numbers success", "seedBlock", seedBlock, "max", max, "numbers", numbers)
	return numbers
}

func (opt *Option) calcSampleHeights() {
	start := opt.StartHeight
	end := opt.EndHeight
	countOfBlocks := end - start
	step := (countOfBlocks + sampleCount - 1) / sampleCount
	randNums := getRandNumbers(end, step, sampleCount)
	for i := uint64(0); i < sampleCount; i++ {
		startFrom := start + i*step
		height := startFrom + randNums[i]
		if height >= end {
			break
		}
		opt.Heights = append(opt.Heights, height)
	}
	log.Info("calc sample height result", "start", start, "end", end, "heights", opt.Heights)
}
