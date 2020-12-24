package distributer

import (
	"bufio"
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/mongodb"
	"github.com/anyswap/ANYToken-distribution/params"
	"github.com/anyswap/ANYToken-distribution/tools"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
)

// Option distribute options
type Option struct {
	BuildTxArgs  *BuildTxArgs
	TotalValue   *big.Int
	StartHeight  uint64 // start inclusive
	EndHeight    uint64 // end exclusive
	StableHeight uint64
	StepCount    uint64 `json:",omitempty"`
	StepReward   *big.Int
	Exchanges    []string
	Weights      []uint64
	RewardToken  string
	InputFiles   []string
	OutputFiles  []string
	SampleHeight uint64 `json:",omitempty"`
	SaveDB       bool
	DryRun       bool
	ArchiveMode  bool

	WeightIsPercentage bool

	BatchCount    uint64
	BatchInterval uint64

	// if use time measurement,
	// then StartHeight/EndHeight are unix timestamp,
	// and StableHeight/StepCount are time duration of seconds.
	// SampleHeight is always block heights, not timestamp
	UseTimeMeasurement bool

	ScalingNumerator   *big.Int
	ScalingDenominator *big.Int

	byWhat    string
	noVolumes uint64

	hasNoMissingVolumes  bool
	noVolumeStartHeights []uint64

	outputFiles []*os.File
}

// ByWhat distribute by what method
func (opt *Option) ByWhat() string {
	return opt.byWhat
}

// GetStandardByWhat get standard byWhat
func GetStandardByWhat(byWhat string) string {
	switch byWhat {
	case byLiquidMethodID, byLiquidMethodAliasID:
		return byLiquidMethodID
	case byVolumeMethodID, byVolumeMethodAliasID:
		return byVolumeMethodID
	case customMethodID:
		return customMethodID
	default:
		return ""
	}
}

// SetByWhat set byWhat
func (opt *Option) SetByWhat(byWhat string) error {
	switch byWhat {
	case byLiquidMethodID, byLiquidMethodAliasID:
		opt.byWhat = byLiquidMethodID
	case byVolumeMethodID, byVolumeMethodAliasID:
		opt.byWhat = byVolumeMethodID
	case customMethodID:
		opt.byWhat = customMethodID
	default:
		return fmt.Errorf("unknown byWhat '%v'", byWhat)
	}
	return nil
}

// GetSender get sender from keystore
func (opt *Option) GetSender() common.Address {
	return opt.BuildTxArgs.GetSender()
}

// GetChainID get chainID
func (opt *Option) GetChainID() *big.Int {
	return opt.BuildTxArgs.GetChainID()
}

func (opt *Option) String() string {
	return fmt.Sprintf("%v TotalValue %v StartHeight %v EndHeight %v StableHeight %v"+
		" StepCount %v StepReward %v SampleHeight %v Exchanges %v Weights %v"+
		" RewardToken %v DryRun %v SaveDB %v ArchiveMode %v Sender %v ChainID %v",
		opt.byWhat, opt.TotalValue, opt.StartHeight, opt.EndHeight, opt.StableHeight,
		opt.StepCount, opt.StepReward, opt.SampleHeight, opt.Exchanges, opt.Weights,
		opt.RewardToken, opt.DryRun, opt.SaveDB, opt.ArchiveMode,
		opt.GetSender().String(), opt.GetChainID(),
	)
}

func (opt *Option) deinit() {
	opt.noVolumeStartHeights = nil
	for _, file := range opt.outputFiles {
		if file != nil {
			file.Close()
			file = nil
		}
	}
}

// CheckBasic check option basic
func (opt *Option) CheckBasic() error {
	if opt.byWhat == customMethodID {
		if opt.RewardToken != "" && !common.IsHexAddress(opt.RewardToken) {
			return fmt.Errorf("[check option] wrong reward token: '%v'", opt.RewardToken)
		}
		return nil
	}
	if opt.StartHeight >= opt.EndHeight {
		return fmt.Errorf("[check option] empty range, start height %v >= end height %v", opt.StartHeight, opt.EndHeight)
	}
	if len(opt.Exchanges) == 0 {
		return fmt.Errorf("[check option] no exchanges")
	}
	for _, exchange := range opt.Exchanges {
		if exchange == "" {
			return fmt.Errorf("[check option] empty exchange")
		}
		if opt.SaveDB && !params.IsConfigedExchange(exchange) {
			return fmt.Errorf("[check option] exchange '%v' is not configed", exchange)
		}
	}
	if opt.RewardToken == "" {
		return fmt.Errorf("[check option] empty reward token")
	}
	if !common.IsHexAddress(opt.RewardToken) {
		return fmt.Errorf("[check option] wrong reward token: '%v'", opt.RewardToken)
	}
	if opt.ScalingDenominator != nil && opt.ScalingDenominator.Sign() == 0 {
		return fmt.Errorf("[check option] scaling denominator is zero (divided by zero)")
	}
	return nil
}

func (opt *Option) checkWeights() error {
	if opt.byWhat == customMethodID {
		return nil
	}
	if len(opt.Exchanges) != len(opt.Weights) {
		return fmt.Errorf("[check option] count of exchanges %v != count of weights %v", len(opt.Exchanges), len(opt.Weights))
	}
	for i, weight := range opt.Weights {
		if weight == 0 {
			return fmt.Errorf("[check option] has zero weight exchange %v", opt.Exchanges[i])
		}
	}
	if opt.WeightIsPercentage {
		sumWeight := uint64(0)
		for _, weight := range opt.Weights {
			sumWeight += weight
		}
		if sumWeight != 100 {
			return fmt.Errorf("[check option] sum of percentage weight is %v, not equal to 100", sumWeight)
		}
	}
	return nil
}

func (opt *Option) checkSteps() (err error) {
	step := opt.StepCount
	if step == 0 || opt.byWhat != byVolumeMethodID {
		return nil
	}
	length := opt.EndHeight - opt.StartHeight
	if length%step != 0 {
		return fmt.Errorf("[check option] cycle length %v is not intergral multiple of step %v", length, step)
	}
	if opt.StepReward == nil {
		return fmt.Errorf("[check option] StepReward is not specified but with StepCount %v", step)
	}
	log.Info("[check option] check step count success", "start", opt.StartHeight, "end", opt.EndHeight, "step", step, "StepReward", opt.StepReward, "totalReward", opt.TotalValue)
	return nil
}

func (opt *Option) checkAndInit() (err error) {
	log.Info("checkAndInit start")
	err = opt.CheckBasic()
	if err != nil {
		return err
	}
	err = opt.checkSteps()
	if err != nil {
		return err
	}
	err = opt.checkWeights()
	if err != nil {
		return err
	}
	err = opt.CheckSenderRewardTokenBalance()
	if err != nil {
		return err
	}
	err = opt.CheckStable()
	if err != nil {
		return err
	}
	log.Info("checkAndInit success")
	return nil
}

// CheckStable check latest block is stable to end height
func (opt *Option) CheckStable() error {
	if opt.byWhat == customMethodID {
		return nil
	}
	latest := calcLatestBlockNumberOrTimestamp(opt.UseTimeMeasurement)
	if latest >= opt.EndHeight+opt.StableHeight {
		return nil
	}
	if !opt.DryRun {
		return fmt.Errorf("[check option] latest %v is lower than end %v plus stable %v", latest, opt.EndHeight, opt.StableHeight)
	}
	if opt.byWhat == byLiquidMethodID && opt.SampleHeight == 0 && opt.ArchiveMode {
		return fmt.Errorf("[check option] latest %v is lower than end %v plus sable %v, please specify '--sample' option in dry run", latest, opt.EndHeight, opt.StableHeight)
	}
	log.Warn("[check option] block not satisfied, but ignore in dry run", "latest", latest, "end", opt.EndHeight, "stable", opt.StableHeight)
	return nil
}

func (opt *Option) getDefaultOutputFile(i int) string {
	exchange := opt.Exchanges[i]
	pairs := params.GetExchangePairs(exchange)
	timestamp := time.Now().Unix()
	return fmt.Sprintf("%s-%sReward-%d-%d-%d.csv", pairs, opt.byWhat, opt.StartHeight, opt.EndHeight, timestamp)
}

func (opt *Option) openOutputFile(i int) (err error) {
	if i >= len(opt.Exchanges) {
		return fmt.Errorf("open output file index overflow, index %v >= exchanges %v", i, len(opt.Exchanges))
	}
	if i < len(opt.outputFiles) && opt.outputFiles[i] != nil {
		return nil // already opened
	}
	if opt.outputFiles == nil {
		opt.outputFiles = make([]*os.File, len(opt.Exchanges))
	}
	fileName := ""
	if i < len(opt.OutputFiles) {
		fileName = opt.OutputFiles[i]
	}
	if fileName == "" {
		fileName = opt.getDefaultOutputFile(i)
	}
	opt.outputFiles[i], err = os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Warn("open output file error", "file", fileName, "err", err)
	} else {
		log.Info("open output file success", "file", fileName)
	}
	return err
}

func openOutputFile(fileName string) (io.Writer, error) {
	return os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
}

// WriteOutputLine write output line, will append '\n' automatically
func WriteOutputLine(ofile io.Writer, msg string) error {
	_, err := ofile.Write([]byte(msg + "\n"))
	if err != nil {
		log.Warn("[write output] error", "msg", msg, "err", err)
	} else {
		log.Printf("[write output] %v", msg)
	}
	return err
}

// WriteOutput write output
func WriteOutput(ofile io.Writer, contents ...string) error {
	msg := strings.Join(contents, ",")
	return WriteOutputLine(ofile, msg)
}

// WriteLiquiditySubject write liquidity subject
func WriteLiquiditySubject(exchange string, start, end uint64, numAccounts int) {
	msg := fmt.Sprintf("getLiquidity exchange=%v start=%v end=%v accounts=%v", exchange, start, end, numAccounts)
	log.Println(msg)
}

// WriteLiquiditySummary write liquidity summary
func WriteLiquiditySummary(exchange string, start, end uint64, numAccounts int, totalShares, totalRewards *big.Int) {
	msg := fmt.Sprintf("getLiquidity exchange=%v start=%v end=%v accounts=%v totalShares=%v totalRewards=%v", exchange, start, end, numAccounts, totalShares, totalRewards)
	log.Println(msg)
}

// WriteLiquidityBalance write liquidity balance
func WriteLiquidityBalance(account common.Address, value *big.Int, height uint64) {
	msg := fmt.Sprintf("getLiquidity %v %v height=%v", strings.ToLower(account.Hex()), value, height)
	log.Println(msg)
}

// WriteSendRewardResult write send reward result
func (opt *Option) WriteSendRewardResult(ofile io.Writer, exchange string, stat *mongodb.AccountStat, txHash *common.Hash) (err error) {
	account := stat.Account
	reward := stat.Reward
	share := stat.Share
	number := stat.Number

	accoutStr := strings.ToLower(account.String())
	rewardStr := reward.String()
	numStr := fmt.Sprintf("%d", number)

	var shareStr, hashStr string
	if share != nil {
		shareStr = share.String()
	}
	if txHash != nil {
		hashStr = txHash.Hex()
	}

	// write output beofre write database
	if txHash == nil {
		if share != nil {
			err = WriteOutput(ofile, accoutStr, rewardStr, shareStr, numStr)
		} else {
			err = WriteOutput(ofile, accoutStr, rewardStr)
		}
	} else {
		if share != nil {
			err = WriteOutput(ofile, accoutStr, rewardStr, shareStr, numStr, hashStr)
		} else {
			err = WriteOutput(ofile, accoutStr, rewardStr, hashStr)
		}
	}

	opt.WriteRewardResultToDB(exchange, accoutStr, rewardStr, shareStr, number, hashStr)

	return err
}

// WriteRewardResultToDB write reward result to database
func (opt *Option) WriteRewardResultToDB(exchange, accoutStr, rewardStr, shareStr string, number uint64, hashStr string) {
	if !opt.SaveDB || opt.byWhat == customMethodID {
		return
	}
	exchange = strings.ToLower(exchange)
	pairs := params.GetExchangePairs(exchange)
	switch opt.byWhat {
	case byVolumeMethodID:
		mr := &mongodb.MgoVolumeRewardResult{
			Key:         mongodb.GetKeyOfRewardResult(exchange, accoutStr, opt.StartHeight),
			Exchange:    exchange,
			Pairs:       pairs,
			Start:       opt.StartHeight,
			End:         opt.EndHeight,
			RewardToken: opt.RewardToken,
			Account:     accoutStr,
			Reward:      rewardStr,
			Volume:      shareStr,
			TxCount:     number,
			RewardTx:    hashStr,
			Timestamp:   uint64(time.Now().Unix()),
		}
		_ = mongodb.TryDoTimes("AddVolumeRewardResult "+mr.Key, func() error {
			return mongodb.AddVolumeRewardResult(mr)
		})
	case byLiquidMethodID:
		mr := &mongodb.MgoLiquidRewardResult{
			Key:         mongodb.GetKeyOfRewardResult(exchange, accoutStr, opt.StartHeight),
			Exchange:    exchange,
			Pairs:       pairs,
			Start:       opt.StartHeight,
			End:         opt.EndHeight,
			RewardToken: opt.RewardToken,
			Account:     accoutStr,
			Reward:      rewardStr,
			Liquidity:   shareStr,
			Height:      number,
			RewardTx:    hashStr,
			Timestamp:   uint64(time.Now().Unix()),
		}
		_ = mongodb.TryDoTimes("AddLiquidRewardResult "+mr.Key, func() error {
			return mongodb.AddLiquidRewardResult(mr)
		})
	case customMethodID:
	default:
		log.Warn("unknown byWhat in option", "byWhat", opt.byWhat)
	}
}

// WriteNoVolumeOutput write output
func WriteNoVolumeOutput(exchange string, start, end uint64) {
	msg := fmt.Sprintf("calcRewards exchange=%s start=%d end=%d novolume", exchange, start, end)
	log.Println(msg)
}

// WriteNoVolumeSummary write no volume summary
func (opt *Option) WriteNoVolumeSummary() {
	msg := fmt.Sprintf("calcRewards start=%d end=%d novolumes=%v", opt.StartHeight, opt.EndHeight, opt.noVolumes)
	log.Println(msg)
}

// SendRewardsTransaction send rewards
func (opt *Option) SendRewardsTransaction(account common.Address, reward *big.Int) (txHash *common.Hash, err error) {
	rewardToken := common.HexToAddress(opt.RewardToken)
	return opt.BuildTxArgs.sendRewardsTransaction(account, reward, rewardToken, opt.DryRun)
}

// CheckSenderRewardTokenBalance check token balance
func (opt *Option) CheckSenderRewardTokenBalance() (err error) {
	if opt.RewardToken == "" {
		return fmt.Errorf("[check option] check token balance with empty reward token")
	}
	sender := opt.BuildTxArgs.fromAddr
	rewardTokenAddr := common.HexToAddress(opt.RewardToken)
	var senderTokenBalance *big.Int
	for {
		senderTokenBalance, err = capi.GetTokenBalance(rewardTokenAddr, sender, nil)
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		if senderTokenBalance.Cmp(opt.TotalValue) < 0 {
			err = fmt.Errorf("[check option] not enough reward token balance, %v < %v, sender: %v token: %v", senderTokenBalance, opt.TotalValue, sender.String(), opt.RewardToken)
			if opt.DryRun {
				log.Warn("[check option] check sender reward token balance failed, but ignore in dry run", "err", err)
				return nil // only warn not enough balance in dry run
			}
			return err
		}
		break
	}
	log.Info("sender reward token balance is enough", "sender", sender.String(), "token", rewardTokenAddr.String(), "balance", senderTokenBalance, "needed", opt.TotalValue)

	senderBalance, err := capi.GetCoinBalance(sender, nil)
	if err != nil {
		log.Warn("get sender coin balance failed", "err", err)
	} else {
		log.Info("get sender coin balance success, please ensure it's enough for gas fee", "balance", senderBalance)
	}
	return nil
}

// CheckSenderCoinBalance check coin balance
func (opt *Option) CheckSenderCoinBalance() (err error) {
	if opt.RewardToken != "" {
		return fmt.Errorf("[check option] check coin balance with nonempty reward token %v", opt.RewardToken)
	}
	sender := opt.BuildTxArgs.fromAddr
	var senderBalance *big.Int
	for {
		senderBalance, err = capi.GetCoinBalance(sender, nil)
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		if senderBalance.Cmp(opt.TotalValue) < 0 {
			err = fmt.Errorf("[check option] not enough coin balance, %v < %v, sender: %v", senderBalance, opt.TotalValue, sender.String())
			if opt.DryRun {
				log.Warn("[check option] check sender coin balance failed, but ignore in dry run", "err", err)
				return nil // only warn not enough balance in dry run
			}
			return err
		}
		break
	}
	log.Info("sender coin balance is enough", "sender", sender.String(), "balance", senderBalance, "needed", opt.TotalValue)
	return nil
}

func (opt *Option) getAccounts() (accounts [][]common.Address, err error) {
	accounts = make([][]common.Address, len(opt.Exchanges))
	var accs []common.Address
	for i, exchange := range opt.Exchanges {
		ifile := opt.getInputFileName(i)
		if ifile == "" {
			accs = getAccountsFromDB(exchange)
		} else {
			accs, err = getAccountsFromFile(ifile)
			if err != nil {
				return nil, err
			}
		}
		accounts[i] = accs
	}
	return accounts, nil
}

func getAccountsFromDB(exchange string) []common.Address {
	return mongodb.FindAllAccounts(exchange)
}

func getAccountsFromFile(ifile string) (accounts []common.Address, err error) {
	file, err := os.Open(ifile)
	if err != nil {
		return nil, fmt.Errorf("open %v failed. %v)", ifile, err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	for {
		lineData, _, errf := reader.ReadLine()
		if errf == io.EOF {
			break
		}
		line := strings.TrimSpace(string(lineData))
		if isCommentedLine(line) {
			continue
		}
		if !common.IsHexAddress(line) {
			return nil, fmt.Errorf("found wrong address line %v", line)
		}
		account := common.HexToAddress(line)
		if params.IsExcludedRewardAccount(account) {
			log.Warn("ignore excluded account", "account", line)
			continue
		}
		if IsAccountExist(account, accounts) {
			log.Warn("ignore duplicate account", "account", line)
			continue
		}
		accounts = append(accounts, account)
	}

	return accounts, nil
}

func (opt *Option) getInputFileName(i int) string {
	if i < len(opt.InputFiles) {
		return opt.InputFiles[i]
	}
	return ""
}

func (opt *Option) getOutputFile(i int) (io.Writer, error) {
	err := opt.openOutputFile(i)
	return opt.outputFiles[i], err
}

// GetAccountsAndRewards get from file if input file exist, or else from database
func (opt *Option) GetAccountsAndRewards() (accountStats []mongodb.AccountStatSlice, err error) {
	accountStats = make([]mongodb.AccountStatSlice, len(opt.Exchanges))
	var stats mongodb.AccountStatSlice
	for i, exchange := range opt.Exchanges {
		ifile := opt.getInputFileName(i)
		if ifile == "" {
			stats = opt.GetAccountsAndRewardsFromDB(exchange)
		} else {
			stats, _, err = GetAccountsAndRewardsFromFile(ifile)
			if err != nil {
				return nil, err
			}
		}
		accountStats[i] = stats
	}
	opt.noVolumes = uint64(len(opt.noVolumeStartHeights))
	return accountStats, nil
}

// no volume is intersection set of all exchanges
func (opt *Option) updateNoVolumes(noVolumeStarts []uint64) {
	if opt.hasNoMissingVolumes {
		return
	}
	if len(noVolumeStarts) == 0 {
		opt.hasNoMissingVolumes = true
		opt.noVolumeStartHeights = nil
		return
	}
	if len(opt.noVolumeStartHeights) == 0 {
		opt.noVolumeStartHeights = noVolumeStarts
		return
	}
	var intersection []uint64
	for _, oldH := range opt.noVolumeStartHeights {
		for _, newH := range noVolumeStarts {
			if oldH == newH {
				intersection = append(intersection, oldH)
			}
		}
	}
	opt.noVolumeStartHeights = intersection
	if len(opt.noVolumeStartHeights) == 0 {
		opt.hasNoMissingVolumes = true
	}
}

// GetAccountsAndRewardsFromDB get from database
func (opt *Option) GetAccountsAndRewardsFromDB(exchange string) (accountStats mongodb.AccountStatSlice) {
	step := opt.StepCount
	if step == 0 {
		return getSingleCycleRewardsFromDB(opt.TotalValue, exchange, opt.StartHeight, opt.EndHeight, opt.UseTimeMeasurement, opt.ArchiveMode)
	}

	// use map to statistic
	finStatMap := make(map[common.Address]*mongodb.AccountStat)

	steps := (opt.EndHeight - opt.StartHeight) / step
	stepRewards := new(big.Int).Div(opt.TotalValue, new(big.Int).SetUint64(steps))

	var noVolumeStarts []uint64
	for start := opt.StartHeight; start < opt.EndHeight; start += step {
		cycleStats := getSingleCycleRewardsFromDB(stepRewards, exchange, start, start+step, opt.UseTimeMeasurement, opt.ArchiveMode)
		if len(cycleStats) == 0 {
			WriteNoVolumeOutput(exchange, start, start+step)
			noVolumeStarts = append(noVolumeStarts, start)
			continue
		}
		for _, stat := range cycleStats {
			reward := stat.Reward
			if reward == nil || reward.Sign() <= 0 {
				log.Error("non positive reward exist, please check")
				continue
			}
			finStat, exist := finStatMap[stat.Account]
			if exist {
				finStat.Reward.Add(finStat.Reward, reward)
				finStat.Share.Add(finStat.Share, stat.Share)
				finStat.Number += stat.Number
			} else {
				finStatMap[stat.Account] = stat
			}
		}
	}
	opt.updateNoVolumes(noVolumeStarts)
	accountStats = mongodb.ConvertToSortedSlice(finStatMap)
	log.Info("get account volumes from db success", "exchange", exchange, "start", opt.StartHeight, "end", opt.EndHeight, "step", step, "missSteps", opt.noVolumes)
	opt.WriteNoVolumeSummary()
	return accountStats
}

func getSingleCycleRewardsFromDB(totalRewards *big.Int, exchange string, startHeight, endHeight uint64, useTimestamp, archiveMode bool) mongodb.AccountStatSlice {
	accountStats := mongodb.FindAccountVolumes(exchange, startHeight, endHeight, useTimestamp)
	if len(accountStats) == 0 {
		return nil
	}
	if params.GetConfig().Stake != nil {
		var blockNumber *big.Int
		if archiveMode {
			blockNumber = new(big.Int).SetUint64(endHeight)
		}
		weightedByStakeAmount(accountStats, blockNumber)
	}
	accountStats.CalcRewards(totalRewards)

	subject := fmt.Sprintf("calcRewards exchange=%v start=%v end=%v rewards=%v accounts=%v", exchange, startHeight, endHeight, totalRewards, len(accountStats))
	log.Println(subject)
	for _, stat := range accountStats {
		line := fmt.Sprintf("calcRewards %v %v start=%v end=%v share=%v", strings.ToLower(stat.Account.String()), stat.Reward, startHeight, endHeight, stat.Share)
		log.Println(line)
	}

	return accountStats
}

func weightedByStakeAmount(accountStats mongodb.AccountStatSlice, blockNumber *big.Int) {
	stakeCfg := params.GetConfig().Stake
	stakeContract := common.HexToAddress(stakeCfg.Contract)
	for _, stat := range accountStats {
		if !params.IsInStakerList(stat.Account) {
			continue
		}
		stakeAmount := capi.LoopGetStakeAmount(stakeContract, stat.Account, blockNumber)
		stakeWholeAmount := stakeAmount.Div(stakeAmount, big.NewInt(1e18)).Uint64()
		addPercent := calcAddPercentOfStaking(stakeWholeAmount)
		if addPercent == 0 {
			continue
		}
		added := new(big.Int).Mul(stat.Share, new(big.Int).SetUint64(addPercent))
		added.Div(added, big.NewInt(100))
		log.Trace("add weighted volume share by stake", "account", stat.Account.String(),
			"origin", stat.Share, "added", added, "addPercent", addPercent,
			"stakeAmount", stakeAmount, "stakeWholeAmount", stakeWholeAmount,
			"blockNumber", blockNumber)
		stat.Share.Add(stat.Share, added)
	}
}

func calcAddPercentOfStaking(stakeWholeAmount uint64) (percent uint64) {
	stakeCfg := params.GetConfig().Stake
	for _, point := range stakeCfg.Points {
		if stakeWholeAmount >= point {
			percent = point
		}
	}
	return percent
}

// GetAccountsAndRewardsFromFile pass line format "<address> <amount>" from input file
func GetAccountsAndRewardsFromFile(ifile string) (accountStats mongodb.AccountStatSlice, titleLine string, err error) {
	file, err := os.Open(ifile)
	if err != nil {
		return nil, "", fmt.Errorf("open %v failed. %v)", ifile, err)
	}
	defer file.Close()

	accountStats = make(mongodb.AccountStatSlice, 0)

	reader := bufio.NewReader(file)
	isFirstLine := true

	for {
		lineData, _, errf := reader.ReadLine()
		if errf == io.EOF {
			break
		}
		line := strings.TrimSpace(string(lineData))
		if isCommentedLine(line) {
			if isFirstLine {
				titleLine = line
			}
			isFirstLine = false
			continue
		}
		isFirstLine = false
		parts := blankOrCommaSepRegexp.Split(line, -1)
		if len(parts) < 2 {
			return nil, "", fmt.Errorf("less than 2 parts in line %v", line)
		}
		accountStr := parts[0]
		rewardStr := parts[1]
		if !common.IsHexAddress(accountStr) {
			return nil, "", fmt.Errorf("wrong address in line %v", line)
		}
		account := common.HexToAddress(accountStr)
		if params.IsExcludedRewardAccount(account) {
			log.Warn("ignore excluded account", "account", accountStr)
			continue
		}
		if accountStats.IsAccountExist(account) {
			log.Info("found duplicate account", "account", accountStr)
		}
		reward, err := tools.GetBigIntFromString(rewardStr)
		if err != nil {
			return nil, "", fmt.Errorf("wrong reward in line %v, err=%v", line, err)
		}
		if reward.Sign() <= 0 {
			continue
		}
		stat := &mongodb.AccountStat{
			Account: account,
			Reward:  reward,
		}
		if len(parts) >= 4 {
			shareStr := parts[2]
			numberStr := parts[3]
			share, err := tools.GetBigIntFromString(shareStr)
			if err != nil {
				return nil, "", fmt.Errorf("wrong share in line %v, err=%v", line, err)
			}
			number, err := tools.GetBigIntFromString(numberStr)
			if err != nil {
				return nil, "", fmt.Errorf("wrong number in line %v, err=%v", line, err)
			}
			stat.Share = share
			stat.Number = number.Uint64()
		}
		accountStats = append(accountStats, stat)
	}

	return accountStats, titleLine, nil
}

// GetAccountsAndShares get accounts and shares
func (opt *Option) GetAccountsAndShares() (accountStats []mongodb.AccountStatSlice, err error) {
	if len(opt.InputFiles) == 0 {
		return nil, nil
	}
	if len(opt.InputFiles) != len(opt.Exchanges) {
		return nil, fmt.Errorf("count of input files %v and exchanges %v are not equal", len(opt.InputFiles), len(opt.Exchanges))
	}
	accountStats = make([]mongodb.AccountStatSlice, len(opt.Exchanges))
	var stats mongodb.AccountStatSlice
	for i, inputFile := range opt.InputFiles {
		stats, err = GetAccountsAndSharesFromFile(inputFile, opt.SampleHeight)
		if err != nil {
			return nil, err
		}
		accountStats[i] = stats
	}
	return accountStats, nil
}

// GetAccountsAndSharesFromFile get accounts and shares from file
func GetAccountsAndSharesFromFile(ifile string, sampleHeight uint64) (accountStats mongodb.AccountStatSlice, err error) {
	file, err := os.Open(ifile)
	if err != nil {
		return nil, fmt.Errorf("open %v failed. %v)", ifile, err)
	}
	defer file.Close()

	accountStats = make(mongodb.AccountStatSlice, 0)

	reader := bufio.NewReader(file)

	for {
		lineData, _, errf := reader.ReadLine()
		if errf == io.EOF {
			break
		}
		line := strings.TrimSpace(string(lineData))
		if isCommentedLine(line) {
			continue
		}
		parts := blankOrCommaSepRegexp.Split(line, -1)
		if len(parts) < 2 {
			return nil, fmt.Errorf("less than 2 parts in line %v", line)
		}
		accountStr := parts[0]
		shareStr := parts[1]
		if !common.IsHexAddress(accountStr) {
			return nil, fmt.Errorf("wrong address in line %v", line)
		}
		account := common.HexToAddress(accountStr)
		if params.IsExcludedRewardAccount(account) {
			log.Warn("ignore excluded account", "account", accountStr)
			continue
		}
		if accountStats.IsAccountExist(account) {
			log.Info("found duplicate account", "account", accountStr)
		}
		share, err := tools.GetBigIntFromString(shareStr)
		if err != nil {
			return nil, fmt.Errorf("wrong share in line %v, err=%v", line, err)
		}
		if share.Sign() <= 0 {
			continue
		}
		stat := &mongodb.AccountStat{
			Account: account,
			Share:   share,
			Number:  sampleHeight,
		}
		accountStats = append(accountStats, stat)
	}

	return accountStats, nil
}

func isCommentedLine(line string) bool {
	return strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//")
}
