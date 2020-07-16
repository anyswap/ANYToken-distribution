package distributer

import (
	"crypto/rand"
	"fmt"
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
func ByLiquidity(opt *Option, args *BuildTxArgs) error {
	if opt.TotalValue == nil || opt.TotalValue.Sign() <= 0 {
		log.Warn("no liquidity rewards", "option", opt.String())
		return errTotalRewardsIsZero
	}
	opt.byWhat = byLiquidMethod
	opt.buildTxArgs = args
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
	liquids := make([]*big.Int, len(accounts))
	countOfBlocks := opt.EndHeight - opt.StartHeight
	// randomly pick smpale blocks to query liquidity balance, and keep the minimumn
	quarterCount := countOfBlocks/sampleCount + 1
	for i := uint64(0); i < sampleCount; i++ {
		height := opt.StartHeight + i*quarterCount + getRandNumber(quarterCount)
		if height >= opt.EndHeight {
			break
		}
		updateLiquidityBalance(accounts, liquids, height, opt.Exchange)
	}
	return dispatchRewards(opt, accounts, liquids)
}

func updateLiquidityBalance(accounts []common.Address, liquids []*big.Int, height uint64, exchange string) {
	exchangeAddr := common.HexToAddress(exchange)
	blockNumber := new(big.Int).SetUint64(height)
	totalLiquid := big.NewInt(0)
	for i, account := range accounts {
		var value *big.Int
		liquid, err := mongodb.FindLiquidityBalance(exchange, account.String(), height)
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
		log.Info("[byliquid] GetLiquidityBalance success", "exchange", exchange, "account", account.String(), "height", height)
		mliq := &mongodb.MgoLiquidityBalance{
			Key:         mongodb.GetKeyOfLiquidityBalance(exchange, account.String(), height),
			Exchange:    strings.ToLower(exchange),
			Pairs:       params.GetExchangePairs(exchange),
			Account:     strings.ToLower(account.String()),
			BlockNumber: height,
			Liquidity:   value.String(),
		}
		_ = mongodb.TryDoTimes("AddLiquidityBalance "+mliq.Key, func() error {
			return mongodb.AddLiquidityBalance(mliq)
		})
		totalLiquid.Add(totalLiquid, value)
		oldVal := liquids[i]
		if oldVal == nil || oldVal.Cmp(value) > 0 { // get minimumn liquidity balance
			liquids[i] = value
		}
	}
	err := verifyTotalLiquidity(exchangeAddr, blockNumber, totalLiquid)
	if err != nil {
		log.Warn("[byliquid] " + err.Error())
	}
}

func verifyTotalLiquidity(exchangeAddr common.Address, blockNumber, totalLiquid *big.Int) error {
	for {
		totalSupply, err := capi.GetExchangeLiquidity(exchangeAddr, blockNumber)
		if err == nil {
			if totalLiquid.Cmp(totalSupply) != 0 {
				return fmt.Errorf("account list is not complete at height %v. total liqudity %v is not equal to total supply %v", blockNumber, totalLiquid, totalSupply)
			}
			log.Info("[byliquid] account list is complete", "height", blockNumber, "totalsupply", totalSupply)
			return nil
		}
		log.Warn("[byliquid] GetExchangeLiquidity error", "exchange", exchangeAddr.String(), "blockNumber", blockNumber, "err", err)
		time.Sleep(time.Second)
	}
}
