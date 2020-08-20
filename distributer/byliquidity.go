package distributer

import (
	"fmt"
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
	if len(opt.Heights) == 0 {
		opt.calcSampleHeights()
	}
	_ = opt.WriteLiquiditySubject(opt.Exchange, opt.StartHeight, opt.EndHeight, len(accounts))
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

	// pick smpale blocks to query liquidity balance, and keep the minimumn
	for i, height := range opt.Heights {
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
		err := verifyTotalLiquidity(exchangeAddr, blockNumber, totalLiquid, i == 0)
		if err != nil {
			log.Warn("[byliquid] " + err.Error())
		}
	}

	return mongodb.ConvertToSortedSlice(finStatMap)
}

// StoreLiquidityBalanceAtHeight store liquidity balance at specified height
func StoreLiquidityBalanceAtHeight(exchange string, height uint64) {
	accounts := mongodb.FindAllAccounts(exchange)
	if len(accounts) == 0 {
		return
	}

	exchangeAddr := common.HexToAddress(exchange)

	blockNumber := new(big.Int).SetUint64(height)
	totalLiquid := big.NewInt(0)
	for _, account := range accounts {
		var value *big.Int
		accoutStr := strings.ToLower(account.String())
		liquid, err := mongodb.FindLiquidityBalance(exchange, accoutStr, height)
		if err == nil {
			value, _ = tools.GetBigIntFromString(liquid)
		}
		for value == nil {
			value, err = capi.GetLiquidityBalance(exchangeAddr, account, blockNumber)
			if err != nil {
				log.Warn("[store] GetLiquidityBalance error", "err", err)
				time.Sleep(time.Second)
				continue
			}
		}
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
		totalLiquid.Add(totalLiquid, value)
	}
	err := verifyTotalLiquidity(exchangeAddr, blockNumber, totalLiquid, true)
	if err != nil {
		log.Warn("[store] verify total liquidity equal failed", "err", err)
	}
}

func verifyTotalLiquidity(exchangeAddr common.Address, blockNumber, totalLiquid *big.Int, strict bool) error {
	for {
		totalSupply, err := capi.GetExchangeLiquidity(exchangeAddr, blockNumber)
		if err != nil {
			log.Warn("[byliquid] GetExchangeLiquidity error", "exchange", exchangeAddr.String(), "blockNumber", blockNumber, "err", err)
			time.Sleep(time.Second)
			continue
		}
		if totalLiquid.Cmp(totalSupply) == 0 {
			log.Info("[byliquid] account list is complete", "height", blockNumber, "totalsupply", totalSupply)
			return nil
		}
		if strict {
			return fmt.Errorf("[byliquid] account list is not complete. height=%v totalsupply=%v totalLiquid=%v", blockNumber, totalSupply, totalLiquid)
		}
		return nil
	}
}

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
