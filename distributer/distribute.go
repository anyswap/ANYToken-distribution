package distributer

import (
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/anyswap/ANYToken-distribution/callapi"
	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/mongodb"
	"github.com/anyswap/ANYToken-distribution/params"
	"github.com/anyswap/ANYToken-distribution/syncer"
	"github.com/anyswap/ANYToken-distribution/tools"
)

var capi *callapi.APICaller

const (
	byLiquidMethodID      = "liquidity"
	byLiquidMethodAliasID = "liquid"
	byVolumeMethodID      = "volume"
	byVolumeMethodAliasID = "trade"
	customMethodID        = "custom"
)

// Calc rewards type
const (
	CalcBothRewards   = "both"
	CalcVolumeRewards = "volume"
	CalcLiquidRewards = "liquid"
)

var (
	errTotalRewardsIsZero       = errors.New("total rewards is zero")
	errCheckOptionFailed        = errors.New("check option failed")
	errGetAccountListFailed     = errors.New("get account list failed")
	errGetAccountsRewardsFailed = errors.New("get accounts rewards failed")
	errGetAccountsSharesFailed  = errors.New("get accounts shares failed")
	errAccountsNotComplete      = errors.New("account list is not complete")
	errSendTransactionFailed    = errors.New("send transaction failed")
)

// IsCustomMethod is custom method
func IsCustomMethod(method string) bool {
	return method == customMethodID
}

// SetAPICaller set API caller
func SetAPICaller(apiCaller *callapi.APICaller) {
	capi = apiCaller
}

// Start start distribute
// every 6600 blocks distribute:
// 	1. by liquidity rewards
// 	2. by volume rewards
// give configed node rewards to liquidity rewards
// check volumes every 100 block,
// if no volume then give configed some of this rewards to liquidity rewards
func Start(apiCaller *callapi.APICaller) {
	SetAPICaller(apiCaller)
	config := params.GetConfig()
	distCfg := config.Distribute
	if !distCfg.Enable {
		log.Warn("[distribute] stop distribute as it's disabled")
		return
	}

	log.Info("[distribute] start job", "config", distCfg)

	runner, err := initDistributer(distCfg)
	if err != nil {
		log.Error("[distribute] start failed", "err", err)
		return
	}

	go runner.run()
}

type distributeRunner struct {
	liquidExchanges []string
	liquidWeights   []uint64

	tradeExchanges []string
	tradeWeights   []uint64
	sampleHeights  []uint64

	rewardToken string
	start       uint64
	stable      uint64

	byLiquidCycleLen     uint64
	byLiquidCycleRewards *big.Int

	byVolumeCycleLen     uint64
	byVolumeCycleRewards *big.Int
	totalVolumeCycles    uint64
	totalVolumeRewards   *big.Int

	quickSettleVolumeRewards bool
	useTimeMeasurement       bool

	byLiquidArgs *BuildTxArgs
	byVolumeArgs *BuildTxArgs
}

func initDistributer(distCfg *params.DistributeConfig) (*distributeRunner, error) {
	var err error
	runner := &distributeRunner{}

	runner.byLiquidArgs, err = getBuildTxArgs(distCfg)
	if err != nil {
		return nil, err
	}

	runner.byVolumeArgs, err = getBuildTxArgs(distCfg)
	if err != nil {
		return nil, err
	}

	if distCfg.UseTimeMeasurement {
		runner.useTimeMeasurement = true
		runner.start = distCfg.StartTimestamp
		runner.stable = distCfg.StableDuration
		runner.byLiquidCycleLen = distCfg.ByLiquidCycleDuration
		runner.byVolumeCycleLen = distCfg.ByVolumeCycleDuration
	} else {
		runner.start = distCfg.StartHeight
		runner.stable = distCfg.StableHeight
		runner.byLiquidCycleLen = distCfg.ByLiquidCycle
		runner.byVolumeCycleLen = distCfg.ByVolumeCycle
	}

	if runner.byLiquidCycleLen == 0 {
		return nil, fmt.Errorf("liquidity cycle length is zero")
	}
	if runner.byVolumeCycleLen == 0 {
		return nil, fmt.Errorf("volume cycle length is zero")
	}
	if runner.byLiquidCycleLen%runner.byVolumeCycleLen != 0 {
		return nil, fmt.Errorf("liquid cycle %v is not multiple intergral of volume cycle %v", runner.byLiquidCycleLen, runner.byVolumeCycleLen)
	}

	runner.rewardToken = distCfg.RewardToken

	runner.byLiquidCycleRewards = distCfg.GetByLiquidCycleRewards()
	runner.byVolumeCycleRewards = distCfg.GetByVolumeCycleRewards()
	runner.quickSettleVolumeRewards = distCfg.QuickSettleVolumeRewards
	runner.totalVolumeCycles = runner.byLiquidCycleLen / runner.byVolumeCycleLen
	runner.totalVolumeRewards = new(big.Int).Mul(runner.byVolumeCycleRewards, new(big.Int).SetUint64(runner.totalVolumeCycles))

	for _, exchange := range params.GetConfig().Exchanges {
		if exchange.LiquidWeight > 0 {
			runner.liquidExchanges = append(runner.liquidExchanges, exchange.Exchange)
			runner.liquidWeights = append(runner.liquidWeights, exchange.LiquidWeight)
		}
		if exchange.TradeWeight > 0 {
			runner.tradeExchanges = append(runner.tradeExchanges, exchange.Exchange)
			runner.tradeWeights = append(runner.tradeWeights, exchange.TradeWeight)
		}
	}

	log.Info("init distributer runner finish",
		"byVolumeCycleLen", runner.byVolumeCycleLen,
		"byVolumeCycleRewards", runner.byVolumeCycleRewards,
		"byLiquidCycleLen", runner.byLiquidCycleLen,
		"byLiquidCycleRewards", runner.byLiquidCycleRewards,
		"liquidExchanges", len(runner.liquidExchanges),
		"tradeExchanges", len(runner.tradeExchanges),
		"quickSettleVolumeRewards", runner.quickSettleVolumeRewards,
		"useTimeMeasurement", runner.useTimeMeasurement,
	)

	return runner, nil
}

func waitNodeSyncFinish() {
	for {
		isNodeSyncing := capi.IsNodeSyncing()
		if !isNodeSyncing {
			break
		}
		log.Warn("wait node syncing finish in process")
		time.Sleep(60 * time.Second)
	}
	log.Info("wait node syncing finish success")
}

func (runner *distributeRunner) run() {
	waitNodeSyncFinish()
	syncer.WaitSyncToLatest()
	curCycleStart := calcCurCycleStart(runner.start, runner.stable, runner.byLiquidCycleLen, runner.useTimeMeasurement)
	for {
		curCycleEnd := curCycleStart + runner.byLiquidCycleLen
		missVolumeCycles, err := runner.settleVolumeRewards(curCycleStart, curCycleEnd)
		if missVolumeCycles != 0 {
			log.Warn("found missing volume cycles", "start", curCycleStart, "end", curCycleEnd, "missing", missVolumeCycles)
		}
		if err == nil {
			_ = runner.sendLiquidRewards(runner.byLiquidCycleRewards, curCycleStart, curCycleEnd)
		}

		// start next cycle
		curCycleStart = curCycleEnd
		log.Info("start next cycle", "start", curCycleStart)
	}
}

func (runner *distributeRunner) settleVolumeRewards(cycleStart, cycleEnd uint64) (uint64, error) {
	if !runner.quickSettleVolumeRewards {
		waitCycleEnd("liquid", cycleStart, cycleEnd, runner.stable, 60*time.Second, runner.useTimeMeasurement)
		return runner.sendVolumeRewards(runner.totalVolumeRewards, cycleStart, cycleEnd)
	}
	latest := calcLatestBlockNumberOrTimestamp(runner.useTimeMeasurement)
	var missVolumeCycles uint64
	step := runner.byVolumeCycleLen
	for start := cycleStart; start < cycleEnd; start += step {
		if start+step < latest {
			continue
		}
		waitCycleEnd("trade", start, start+step, runner.stable, 20*time.Second, runner.useTimeMeasurement)
		missing, err := runner.sendVolumeRewards(runner.byVolumeCycleRewards, start, start+step)
		if err != nil {
			continue
		}
		missVolumeCycles += missing
	}
	log.Info("settleVolumeRewards finish", "start", cycleStart, "end", cycleEnd, "novolumes", missVolumeCycles)
	return missVolumeCycles, nil
}

func (runner *distributeRunner) sendVolumeRewards(rewards *big.Int, start, end uint64) (missVolumeCycles uint64, err error) {
	if len(runner.tradeExchanges) == 0 {
		return 0, nil
	}
	opt := &Option{
		BuildTxArgs:        runner.byVolumeArgs,
		TotalValue:         rewards,
		StartHeight:        start,
		EndHeight:          end,
		StableHeight:       runner.stable,
		StepCount:          runner.byVolumeCycleLen,
		StepReward:         runner.byVolumeCycleRewards,
		Exchanges:          runner.tradeExchanges,
		Weights:            runner.tradeWeights,
		RewardToken:        runner.rewardToken,
		DryRun:             true,
		UseTimeMeasurement: runner.useTimeMeasurement,
	}
	log.Info("start send volume reward", "option", opt.String())
	err = ByVolume(opt)
	if err != nil {
		log.Error("send volume reward failed", "err", err)
		return 0, err
	}
	log.Info("send volume reward success", "start", start, "end", end, "rewards", rewards)
	return opt.noVolumes, err
}

func (runner *distributeRunner) sendLiquidRewards(rewards *big.Int, start, end uint64) error {
	if len(runner.liquidExchanges) == 0 {
		return nil
	}
	opt := &Option{
		BuildTxArgs:        runner.byLiquidArgs,
		TotalValue:         rewards,
		StartHeight:        start,
		EndHeight:          end,
		StableHeight:       runner.stable,
		Exchanges:          runner.liquidExchanges,
		Weights:            runner.liquidWeights,
		Heights:            runner.sampleHeights,
		RewardToken:        runner.rewardToken,
		DryRun:             true,
		UseTimeMeasurement: runner.useTimeMeasurement,
	}
	log.Info("start send liquid reward", "option", opt.String())
	err := ByLiquidity(opt)
	if err != nil {
		log.Error("send liquid reward failed", "err", err)
		return err
	}
	log.Info("send liquid reward success", "start", start, "end", end, "rewards", rewards)
	return nil
}

func waitCycleEnd(cycleName string, cycleStart, cycleEnd, stable uint64, waitInterval time.Duration, useTimeMeasurement bool) {
	latest := uint64(0)
	for {
		syncInfo, err := mongodb.FindLatestSyncInfo()
		if err != nil {
			log.Warn("find latest sync info failed", "err", err)
			continue
		}
		if useTimeMeasurement {
			latest = syncInfo.Timestamp
		} else {
			latest = syncInfo.Number
		}

		if latest >= cycleEnd+stable {
			break
		}
		log.Info(fmt.Sprintf("wait to %v cycle end", cycleName), "cycleStart", cycleStart, "cycleEnd", cycleEnd, "stable", stable, "latest", latest)
		time.Sleep(waitInterval)
	}
	log.Info(fmt.Sprintf("%v cycle end is achieved", cycleName), "cycleStart", cycleStart, "cycleEnd", cycleEnd, "stable", stable, "latest", latest)
}

func calcLatestBlockNumberOrTimestamp(useTimeMeasurement bool) uint64 {
	latestBlock := capi.LoopGetLatestBlockHeader()
	var latest uint64
	if useTimeMeasurement {
		latest = latestBlock.Time.Uint64()
	} else {
		latest = latestBlock.Number.Uint64()
	}
	return latest
}

func calcCurCycleStart(start, stable, cycleLen uint64, useTimeMeasurement bool) uint64 {
	latest := calcLatestBlockNumberOrTimestamp(useTimeMeasurement)
	cycles := (latest - start - stable) / cycleLen
	curCycleStart := start + cycles*cycleLen
	return curCycleStart
}

func getBuildTxArgs(distCfg *params.DistributeConfig) (*BuildTxArgs, error) {
	var (
		gasLimitPtr *uint64
		gasPrice    *big.Int
	)
	if distCfg.GasLimit != 0 {
		gasLimitPtr = &distCfg.GasLimit
	}
	if distCfg.GasPrice != "" {
		gasPrice, _ = tools.GetBigIntFromString(distCfg.GasPrice)
	}

	args := &BuildTxArgs{
		GasLimit: gasLimitPtr,
		GasPrice: gasPrice,
	}
	err := args.Check(true)
	if err != nil {
		log.Error("check build tx args failed", "err", err)
		return nil, err
	}
	return args, nil
}

// CalcRewards calc rewards
func CalcRewards(startHeight, endHeight uint64, sampleHeights []uint64, calcType string) error {
	log.Info("[CalcRewards] call", "startHeight", startHeight, "endHeight", endHeight, "sampleHeights", sampleHeights, "calcType", calcType)
	distCfg := params.GetConfig().Distribute

	log.Info("[CalcRewards] start job", "config", distCfg)

	runner, err := initDistributer(distCfg)
	if err != nil {
		log.Error("[CalcRewards] start failed", "err", err)
		return err
	}

	err = runner.checkStartEndHeight(startHeight, endHeight, calcType)
	if err != nil {
		return err
	}

	err = runner.checkSampleHeights(sampleHeights, startHeight, endHeight)
	if err != nil {
		return err
	}

	calcVolumeRewards := calcType == CalcVolumeRewards || calcType == CalcBothRewards
	calcLiquidRewards := calcType == CalcLiquidRewards || calcType == CalcBothRewards

	if calcVolumeRewards {
		missingCycles := uint64(0)
		if !runner.quickSettleVolumeRewards {
			waitCycleEnd("tradeWhole", startHeight, endHeight, runner.stable, 60*time.Second, runner.useTimeMeasurement)
			missingCycles, err = runner.sendVolumeRewards(runner.totalVolumeRewards, startHeight, endHeight)
			if err != nil {
				return err
			}
		} else {
			step := runner.byVolumeCycleLen
			var missing uint64
			for start := startHeight; start < endHeight; start += step {
				waitCycleEnd("trade", start, start+step, runner.stable, 20*time.Second, runner.useTimeMeasurement)
				missing, err = runner.sendVolumeRewards(runner.byVolumeCycleRewards, start, start+step)
				if err != nil {
					return err
				}
				missingCycles += missing
			}
		}
		if missingCycles != 0 {
			log.Warn("found missing volume cycles", "start", startHeight, "end", endHeight, "missing", missingCycles)
		}
		log.Info("calc volume rewards success", "start", startHeight, "end", endHeight)
	}

	if calcLiquidRewards {
		waitCycleEnd("liquid", startHeight, endHeight, runner.stable, 60*time.Second, runner.useTimeMeasurement)
		runner.sampleHeights = sampleHeights
		err = runner.sendLiquidRewards(runner.byLiquidCycleRewards, startHeight, endHeight)
		if err != nil {
			return err
		}
		log.Info("calc liquid rewards success", "start", startHeight, "end", endHeight)
	}

	return nil
}

func (runner *distributeRunner) checkStartEndHeight(startHeight, endHeight uint64, calcType string) error {
	if startHeight >= endHeight {
		return fmt.Errorf("start height %v is not lower than than end height %v", startHeight, endHeight)
	}

	if startHeight < runner.start {
		return fmt.Errorf("height %v is lower than distribute start height %v", startHeight, runner.start)
	}

	calcVolumeRewards := calcType == CalcVolumeRewards || calcType == CalcBothRewards
	calcLiquidRewards := calcType == CalcLiquidRewards || calcType == CalcBothRewards

	if calcLiquidRewards || (calcVolumeRewards && !runner.quickSettleVolumeRewards) {
		cycleLen := runner.byLiquidCycleLen
		if startHeight+cycleLen != endHeight {
			return fmt.Errorf("wrong start or end height, start=%v end=%v byLiquidCycleLen=%v", startHeight, endHeight, cycleLen)
		}
	}

	if calcVolumeRewards && runner.quickSettleVolumeRewards {
		step := runner.byVolumeCycleLen
		if (endHeight-startHeight)%step != 0 {
			return fmt.Errorf("wrong start or end height, start=%v end=%v byVolumeCycleLen=%v", startHeight, endHeight, step)
		}
	}

	log.Info("check start and end height success", "startHeight", startHeight, "endHeight", endHeight, "calcType", calcType)
	return nil
}

func (runner *distributeRunner) checkSampleHeights(sampleHeights []uint64, startHeight, endHeight uint64) error {
	startH := startHeight
	endH := endHeight
	if runner.useTimeMeasurement {
		blockHeader := FindBlockByTimestamp(startHeight)
		startH = blockHeader.Number.Uint64()

		blockHeader = FindBlockByTimestamp(endHeight)
		endH = blockHeader.Number.Uint64()

		log.Info("get block height by timestamp success", "startTime", startHeight, "startHeight", startH, "endTime", endHeight, "endHeight", endH)
	}

	for _, sampleH := range sampleHeights {
		if sampleH < startH || sampleH >= endH {
			return fmt.Errorf("sample height %v not in the range of start %v to end %v", sampleH, startH, endH)
		}
	}

	log.Info("check sample height success", "sampleHeights", sampleHeights, "startHeight", startH, "endHeight", endH)
	return nil
}
