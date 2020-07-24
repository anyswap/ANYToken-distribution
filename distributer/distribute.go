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
// give configed node rewards to liquidity rewards
// check volumes every 100 block,
// if no volume then give configed some of this rewards to liquidity rewards
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

	byLiquidCycleLen := distCfg.ByLiquidCycle
	byLiquidCycleStart := calcCurCycleStart(start, stable, byLiquidCycleLen)
	byLiquidCycleRewards, _ := tools.GetBigIntFromString(distCfg.ByLiquidRewards)
	if byLiquidCycleRewards == nil {
		byLiquidCycleRewards = big.NewInt(0)
	}

	addedNodeRewards, _ := tools.GetBigIntFromString(distCfg.AddNodeRewards)
	if addedNodeRewards != nil {
		byLiquidCycleRewards.Add(byLiquidCycleRewards, addedNodeRewards)
	}

	addedNoVolumeRewardsPerCycle, _ := tools.GetBigIntFromString(distCfg.AddNoVolumeRewards)

	byVolumeCycleLen := distCfg.ByVolumeCycle
	byVolumeCycleRewards, _ := tools.GetBigIntFromString(distCfg.ByVolumeRewards)
	if byVolumeCycleRewards == nil {
		byVolumeCycleRewards = big.NewInt(0)
	}

	totalVolumeRewardsIfNoMissing := new(big.Int).Mul(byVolumeCycleRewards, new(big.Int).SetUint64(byLiquidCycleLen/byVolumeCycleLen))

	opt := &Option{
		Exchange:     exchange,
		RewardToken:  rewardToken,
		DryRun:       distCfg.DryRun,
		StableHeight: stable,
	}

	curCycleStart := byLiquidCycleStart
	for {
		curCycleEnd := curCycleStart + byLiquidCycleLen
		opt.StartHeight = curCycleStart
		opt.EndHeight = curCycleEnd

		missVolumeCycles := waitAndCheckMissVolumeCycles(exchange, curCycleStart, curCycleEnd, stable, byVolumeCycleLen)
		// give configed missing volume rewards to liquidity rewards sender
		addedNoVolumeRewards := new(big.Int).Mul(addedNoVolumeRewardsPerCycle, big.NewInt(missVolumeCycles))
		if addedNoVolumeRewards.Sign() > 0 {
			opt.TotalValue = addedNoVolumeRewards
			opt.BuildTxArgs = byVolumeArgs
			log.Info("start send missing volume rewards", "to", byLiquidArgs.GetSender().String(), "value", addedNoVolumeRewards, "start", opt.StartHeight, "end", opt.EndHeight)
			loopSendMissingVolumeRewards(opt, byLiquidArgs.GetSender())
		}

		// send by liquidity rewards
		opt.TotalValue = new(big.Int).Add(byLiquidCycleRewards, addedNoVolumeRewards)
		opt.BuildTxArgs = byLiquidArgs
		log.Info("start send liquidity reward", "reward", opt.TotalValue, "start", opt.StartHeight, "end", opt.EndHeight)
		loopDoUntilSuccess(ByLiquidity, opt)

		// send by volume rewards
		missVolumeRewards := new(big.Int).Mul(byVolumeCycleRewards, big.NewInt(missVolumeCycles))
		opt.TotalValue = new(big.Int).Sub(totalVolumeRewardsIfNoMissing, missVolumeRewards)
		opt.BuildTxArgs = byVolumeArgs
		log.Info("start send volume reward", "reward", opt.TotalValue, "start", opt.StartHeight, "end", opt.EndHeight)
		loopDoUntilSuccess(ByVolume, opt)

		// start next cycle
		curCycleStart = curCycleEnd
		log.Info("start next cycle", "start", curCycleStart)
	}
}

func loopSendMissingVolumeRewards(opt *Option, to common.Address) {
	from := opt.GetSender()
	value := opt.TotalValue
	waitInterval := 60 * time.Second
	for {
		txHash, err := opt.SendRewardsTransaction(to, value)
		if err == nil {
			var txHashStr string
			if txHash != nil {
				txHashStr = txHash.String()
			}
			log.Info("send missing volume rewards success", "from", from.String(), "to", to.String(), "value", value, "start", opt.StartHeight, "end", opt.EndHeight, "txHash", txHashStr, "dryrun", opt.DryRun)
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
		log.Info("wait to cycle end", "exchange", exchange, "cycleStart", cycleStart, "cycleEnd", cycleEnd, "stable", stable, "latest", latest)

		for latest >= start+step+stable {
			accountStats := mongodb.FindAccountVolumes(exchange, start, start+step)
			if len(accountStats) == 0 {
				log.Info("find miss volume cycle", "exchange", exchange, "start", start, "end", start+step)
				missCycles++
			}
			log.Info("has trades in range", "start", start, "end", start+step)
			start += step // next by volume cycle
		}

		if latest >= cycleEnd+stable {
			break
		}
	}
	log.Info("cycle end is achieved", "exchange", exchange, "cycleStart", cycleStart, "cycleEnd", cycleEnd, "stable", stable, "novolumes", missCycles)
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
	err := args.Check(distCfg.DryRun)
	if err != nil {
		log.Error("check build tx args failed", "err", err)
		return nil, err
	}
	return args, nil
}
