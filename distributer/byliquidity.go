package distributer

import (
	"math/big"
	"math/rand"
	"strings"

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

func (opt *Option) getLiquidityBalances(accountsSlice [][]common.Address) (accountStats []mongodb.AccountStatSlice) {
	accountStats = make([]mongodb.AccountStatSlice, len(opt.Exchanges))
	for i, exchange := range opt.Exchanges {
		accounts := accountsSlice[i]
		WriteLiquiditySubject(exchange, opt.StartHeight, opt.EndHeight, len(accounts))
		stats, _ := opt.getLiquidityBalancesOfExchange(exchange, accounts)
		totalLiquids := stats.CalcTotalShare()
		WriteLiquiditySummary(exchange, opt.StartHeight, opt.EndHeight, len(stats), totalLiquids, opt.TotalValue)
		for _, stat := range stats {
			WriteLiquidityBalance(stat.Account, stat.Share, stat.Number)
		}
		accountStats[i] = stats
	}
	return accountStats
}

func (opt *Option) getLiquidityBalancesOfExchange(exchange string, accounts []common.Address) (accountStats mongodb.AccountStatSlice, complete bool) {
	exchangeAddr := common.HexToAddress(exchange)

	finStatMap := make(map[common.Address]*mongodb.AccountStat)

	height := opt.SampleHeight
	var blockNumber *big.Int
	if !opt.ArchiveMode {
		latestBlock := capi.LoopGetLatestBlockHeader()
		height = latestBlock.Number.Uint64()
		blockNumber = nil // use latest block in non archive mode
		log.Warn("get liquidity balance in non archive mode", "latest", height)
	} else {
		blockNumber = new(big.Int).SetUint64(height)
	}
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
	diffLiquid := new(big.Int).Sub(totalSupply, totalLiquid)
	diffLiquid = diffLiquid.Abs(diffLiquid)
	if new(big.Int).Mul(diffLiquid, big.NewInt(20)).Cmp(totalLiquid) <= 0 { // allow 5% diff
		complete = true
	}
	log.Info("[byliquid] check if account list is complete", "exchange", exchange, "smaple", height, "totalsupply", totalSupply, "totalLiquid", totalLiquid, "diffLiquid", diffLiquid)

	return mongodb.ConvertToSortedSlice(finStatMap), complete
}

// CalcSampleHeight calc sample height
func (opt *Option) CalcSampleHeight() {
	if opt.SampleHeight != 0 || !opt.ArchiveMode {
		return
	}
	opt.SampleHeight = CalcRandomSampleHeight(opt.StartHeight, opt.EndHeight, opt.UseTimeMeasurement)
}

// CalcRandomSampleHeight calc random sample height base on start
func CalcRandomSampleHeight(start, end uint64, useTimeMeasurement bool) (sampleHeight uint64) {
	sample := CalcRandomSample(start, end, useTimeMeasurement)
	if useTimeMeasurement {
		sampleHeight = getBlockHeightByTime(sample)
	} else {
		sampleHeight = sample
	}
	log.Info("calc random sample height result", "start", start, "end", end, "useTimeMeasurement", useTimeMeasurement, "sample", sample, "sampleHeight", sampleHeight)
	return sampleHeight
}

// CalcRandomSample calc random sample (height or timestamp) base on start
func CalcRandomSample(start, end uint64, useTimeMeasurement bool) (sample uint64) {
	head := (end - start) / 3
	tail := end - start - head
	startHeight := start
	if useTimeMeasurement {
		startHeight = getBlockHeightByTime(start)
	}
	randTail := getRandNumber(startHeight, tail)
	sample = start + head + randTail
	log.Info("calc random sample height or timestamp result", "start", start, "end", end, "useTimeMeasurement", useTimeMeasurement, "sample", sample)
	return sample
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
