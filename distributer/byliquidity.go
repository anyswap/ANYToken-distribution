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
	opt.CalcSampleHeight()
	err := opt.checkAndInit()
	defer opt.deinit()
	if err != nil {
		log.Error("[byliquid] check option error", "option", opt.String(), "err", err)
		return errCheckOptionFailed
	}
	accountStats, err := opt.GetAccountsAndShares()
	if err != nil {
		log.Error("[byliquid] GetAccountsAndShares error", "err", err)
		return errGetAccountsSharesFailed
	}
	if len(accountStats) == 0 {
		accounts, err := opt.getAccounts()
		if err != nil {
			log.Error("[byliquid] get accounts error", "err", err)
			return errGetAccountListFailed
		}
		accountStats = opt.getLiquidityBalances(accounts)
	}
	if len(accountStats) != len(opt.Exchanges) {
		log.Warn("[byliquid] account list is not complete. " + opt.String())
		return errAccountsNotComplete
	}
	mongodb.CalcWeightedRewards(accountStats, opt.TotalValue, opt.Weights)
	return opt.dispatchRewards(accountStats)
}

func (opt *Option) getLiquidityBalances(accounts [][]common.Address) (accountStats []mongodb.AccountStatSlice) {
	accountStats = opt.updateLiquidityBalances(accounts)
	return accountStats
}

func (opt *Option) updateLiquidityBalances(accountsSlice [][]common.Address) (accountStats []mongodb.AccountStatSlice) {
	accountStats = make([]mongodb.AccountStatSlice, len(opt.Exchanges))
	for i, exchange := range opt.Exchanges {
		accounts := accountsSlice[i]
		WriteLiquiditySubject(exchange, opt.StartHeight, opt.EndHeight, len(accounts))
		stats, _ := opt.updateLiquidityBalance(exchange, accounts)
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

	height := opt.SampleHeight
	blockNumber := new(big.Int).SetUint64(height)
	totalSupply := capi.LoopGetExchangeLiquidity(exchangeAddr, blockNumber)
	exCoinBalance := capi.LoopGetCoinBalance(exchangeAddr, blockNumber)
	log.Info("get exchange liquidity and coin balance", "totalSupply", totalSupply, "exCoinBalance", exCoinBalance, "blockNumber", blockNumber)
	totalLiquid := big.NewInt(0)
	totalCoinBalance := big.NewInt(0)
	for _, account := range accounts {
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
		WriteLiquidityBalance(account, value, height)
		// convert liquid balance to coin balance
		coinBalance := new(big.Int).Mul(value, exCoinBalance)
		if totalSupply.Sign() > 0 {
			coinBalance.Div(coinBalance, totalSupply)
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
		if params.IsExcludedRewardAccount(account) {
			continue
		}
		totalCoinBalance.Add(totalCoinBalance, coinBalance)
		finStat, exist := finStatMap[account]
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

	return mongodb.ConvertToSortedSlice(finStatMap), complete
}

func distLeftValue(finStatMap map[common.Address]*mongodb.AccountStat, leftValue *big.Int) {
	if leftValue.Sign() <= 0 {
		return
	}
	stats := mongodb.ConvertToSortedSlice(finStatMap)
	if len(stats) == 0 {
		return
	}
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

// CalcSampleHeight calc sample height
func (opt *Option) CalcSampleHeight() {
	if opt.SampleHeight != 0 {
		return
	}
	start := opt.StartHeight
	end := opt.EndHeight
	opt.SampleHeight = calcSampleHeightImpl(start, end, opt.UseTimeMeasurement)
	log.Info("calc sample height result", "start", start, "end", end, "sample", opt.SampleHeight)
}

func calcSampleHeightImpl(start, end uint64, useTimeMeasurement bool) (height uint64) {
	head := (end - start) / 3
	tail := end - start - head
	startHeight := start
	if useTimeMeasurement {
		startHeight = getBlockHeightByTime(start)
	}
	randTail := getRandNumber(startHeight, tail)
	return start + head + randTail
}

// nolint:gosec // use of weak random number generator math/rand intentionally
func getRandNumber(seedBlock, max uint64) (number uint64) {
	log.Info("start get random number", "seedBlock", seedBlock, "max", max)
	header := capi.LoopGetBlockHeader(new(big.Int).SetUint64(seedBlock))
	log.Info("get seed block hash success", "hash", header.Hash().String())
	seadHash := common.Keccak256Hash(header.Hash().Bytes(), header.Number.Bytes(), []byte("anyswap"))
	rand.Seed(new(big.Int).SetBytes(seadHash.Bytes()).Int64())
	number = uint64(rand.Intn(int(max)))
	log.Info("get random numbers success", "seedBlock", seedBlock, "max", max, "number", number)
	return number
}

func getBlockHeightByTime(timestamp uint64) uint64 {
	block := FindBlockByTimestamp(timestamp)
	return block.Number.Uint64()
}
