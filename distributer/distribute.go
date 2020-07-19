package distributer

import (
	"errors"
	"math/big"
	"time"

	"github.com/anyswap/ANYToken-distribution/callapi"
	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/mongodb"
	"github.com/anyswap/ANYToken-distribution/params"
	"github.com/anyswap/ANYToken-distribution/tools"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
)

var capi *callapi.APICaller

const (
	byLiquidMethod = "byliquid"
	byVolumeMethod = "byvolume"
)

var (
	errTotalRewardsIsZero       = errors.New("total rewards is zero")
	errCheckOptionFailed        = errors.New("check option failed")
	errGetAccountListFailed     = errors.New("get account list failed")
	errGetAccountsRewardsFailed = errors.New("get accounts rewards failed")
	errNoAccountSatisfied       = errors.New("no account satisfied")
	errSendTransactionFailed    = errors.New("send transaction failed")
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

// every 6600 blocks distribute:
// 	1. by liquidity rewards
// 	2. by volume rewards
// check volumes every 100 block,
// if no volume then give this rewards to liquidity rewards
func startDistributeJob(distCfg *params.DistributeConfig) {
	log.Info("[distribute] start job", "config", distCfg)

	byLiquidArgs, err := getBuildTxArgs(byLiquidMethod, distCfg)
	if err != nil {
		return
	}

	byVolumeArgs, err := getBuildTxArgs(byVolumeMethod, distCfg)
	if err != nil {
		return
	}

	exchange := distCfg.Exchange
	rewardToken := distCfg.RewardToken
	start := distCfg.StartHeight
	stable := distCfg.StableHeight

	var (
		byLiquidCycleRewards *big.Int
		byVolumeCycleRewards *big.Int

		totalVolumeRewards *big.Int
		missVolumeRewards  *big.Int
	)

	byLiquidCycleLen := distCfg.ByLiquidCycle
	byLiquidCycleStart := calcCurCycleStart(start, stable, byLiquidCycleLen)
	if distCfg.ByLiquidRewards != "" {
		byLiquidCycleRewards, _ = tools.GetBigIntFromString(distCfg.ByLiquidRewards)
	}

	byVolumeCycleLen := distCfg.ByVolumeCycle
	if distCfg.ByVolumeRewards != "" {
		byVolumeCycleRewards, _ = tools.GetBigIntFromString(distCfg.ByVolumeRewards)
	}
	totalVolumeRewards = new(big.Int).Mul(byVolumeCycleRewards, new(big.Int).SetUint64(byLiquidCycleLen/byVolumeCycleLen))

	opt := &Option{
		Exchange:    exchange,
		RewardToken: rewardToken,
	}

	curCycleStart := byLiquidCycleStart
	for {
		curCycleEnd := curCycleStart + byLiquidCycleLen
		opt.StartHeight = curCycleStart
		opt.EndHeight = curCycleEnd

		missVolumeCycles := waitAndCheckMissVolumeCycles(exchange, curCycleStart, curCycleEnd, stable, byVolumeCycleLen)
		missVolumeRewards = new(big.Int).Mul(byVolumeCycleRewards, big.NewInt(missVolumeCycles))

		// give missing volume rewards to liquidity rewards sender
		if missVolumeRewards.Sign() > 0 {
			opt.TotalValue = missVolumeRewards
			opt.BuildTxArgs = byVolumeArgs
			loopSendMissingVolumeRewards(opt, byLiquidArgs.GetSender())
		}

		opt.TotalValue = new(big.Int).Add(byLiquidCycleRewards, missVolumeRewards)
		opt.BuildTxArgs = byLiquidArgs
		loopDoUntilSuccess(ByLiquidity, opt)

		opt.TotalValue = new(big.Int).Sub(totalVolumeRewards, missVolumeRewards)
		opt.BuildTxArgs = byVolumeArgs
		loopDoUntilSuccess(ByVolume, opt)

		curCycleStart += byVolumeCycleLen
	}
}

func loopSendMissingVolumeRewards(opt *Option, to common.Address) {
	from := opt.GetSender()
	value := opt.TotalValue
	waitInterval := 60 * time.Second
	for {
		txHash, err := opt.SendRewardsTransaction(to, value)
		if err == nil {
			log.Info("send missing volume rewards success", "from", from.String(), "to", to.String(), "value", value, "start", opt.StartHeight, "end", opt.EndHeight, "txHash", txHash.String())
			break
		}
		log.Info("send missing volume rewards failed", "from", from.String(), "to", to.String(), "value", value, "start", opt.StartHeight, "end", opt.EndHeight, "err", err)
		time.Sleep(waitInterval)
	}
}

func loopDoUntilSuccess(distributeFunc func(*Option) error, opt *Option) {
	waitInterval := 60 * time.Second
	for {
		err := distributeFunc(opt)
		if err != nil {
			log.Info("distribute error", "byWhat", opt.ByWhat(), "err", err)
		}
		if !shouldRetry(err) {
			break
		}
		log.Info("retry as meet error", "opt", opt.String(), "err", err)
		time.Sleep(waitInterval)
	}
}

func waitAndCheckMissVolumeCycles(exchange string, cycleStart, cycleEnd, stable, step uint64) (missCycles int64) {
	waitInterval := 60 * time.Second
	start := cycleStart
	for {
		time.Sleep(waitInterval)
		latestBlock := capi.LoopGetLatestBlockHeader()
		latest := latestBlock.Number.Uint64()

		for latest >= start+step+stable {
			accounts, _ := mongodb.FindAccountVolumes(exchange, start, start+step)
			if len(accounts) == 0 {
				log.Info("find miss volume cycle", "exchange", exchange, "start", start, "end", start+step)
				missCycles++
			}
			start += step // next by volume cycle
		}

		if latest >= cycleEnd+stable {
			break
		}
	}
	return missCycles
}

func shouldRetry(err error) bool {
	switch err {
	case
		errCheckOptionFailed,
		errGetAccountListFailed,
		errGetAccountsRewardsFailed,
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

func getBuildTxArgs(byWhat string, distCfg *params.DistributeConfig) (*BuildTxArgs, error) {
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

	args := &BuildTxArgs{
		KeystoreFile: keystoreFile,
		PasswordFile: passwordFile,
		GasLimit:     gasLimitPtr,
		GasPrice:     gasPrice,
	}
	err := args.Check()
	if err != nil {
		log.Error("check build tx args failed", "err", err)
		return nil, err
	}
	return args, nil
}
