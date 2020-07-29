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

const sampleCount = 4

// Option distribute options
type Option struct {
	BuildTxArgs  *BuildTxArgs
	TotalValue   *big.Int
	StartHeight  uint64 // start inclusive
	EndHeight    uint64 // end exclusive
	StableHeight uint64
	StepCount    uint64 `json:",omitempty"`
	StepReward   *big.Int
	Exchange     string
	RewardToken  string
	InputFile    string
	OutputFile   string
	Heights      []uint64 `json:",omitempty"`
	SaveDB       bool
	DryRun       bool

	byWhat     string
	outputFile *os.File
	noVolumes  uint64
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
		" StepCount %v StepReward %v Heights %v Exchange %v RewardToken %v DryRun %v SaveDB %v Sender %v ChainID %v",
		opt.byWhat, opt.TotalValue, opt.StartHeight, opt.EndHeight, opt.StableHeight,
		opt.StepCount, opt.StepReward, opt.Heights, opt.Exchange, opt.RewardToken, opt.DryRun, opt.SaveDB,
		opt.GetSender().String(), opt.GetChainID(),
	)
}

func (opt *Option) deinit() {
	if opt.outputFile != nil {
		opt.outputFile.Close()
		opt.outputFile = nil
	}
}

// CheckBasic check option basic
func (opt *Option) CheckBasic() error {
	if opt.StartHeight >= opt.EndHeight {
		return fmt.Errorf("[check option] empty range, start height %v >= end height %v", opt.StartHeight, opt.EndHeight)
	}
	if opt.Exchange == "" {
		return fmt.Errorf("[check option] must specify exchange")
	}
	if (!opt.DryRun || opt.SaveDB) && !params.IsConfigedExchange(opt.Exchange) {
		return fmt.Errorf("[check option] exchange '%v' is not configed", opt.Exchange)
	}
	if opt.RewardToken == "" {
		return fmt.Errorf("[check option] must specify reward token")
	}
	if !common.IsHexAddress(opt.RewardToken) {
		return fmt.Errorf("[check option] wrong reward token: '%v'", opt.RewardToken)
	}
	return nil
}

func (opt *Option) checkAndInit() (err error) {
	if opt.StepCount != 0 && opt.byWhat == byVolumeMethodID {
		length := opt.EndHeight - opt.StartHeight
		if length%opt.StepCount != 0 {
			return fmt.Errorf("[check option] cycle length %v is not intergral multiple of step %v", length, opt.StepCount)
		}
		steps := length / opt.StepCount
		if new(big.Int).Mod(opt.TotalValue, new(big.Int).SetUint64(steps)).Sign() != 0 {
			return fmt.Errorf("[check option] total value %v is not intergral multiple of steps %v", opt.TotalValue, steps)
		}
		if opt.StepReward == nil {
			return fmt.Errorf("[check option] StepReward is not specified but with StepCount %v", opt.StepCount)
		}
		log.Info("[check option] check step count success", "start", opt.StartHeight, "end", opt.EndHeight, "step", opt.StepCount, "StepReward", opt.StepReward)
	}

	err = opt.CheckSenderRewardTokenBalance()
	if err != nil {
		return err
	}
	err = opt.CheckStable()
	if err != nil {
		return err
	}
	return nil
}

// CheckStable check latest block is stable to end height
func (opt *Option) CheckStable() error {
	latestBlock := capi.LoopGetLatestBlockHeader()
	if latestBlock.Number.Uint64() >= opt.EndHeight+opt.StableHeight {
		return nil
	}
	if !opt.DryRun {
		return fmt.Errorf("[check option] latest height %v is lower than end height %v plus stable height %v", latestBlock.Number, opt.EndHeight, opt.StableHeight)
	}
	if opt.byWhat == byLiquidMethodID && len(opt.Heights) == 0 {
		return fmt.Errorf("[check option] latest height %v is lower than end height %v plus sable height %v, please specify '--heights' option in dry run", latestBlock.Number, opt.EndHeight, opt.StableHeight)
	}
	log.Warn("[check option] block height not satisfied, but ignore in dry run", "latest", latestBlock.Number, "end", opt.EndHeight, "stable", opt.StableHeight)
	return nil
}

func (opt *Option) getDefaultOutputFile() string {
	pairs := params.GetExchangePairs(opt.Exchange)
	timestamp := time.Now().Unix()
	return fmt.Sprintf("%s-%sReward-%d-%d-%d.csv", pairs, opt.byWhat, opt.StartHeight, opt.EndHeight, timestamp)
}

func (opt *Option) openOutputFile() (err error) {
	if opt.outputFile != nil {
		return nil // already opened
	}
	fileName := opt.OutputFile
	if fileName == "" {
		fileName = opt.getDefaultOutputFile()
	}
	opt.outputFile, err = os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Warn("open output file error", "file", fileName, "err", err)
	} else {
		log.Info("open output file success", "file", fileName)
	}
	return err
}

// WriteOutputLine write output line, will append '\n' automatically
func (opt *Option) WriteOutputLine(msg string) error {
	if opt.outputFile == nil {
		err := opt.openOutputFile()
		if err != nil {
			return err
		}
	}
	_, err := opt.outputFile.Write([]byte(msg + "\n"))
	if err != nil {
		log.Warn("[write output] error", "msg", msg, "err", err)
	} else {
		log.Printf("[write output] %v", msg)
	}
	return err
}

// WriteOutput write output
func (opt *Option) WriteOutput(contents ...string) error {
	msg := strings.Join(contents, ",")
	return opt.WriteOutputLine(msg)
}

// WriteLiquiditySubject write liquidity subject
func (opt *Option) WriteLiquiditySubject(exchange string, start, end uint64, numAccounts int) error {
	msg := fmt.Sprintf("getLiquidity exchange=%v start=%v end=%v accounts=%v", exchange, start, end, numAccounts)
	// only log final result //return opt.WriteOutputLine(msg)
	log.Println(msg)
	return nil
}

// WriteLiquiditySummary write liquidity summary
func (opt *Option) WriteLiquiditySummary(exchange string, start, end uint64, numAccounts int, totalShares, totalRewards *big.Int) error {
	msg := fmt.Sprintf("getLiquidity exchange=%v start=%v end=%v accounts=%v totalShares=%v totalRewards=%v", exchange, start, end, numAccounts, totalShares, totalRewards)
	// only log final result //return opt.WriteOutputLine(msg)
	log.Println(msg)
	return nil
}

// WriteLiquidityBalance write liquidity balance
func (opt *Option) WriteLiquidityBalance(account common.Address, value *big.Int, height uint64) error {
	msg := fmt.Sprintf("getLiquidity %v %v height=%v", strings.ToLower(account.Hex()), value, height)
	// only log final result //return opt.WriteOutputLine(msg)
	log.Println(msg)
	return nil
}

// WriteSendRewardFromFileResult write send reward result
func (opt *Option) WriteSendRewardFromFileResult(account common.Address, reward *big.Int, txHash *common.Hash) error {
	accoutStr := strings.ToLower(account.Hex())
	rewardStr := reward.String()
	if txHash == nil {
		return opt.WriteOutput(accoutStr, rewardStr)
	}
	return opt.WriteOutput(accoutStr, rewardStr, txHash.Hex())
}

// WriteSendRewardResult write send reward result
func (opt *Option) WriteSendRewardResult(stat *mongodb.AccountStat, txHash *common.Hash) (err error) {
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
			err = opt.WriteOutput(accoutStr, rewardStr, shareStr, numStr)
		} else {
			err = opt.WriteOutput(accoutStr, rewardStr)
		}
	} else {
		if share != nil {
			err = opt.WriteOutput(accoutStr, rewardStr, shareStr, numStr, hashStr)
		} else {
			err = opt.WriteOutput(accoutStr, rewardStr, hashStr)
		}
	}

	if !opt.DryRun || opt.SaveDB {
		opt.writeRewardResultToDB(accoutStr, rewardStr, shareStr, number, hashStr)
	}

	return err
}

func (opt *Option) writeRewardResultToDB(accoutStr, rewardStr, shareStr string, number uint64, hashStr string) {
	exchange := strings.ToLower(opt.Exchange)
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
	default:
		log.Warn("unknown byWhat in option", "byWhat", opt.byWhat)
	}
}

// WriteNoVolumeOutput write output
func (opt *Option) WriteNoVolumeOutput(exchange string, start, end uint64) error {
	msg := fmt.Sprintf("calcRewards exchange=%s start=%d end=%d novolume", exchange, start, end)
	// only log final result //return opt.WriteOutputLine(msg)
	log.Println(msg)
	return nil
}

// WriteNoVolumeSummary write no volume summary
func (opt *Option) WriteNoVolumeSummary() error {
	msg := fmt.Sprintf("calcRewards exchange=%s start=%d end=%d novolumes=%v", opt.Exchange, opt.StartHeight, opt.EndHeight, opt.noVolumes)
	// only log final result //return opt.WriteOutputLine(msg)
	log.Println(msg)
	return nil
}

// SendRewardsTransaction send rewards
func (opt *Option) SendRewardsTransaction(account common.Address, reward *big.Int) (txHash *common.Hash, err error) {
	rewardToken := common.HexToAddress(opt.RewardToken)
	return opt.BuildTxArgs.sendRewardsTransaction(account, reward, rewardToken, opt.DryRun)
}

// CheckSenderRewardTokenBalance check balance
func (opt *Option) CheckSenderRewardTokenBalance() (err error) {
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
	return nil
}

func (opt *Option) getAccounts() (accounts []common.Address, err error) {
	if opt.InputFile == "" {
		accounts = mongodb.FindAllAccounts(opt.Exchange)
		return accounts, nil
	}

	file, err := os.Open(opt.InputFile)
	if err != nil {
		return nil, fmt.Errorf("open %v failed. %v)", opt.InputFile, err)
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
		if !IsAccountExist(account, accounts) {
			accounts = append(accounts, account)
		} else {
			log.Warn("ignore duplicate account %v", account.String())
		}
	}

	return accounts, nil
}

// GetAccountsAndRewards get from file if input file exist, or else from database
func (opt *Option) GetAccountsAndRewards() (accountStats mongodb.AccountStatSlice, err error) {
	if opt.InputFile == "" {
		accountStats = opt.GetAccountsAndRewardsFromDB()
	} else {
		accountStats, _, err = opt.GetAccountsAndRewardsFromFile()
	}
	return accountStats, err
}

// GetAccountsAndRewardsFromDB get from database
func (opt *Option) GetAccountsAndRewardsFromDB() (accountStats mongodb.AccountStatSlice) {
	exchange := opt.Exchange
	step := opt.StepCount
	if step == 0 {
		return getSingleCycleRewardsFromDB(opt.TotalValue, exchange, opt.StartHeight, opt.EndHeight)
	}

	// use map to statistic
	finStatMap := make(map[common.Address]*mongodb.AccountStat)

	steps := (opt.EndHeight - opt.StartHeight) / step
	stepRewards := new(big.Int).Div(opt.TotalValue, new(big.Int).SetUint64(steps))

	for start := opt.StartHeight; start < opt.EndHeight; start += step {
		cycleStats := getSingleCycleRewardsFromDB(stepRewards, exchange, start, start+step)
		if len(cycleStats) == 0 {
			_ = opt.WriteNoVolumeOutput(exchange, start, start+step)
			opt.noVolumes++
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
	accountStats = mongodb.ConvertToSortedSlice(finStatMap)
	log.Info("get account volumes from db success", "exchange", exchange, "start", opt.StartHeight, "end", opt.EndHeight, "step", opt.StepCount, "missSteps", opt.noVolumes)
	_ = opt.WriteNoVolumeSummary()
	return accountStats
}

func getSingleCycleRewardsFromDB(totalRewards *big.Int, exchange string, startHeight, endHeight uint64) mongodb.AccountStatSlice {
	accountStats := mongodb.FindAccountVolumes(exchange, startHeight, endHeight)
	if len(accountStats) == 0 {
		return nil
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

// GetAccountsAndRewardsFromFile pass line format "<address> <amount>" from input file
func (opt *Option) GetAccountsAndRewardsFromFile() (accountStats mongodb.AccountStatSlice, titleLine string, err error) {
	if opt.InputFile == "" {
		return nil, "", fmt.Errorf("get account rewards from file error, no input file specified")
	}
	file, err := os.Open(opt.InputFile)
	if err != nil {
		return nil, "", fmt.Errorf("open %v failed. %v)", opt.InputFile, err)
	}
	defer file.Close()

	accountStats = make(mongodb.AccountStatSlice, 0)

	reader := bufio.NewReader(file)
	isTitleLine := true

	for {
		lineData, _, errf := reader.ReadLine()
		if errf == io.EOF {
			break
		}
		line := strings.TrimSpace(string(lineData))
		if isCommentedLine(line) {
			if isTitleLine {
				titleLine = line
				isTitleLine = false
			}
			continue
		}
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
		if accountStats.IsAccountExist(account) {
			return nil, "", fmt.Errorf("has duplicate account %v", accountStr)
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

func isCommentedLine(line string) bool {
	return strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//")
}
