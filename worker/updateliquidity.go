package worker

import (
	"math/big"
	"strings"
	"time"

	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/mongodb"
	"github.com/anyswap/ANYToken-distribution/params"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
	"github.com/fsn-dev/fsn-go-sdk/efsn/core/types"
	"gopkg.in/mgo.v2"
)

const (
	secondsPerDay   = 24 * 3600
	retryDBCount    = 3
	retryDBInterval = 1 * time.Second
)

func tryDoTimes(f func() error) error {
	var err error
	for i := 0; i < retryDBCount; i++ {
		err = f()
		if err == nil || mgo.IsDup(err) {
			return nil
		}
		time.Sleep(retryDBInterval)
	}
	return err
}

func getDayBegin(timestamp uint64) uint64 {
	return timestamp - timestamp%secondsPerDay
}

func timestampToDate(timestamp uint64) string {
	return time.Unix(int64(timestamp), 0).Format("2006-01-02 15:04:05")
}

func updateLiquidityDaily() {
	now := uint64(time.Now().Unix())
	todayBegin := getDayBegin(now)

	for _, ex := range params.GetConfig().Exchanges {
		fromTime := todayBegin
		latest, _ := mongodb.FindLatestLiquidity(ex.Exchange)
		if latest != nil {
			lasttime := getDayBegin(latest.Timestamp)
			if lasttime+secondsPerDay < todayBegin {
				fromTime = lasttime + secondsPerDay
			}
		} else {
			header := loopGetBlockHeader(new(big.Int).SetUint64(ex.CreationHeight))
			fromTime = getDayBegin(header.Time.Uint64())
		}

		timestamp := fromTime
		log.Info("[worker] start updateLiquidityDaily", "fromTime", fromTime)

		for {
			err := updateDateLiquidity(ex, timestamp)
			if err != nil {
				if strings.HasPrefix(err.Error(), "missing trie node") {
					timestamp = todayBegin
					log.Error("[worker] updateLiquidityDaily must query 'archive' node", "err", err)
				} else {
					time.Sleep(rpcRetryInterval)
					continue
				}
			}
			timestamp += secondsPerDay
			now = uint64(time.Now().Unix())
			if timestamp > now {
				time.Sleep(time.Duration(timestamp-now) * time.Second)
			}
		}
	}
}

func updateDateLiquidity(ex *params.ExchangeConfig, timestamp uint64) error {
	exchangeAddr := common.HexToAddress(ex.Exchange)
	tokenAddr := common.HexToAddress(ex.Token)

	blockHeader := findBlockWithTimestamp(timestamp)
	blockNumber := blockHeader.Number
	blockHash := blockHeader.Hash()

	liquidity, err := getExchangeLiquidity(exchangeAddr, blockNumber)
	if err != nil {
		return err
	}

	coins, err := getCoinBalance(exchangeAddr, blockNumber)
	if err != nil {
		return err
	}

	tokens, err := getExchangeTokenBalance(exchangeAddr, tokenAddr, blockNumber)
	if err != nil {
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

	err = tryDoTimes(func() error {
		return mongodb.AddLiquidity(mliq, true)
	})

	if err != nil {
		log.Warn("[worker] updateDateLiquidity error", "err", err)
		return err
	}

	log.Info("[worker] updateDateLiquidity success", "liquidity", mliq, "timestamp", timestampToDate(timestamp))
	return nil
}

func findBlockWithTimestamp(timestamp uint64) *types.Header {
	const acceptRange = 1800
	timeNear := func(blockTimestamp uint64) bool {
		return blockTimestamp > timestamp && blockTimestamp < timestamp+acceptRange
	}

	var (
		blockNumber  *big.Int
		avgBlockTime = params.GetAverageBlockTime()
	)

	for {
		header := loopGetBlockHeader(blockNumber)
		headerTime := header.Time.Uint64()
		if timeNear(headerTime) {
			return header
		}
		if blockNumber == nil {
			blockNumber = header.Number
			if headerTime < timestamp {
				time.Sleep(time.Duration(timestamp-headerTime) * time.Second)
			}
		}
		if headerTime > timestamp {
			countOfBlocks := (headerTime - timestamp) / avgBlockTime
			blockNumber.Sub(blockNumber, new(big.Int).SetUint64(countOfBlocks))
		} else {
			countOfBlocks := (timestamp-headerTime)/avgBlockTime + 1
			blockNumber.Add(blockNumber, new(big.Int).SetUint64(countOfBlocks))
		}
	}
}
