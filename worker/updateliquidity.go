package worker

import (
	"math/big"
	"strings"
	"time"

	"github.com/anyswap/ANYToken-distribution/distributer"
	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/mongodb"
	"github.com/anyswap/ANYToken-distribution/params"
	"github.com/anyswap/ANYToken-distribution/syncer"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
)

const (
	secondsPerDay = 24 * 3600
)

func getDayBegin(timestamp uint64) uint64 {
	return timestamp - timestamp%secondsPerDay
}

func timestampToDate(timestamp uint64) string {
	return time.Unix(int64(timestamp), 0).Format("2006-01-02 15:04:05")
}

func updateLiquidityDaily() {
	if !params.GetConfig().Sync.UpdateLiquidity {
		return
	}
	if !syncer.IsEndlessLoop() {
		return
	}
	go updateLiquidityDailyLoop()
}

func updateLiquidityDailyLoop() {
	for {
		now := uint64(time.Now().Unix())
		todayBegin := getDayBegin(now)

		updateLiquidityDailyOnce(todayBegin)

		now = uint64(time.Now().Unix())
		if now < todayBegin+secondsPerDay {
			time.Sleep(time.Duration(todayBegin+secondsPerDay-now) * time.Second)
		}
	}
}

func updateLiquidityDailyOnce(todayBegin uint64) {
	for _, ex := range params.GetConfig().Exchanges {
		var fromTime uint64
		latest, _ := mongodb.FindLatestLiquidity(ex.Exchange)
		if latest != nil {
			lasttime := getDayBegin(latest.Timestamp)
			fromTime = lasttime + secondsPerDay
		} else {
			header := capi.LoopGetBlockHeader(new(big.Int).SetUint64(ex.CreationHeight))
			fromTime = getDayBegin(header.Time.Uint64())
		}
		if fromTime > todayBegin {
			continue
		}

		timestamp := fromTime
		log.Info("[worker] start updateLiquidityDaily", "exchange", ex, "fromTime", fromTime)

		for timestamp <= todayBegin {
			err := updateDateLiquidity(ex, timestamp)
			if err == nil {
				timestamp += secondsPerDay
				continue
			}
			if strings.HasPrefix(err.Error(), "missing trie node") {
				log.Error("[worker] updateLiquidityDaily must query 'archive' node", "err", err)
				break
			}
			time.Sleep(time.Second)
		}
	}
}

func updateDateLiquidity(ex *params.ExchangeConfig, timestamp uint64) error {
	exchangeAddr := common.HexToAddress(ex.Exchange)
	tokenAddr := common.HexToAddress(ex.Token)

	blockHeader := distributer.FindBlockByTimestamp(timestamp)
	blockNumber := blockHeader.Number
	blockHash := blockHeader.Hash()

	liquidity, err := capi.GetExchangeLiquidity(exchangeAddr, blockNumber)
	if err != nil {
		log.Warn("[worker] updateDateLiquidity error", "err", err)
		return err
	}

	coins, err := capi.GetCoinBalance(exchangeAddr, blockNumber)
	if err != nil {
		log.Warn("[worker] updateDateLiquidity error", "err", err)
		return err
	}

	tokens, err := capi.GetExchangeTokenBalance(exchangeAddr, tokenAddr, blockNumber)
	if err != nil {
		log.Warn("[worker] updateDateLiquidity error", "err", err)
		return err
	}

	mliq := &mongodb.MgoLiquidity{}
	mliq.Key = mongodb.GetKeyOfExchangeAndTimestamp(ex.Exchange, timestamp)
	mliq.Exchange = ex.Exchange
	mliq.Pairs = ex.Pairs
	mliq.Coin = coins.String()
	mliq.Token = tokens.String()
	mliq.Liquidity = liquidity.String()
	mliq.BlockNumber = blockNumber.Uint64()
	mliq.BlockHash = blockHash.String()
	mliq.Timestamp = timestamp

	err = mongodb.TryDoTimes("AddLiquidity "+mliq.Key, func() error {
		return mongodb.AddLiquidity(mliq, true)
	})

	if err != nil {
		log.Warn("[worker] updateDateLiquidity error", "err", err)
		return err
	}

	log.Info("[worker] updateDateLiquidity success", "liquidity", mliq, "timestamp", timestampToDate(timestamp))
	return nil
}
