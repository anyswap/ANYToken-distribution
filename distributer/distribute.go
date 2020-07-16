package distributer

import (
	"errors"
	"math/big"
	"time"

	"github.com/anyswap/ANYToken-distribution/callapi"
	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/params"
	"github.com/anyswap/ANYToken-distribution/tools"
)

var capi *callapi.APICaller

const (
	byLiquidMethod = "byliquid"
	byVolumeMethod = "byvolume"
)

var (
	errAccountsLengthMismatch  = errors.New("accounts length mismatch")
	errTotalRewardsIsZero      = errors.New("total rewards is zero")
	errCheckOptionFailed       = errors.New("check option failed")
	errGetAccountListFailed    = errors.New("get account list failed")
	errGetAccountsVolumeFailed = errors.New("get accounts volume failed")
	errNoAccountSatisfied      = errors.New("no account satisfied")
	errSendTransactionFailed   = errors.New("send transaction failed")
)

// SetAPICaller set API caller
func SetAPICaller(apiCaller *callapi.APICaller) {
	capi = apiCaller
}

// Start start distribute
func Start(apiCaller *callapi.APICaller) {
	SetAPICaller(apiCaller)
	config := params.GetConfig()
	for _, distCfg := range config.Distribute {
		if !distCfg.Enable {
			log.Warn("[distribute] ignore disabled config", "config", distCfg)
			continue
		}
		go startDistributeJob(distCfg)
	}
}

func startDistributeJob(distCfg *params.DistributeConfig) {
	log.Info("[distribute] start job", "config", distCfg)

	byLiquidArgs := getBuildTxArgs(byLiquidMethod, distCfg)
	byVolumeArgs := getBuildTxArgs(byVolumeMethod, distCfg)

	exchange := distCfg.Exchange
	rewardToken := distCfg.RewardToken
	start := distCfg.StartHeight
	stable := distCfg.StableHeight

	var (
		byLiquidCycleRewards *big.Int
		byVolumeCycleRewards *big.Int
	)

	byLiquidCycleLen := distCfg.ByLiquidCycle
	byLiquidCycleStart := calcCurCycleStart(start, stable, byLiquidCycleLen)
	if distCfg.ByLiquidRewards != "" {
		byLiquidCycleRewards, _ = tools.GetBigIntFromString(distCfg.ByLiquidRewards)
	}

	byVolumeCycleLen := distCfg.ByVolumeCycle
	byVolumeCycleStart := calcCurCycleStart(start, stable, byVolumeCycleLen)
	if distCfg.ByVolumeRewards != "" {
		byVolumeCycleRewards, _ = tools.GetBigIntFromString(distCfg.ByVolumeRewards)
	}

	waitInterval := 60 * time.Second

	for {
		latestBlock := capi.LoopGetLatestBlockHeader()
		latest := latestBlock.Number.Uint64()
		if latest >= byVolumeCycleStart+byVolumeCycleLen+stable {
			opt := &Option{
				Exchange:    exchange,
				RewardToken: rewardToken,
				TotalValue:  byVolumeCycleRewards,
				StartHeight: byVolumeCycleStart,
				EndHeight:   byVolumeCycleStart + byVolumeCycleLen,
			}
			err := ByVolume(opt, byVolumeArgs)
			if err != nil {
				log.Info("[byvolume] distribute error", "err", err)
			}
			if !shouldRetry(err) {
				if err == errNoAccountSatisfied {
					byLiquidCycleRewards.Add(byLiquidCycleRewards, byVolumeCycleRewards)
				}
				byVolumeCycleStart += byVolumeCycleLen
			} else {
				log.Info("[byvolume] retry as meet error", "opt", opt.String(), "err", err)
			}
		}
		if latest >= byLiquidCycleStart+byLiquidCycleLen+stable {
			opt := &Option{
				Exchange:    exchange,
				RewardToken: rewardToken,
				TotalValue:  byLiquidCycleRewards,
				StartHeight: byLiquidCycleStart,
				EndHeight:   byLiquidCycleStart + byLiquidCycleLen,
			}
			err := ByLiquidity(opt, byLiquidArgs)
			if err != nil {
				log.Info("[byliquid] distribute error", "err", err)
			}
			if !shouldRetry(err) {
				byLiquidCycleStart += byLiquidCycleLen
			} else {
				log.Info("[byliquid] retry as meet error", "opt", opt.String(), "err", err)
			}
		}
		time.Sleep(waitInterval)
	}
}

func shouldRetry(err error) bool {
	switch err {
	case
		errAccountsLengthMismatch,
		errCheckOptionFailed,
		errGetAccountListFailed,
		errGetAccountsVolumeFailed,
		errSendTransactionFailed:
		return true

	case
		nil,
		errTotalRewardsIsZero,
		errNoAccountSatisfied:
		return false

	default:
		log.Error("don't retry with unknown error", "err", err)
		return false
	}
}

func calcCurCycleStart(start, stable, cycleLen uint64) uint64 {
	latestBlock := capi.LoopGetLatestBlockHeader()
	latest := latestBlock.Number.Uint64()
	cycles := (latest - start - stable) / cycleLen
	curCycleStart := start + cycles*cycleLen
	return curCycleStart
}

func getBuildTxArgs(byWhat string, distCfg *params.DistributeConfig) *BuildTxArgs {
	var (
		keystoreFile string
		passwordFile string
		gasLimitPtr  *uint64
		gasPrice     *big.Int
	)
	if distCfg.GasLimit != 0 {
		gasLimitPtr = &distCfg.GasLimit
	}
	if distCfg.GasPrice != "" {
		gasPrice, _ = tools.GetBigIntFromString(distCfg.GasPrice)
	}

	switch byWhat {
	case byLiquidMethod:
		keystoreFile = distCfg.ByLiquidKeystoreFile
		passwordFile = distCfg.ByLiquidPasswordFile
	case byVolumeMethod:
		keystoreFile = distCfg.ByVolumeKeystoreFile
		passwordFile = distCfg.ByVolumePasswordFile
	}

	return &BuildTxArgs{
		KeystoreFile: keystoreFile,
		PasswordFile: passwordFile,
		GasLimit:     gasLimitPtr,
		GasPrice:     gasPrice,
	}
}
