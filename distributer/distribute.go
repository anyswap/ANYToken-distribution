package distributer

import (
	"errors"
	"math/big"
	"time"

	"github.com/anyswap/ANYToken-distribution/callapi"
	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/mongodb"
	"github.com/anyswap/ANYToken-distribution/params"
	"github.com/anyswap/ANYToken-distribution/syncer"
	"github.com/anyswap/ANYToken-distribution/tools"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
)

var capi *callapi.APICaller

const (
	byLiquidMethodID      = "liquidity"
	byLiquidMethodAliasID = "liquid"
	byVolumeMethodID      = "volume"
	byVolumeMethodAliasID = "trade"
)

var (
	errTotalRewardsIsZero       = errors.New("total rewards is zero")
	errCheckOptionFailed        = errors.New("check option failed")
	errGetAccountListFailed     = errors.New("get account list failed")
	errGetAccountsRewardsFailed = errors.New("get accounts rewards failed")
	errAccountsNotComplete      = errors.New("account list is not complete")
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
	distCfg := config.Distribute
	if !distCfg.Enable {
		log.Warn("[distribute] stop distribute as it's disabled")
		return
	}
	go startDistributeJob(distCfg)
}

// every 6600 blocks distribute:
// 	1. by liquidity rewards
// 	2. by volume rewards
// give configed node rewards to liquidity rewards
// check volumes every 100 block,
// if no volume then give configed some of this rewards to liquidity rewards
func startDistributeJob(distCfg *params.DistributeConfig) {
	log.Info("[distribute] start job", "config", distCfg)

	runner, err := initDistributer(distCfg)
	if err != nil {
		log.Error("[distribute] start failed", "err", err)
		return
	}

	runner.run()
}

type distributeRunner struct {
	exchanges     []string
	liquidWeights []uint64

	rewardToken string
	start       uint64
	stable      uint64
	dryRun      bool
	saveDB      bool

	byLiquidCycleLen     uint64
	byLiquidCycleRewards *big.Int

	addedNodeRewards            *big.Int
	addedNoVolumeRewardsPerStep *big.Int

	byVolumeCycleLen     uint64
	byVolumeCycleRewards *big.Int
	totalVolumeCycles    uint64
	totalVolumeRewards   *big.Int

	byLiquidArgs *BuildTxArgs
	byVolumeArgs *BuildTxArgs
}

func initDistributer(distCfg *params.DistributeConfig) (*distributeRunner, error) {
	var err error
	runner := &distributeRunner{}

	runner.byLiquidArgs, err = getBuildTxArgs(byLiquidMethodID, distCfg)
	if err != nil {
		return nil, err
	}

	runner.byVolumeArgs, err = getBuildTxArgs(byVolumeMethodID, distCfg)
	if err != nil {
		return nil, err
	}

	runner.dryRun = distCfg.DryRun
	runner.saveDB = distCfg.SaveDB

	runner.rewardToken = distCfg.RewardToken
	runner.start = distCfg.StartHeight
	runner.stable = distCfg.StableHeight

	runner.byLiquidCycleLen = distCfg.ByLiquidCycle
	runner.byLiquidCycleRewards = distCfg.GetByLiquidCycleRewards()

	runner.addedNodeRewards = distCfg.GetAddNodeRewards()
	runner.byLiquidCycleRewards.Add(runner.byLiquidCycleRewards, runner.addedNodeRewards)

	runner.addedNoVolumeRewardsPerStep = distCfg.GetAddNoVolumeRewards()

	runner.byVolumeCycleLen = distCfg.ByVolumeCycle
	runner.byVolumeCycleRewards = distCfg.GetByVolumeCycleRewards()
	if runner.byLiquidCycleLen%runner.byVolumeCycleLen != 0 {
		log.Fatal("liquid cycle %v is not multiple intergral of volume cycle %v", runner.byLiquidCycleLen, runner.byVolumeCycleLen)
	}
	runner.totalVolumeCycles = runner.byLiquidCycleLen / runner.byVolumeCycleLen
	runner.totalVolumeRewards = new(big.Int).Mul(runner.byVolumeCycleRewards, new(big.Int).SetUint64(runner.totalVolumeCycles))

	for _, exchange := range params.GetConfig().Exchanges {
		if exchange.LiquidWeight > 0 {
			runner.exchanges = append(runner.exchanges, exchange.Exchange)
			runner.liquidWeights = append(runner.liquidWeights, exchange.LiquidWeight)
		}
	}

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
	curCycleStart := calcCurCycleStart(runner.start, runner.stable, runner.byLiquidCycleLen)
	for {
		curCycleEnd := curCycleStart + runner.byLiquidCycleLen
		waitCycleEnd(curCycleStart, curCycleEnd, runner.stable)

		missVolumeCycles, err := runner.sendVolumeRewards(curCycleStart, curCycleEnd)
		if err == nil {
			err = runner.sendMissingVolumeRewards(curCycleStart, curCycleEnd, missVolumeCycles)
			if err == nil {
				_ = runner.sendLiquidRewards(curCycleStart, curCycleEnd)
			}
		}

		// start next cycle
		curCycleStart = curCycleEnd
		log.Info("start next cycle", "start", curCycleStart)
	}
}

func (runner *distributeRunner) sendVolumeRewards(start, end uint64) (missVolumeCycles uint64, err error) {
	opt := &Option{
		BuildTxArgs:  runner.byVolumeArgs,
		TotalValue:   runner.totalVolumeRewards,
		StartHeight:  start,
		EndHeight:    end,
		StableHeight: runner.stable,
		StepCount:    runner.byVolumeCycleLen,
		StepReward:   runner.byVolumeCycleRewards,
		Exchanges:    runner.exchanges,
		RewardToken:  runner.rewardToken,
		SaveDB:       runner.saveDB,
		DryRun:       runner.dryRun,
	}
	log.Info("start send volume reward", "option", opt.String())
	err = ByVolume(opt)
	if err != nil {
		log.Error("send volume reward failed", "err", err)
		return 0, err
	}
	log.Info("send volume reward success")
	return opt.noVolumes, err
}

func (runner *distributeRunner) sendMissingVolumeRewards(start, end, missVolumeCycles uint64) error {
	if missVolumeCycles == 0 || runner.addedNoVolumeRewardsPerStep.Sign() <= 0 {
		return nil
	}
	addedNoVolumeRewards := new(big.Int).Mul(runner.addedNoVolumeRewardsPerStep, new(big.Int).SetUint64(missVolumeCycles))
	opt := &Option{
		BuildTxArgs:  runner.byVolumeArgs,
		TotalValue:   addedNoVolumeRewards,
		StartHeight:  start,
		EndHeight:    end,
		StableHeight: runner.stable,
		RewardToken:  runner.rewardToken,
		SaveDB:       runner.saveDB,
		DryRun:       runner.dryRun,
	}
	receiver := runner.byLiquidArgs.GetSender()
	log.Info("start send missing volume rewards", "to", receiver.String(), "value", addedNoVolumeRewards, "start", opt.StartHeight, "end", opt.EndHeight, "missVolumeCycles", missVolumeCycles)
	err := sendMissingVolumeRewards(opt, receiver)
	if err != nil {
		log.Error("send missing volume rewards failed", "err", err)
		return err
	}
	log.Info("send missing volume rewards success")

	runner.byLiquidCycleRewards.Add(runner.byLiquidCycleRewards, addedNoVolumeRewards)
	return nil
}

func (runner *distributeRunner) sendLiquidRewards(start, end uint64) error {
	opt := &Option{
		BuildTxArgs:   runner.byLiquidArgs,
		TotalValue:    runner.byLiquidCycleRewards,
		StartHeight:   start,
		EndHeight:     end,
		StableHeight:  runner.stable,
		Exchanges:     runner.exchanges,
		LiquidWeights: runner.liquidWeights,
		RewardToken:   runner.rewardToken,
		SaveDB:        runner.saveDB,
		DryRun:        runner.dryRun,
	}
	log.Info("start send liquid reward", "option", opt.String())
	err := ByLiquidity(opt)
	if err != nil {
		log.Error("send liquid reward failed", "err", err)
		return err
	}
	log.Info("send liquid reward success")
	return nil
}

func sendMissingVolumeRewards(opt *Option, to common.Address) error {
	from := opt.GetSender()
	value := opt.TotalValue
	txHash, err := opt.SendRewardsTransaction(to, value)
	if err != nil {
		log.Warn("send missing volume rewards failed", "from", from.String(), "to", to.String(), "value", value, "start", opt.StartHeight, "end", opt.EndHeight, "err", err)
		return err
	}
	var txHashStr string
	if txHash != nil {
		txHashStr = txHash.String()
	}
	log.Info("send missing volume rewards success", "from", from.String(), "to", to.String(), "value", value, "start", opt.StartHeight, "end", opt.EndHeight, "txHash", txHashStr, "dryrun", opt.DryRun)
	return nil
}

func waitCycleEnd(cycleStart, cycleEnd, stable uint64) {
	latest := uint64(0)
	waitInterval := 60 * time.Second
	for {
		time.Sleep(waitInterval)
		syncInfo, err := mongodb.FindLatestSyncInfo()
		if err != nil {
			log.Warn("find latest sync info failed", "err", err)
			continue
		}
		latest = syncInfo.Number

		if latest >= cycleEnd+stable {
			break
		}
		log.Info("wait to cycle end", "cycleStart", cycleStart, "cycleEnd", cycleEnd, "stable", stable, "latest", latest)
	}
	log.Info("cycle end is achieved", "cycleStart", cycleStart, "cycleEnd", cycleEnd, "stable", stable, "latest", latest)
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
	case byLiquidMethodID:
		keystoreFile = distCfg.ByLiquidKeystoreFile
		passwordFile = distCfg.ByLiquidPasswordFile
	case byVolumeMethodID:
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
