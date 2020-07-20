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
	BuildTxArgs *BuildTxArgs
	TotalValue  *big.Int
	StartHeight uint64 // start inclusive
	EndHeight   uint64 // end exclusive
	StepCount   uint64
	Exchange    string
	RewardToken string
	InputFile   string
	OutputFile  string
	DryRun      bool

	byWhat     string
	outputFile *os.File
}

// ByWhat distribute by what method
func (opt *Option) ByWhat() string {
	return opt.byWhat
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
	return fmt.Sprintf(
		"%v TotalValue %v StartHeight %v EndHeight %v Exchange %v RewardToken %v DryRun %v Sender %v ChainID %v",
		opt.byWhat, opt.TotalValue, opt.StartHeight, opt.EndHeight,
		opt.Exchange, opt.RewardToken, opt.DryRun,
		opt.GetSender().String(), opt.GetChainID(),
	)
}

func (opt *Option) deinit() {
	if opt.outputFile != nil {
		opt.outputFile.Close()
	}
}

func (opt *Option) checkAndInit() (err error) {
	if opt.StartHeight >= opt.EndHeight {
		return fmt.Errorf("empty range, start height %v >= end height %v", opt.StartHeight, opt.EndHeight)
	}
	if opt.StepCount != 0 && (opt.EndHeight-opt.StartHeight)%opt.StepCount != 0 {
		return fmt.Errorf("cycle length %v is not intergral multiple of step %v", opt.EndHeight-opt.StartHeight, opt.StepCount)
	}
	if opt.StepCount != 0 {
		steps := (opt.EndHeight - opt.StartHeight) / opt.StepCount
		if new(big.Int).Mod(opt.TotalValue, new(big.Int).SetUint64(steps)).Sign() != 0 {
			return fmt.Errorf("total value %v is not intergral multiple of steps %v", opt.TotalValue, steps)
		}
	}

	if !params.IsConfigedExchange(opt.Exchange) {
		return fmt.Errorf("exchange %v is not configed", opt.Exchange)
	}
	if !common.IsHexAddress(opt.RewardToken) {
		return fmt.Errorf("wrong reward token: '%v'", opt.RewardToken)
	}
	err = opt.CheckSenderRewardTokenBalance()
	if err != nil {
		if opt.DryRun {
			log.Warn("check sender reward token balance failed, but ignore in dry run", "err", err)
		} else {
			return err
		}
	}
	latestBlock := capi.LoopGetLatestBlockHeader()
	if latestBlock.Number.Uint64() < opt.EndHeight {
		return fmt.Errorf("latest height %v is lower than end height %v", latestBlock.Number, opt.EndHeight)
	}
	return nil
}

func (opt *Option) getDefaultOutputFile() string {
	return fmt.Sprintf("distribute-%s-%d-%d-%s-%d.txt", opt.byWhat, opt.StartHeight, opt.EndHeight, opt.Exchange, time.Now().Unix())
}

func (opt *Option) openOutputFile() (err error) {
	if opt.outputFile != nil {
		return nil // already opened
	}
	if opt.OutputFile == "" {
		opt.OutputFile = opt.getDefaultOutputFile()
	}
	opt.outputFile, err = os.OpenFile(opt.OutputFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Info("open output file error", "file", opt.OutputFile)
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
		log.Info("[write output] error", "msg", msg, "err", err)
	} else {
		log.Printf("[write output] %v", msg)
	}
	return err
}

// WriteOutput write output
func (opt *Option) WriteOutput(contents ...string) error {
	msg := strings.Join(contents, " ")
	return opt.WriteOutputLine(msg)
}

// WriteLiquiditySubject write liquidity subject
func (opt *Option) WriteLiquiditySubject(exchange string, start, end uint64, numAccounts int) error {
	msg := fmt.Sprintf("getLiquidity exchange=%v start=%v end=%v accounts=%v", exchange, start, end, numAccounts)
	return opt.WriteOutputLine(msg)
}

// WriteLiquiditySummary write liquidity summary
func (opt *Option) WriteLiquiditySummary(exchange string, start, end uint64, numAccounts int, totalShares, totalRewards *big.Int) error {
	msg := fmt.Sprintf("getLiquidity exchange=%v start=%v end=%v accounts=%v totalShares=%v totalRewards=%v", exchange, start, end, numAccounts, totalShares, totalRewards)
	return opt.WriteOutputLine(msg)
}

// WriteLiquidityBalance write liquidity balance
func (opt *Option) WriteLiquidityBalance(account common.Address, value *big.Int, height uint64) error {
	msg := fmt.Sprintf("getLiquidity %v %v height=%v", strings.ToLower(account.Hex()), value, height)
	return opt.WriteOutputLine(msg)
}

// WriteSendRewardResult write send reward result
func (opt *Option) WriteSendRewardResult(account common.Address, reward *big.Int, txHash *common.Hash) error {
	prefix := "sendRewards"
	accoutStr := strings.ToLower(account.Hex())
	if txHash == nil {
		return opt.WriteOutput(prefix, accoutStr, reward.String(), "dryrun")
	}
	return opt.WriteOutput(prefix, accoutStr, reward.String(), txHash.Hex())
}

// WriteNoVolumeOutput write output
func (opt *Option) WriteNoVolumeOutput(exchange string, start, end uint64) error {
	msg := fmt.Sprintf("calcRewards exchange=%s start=%d end=%d novolume", exchange, start, end)
	return opt.WriteOutputLine(msg)
}

// WriteNoVolumeSummary write no volume summary
func (opt *Option) WriteNoVolumeSummary(exchange string, start, end, miss uint64) error {
	msg := fmt.Sprintf("calcRewards exchange=%s start=%d end=%d novolumes=%v", exchange, start, end, miss)
	return opt.WriteOutputLine(msg)
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
			return fmt.Errorf("not enough reward token balance, %v < %v, sender: %v token: %v", senderTokenBalance, opt.TotalValue, sender.String(), opt.RewardToken)
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
		accounts = append(accounts, account)
	}

	return accounts, nil
}

// GetAccountsAndRewards get from file if input file exist, or else from database
func (opt *Option) GetAccountsAndRewards() (accounts []common.Address, rewards []*big.Int, missSteps uint64, err error) {
	if opt.InputFile == "" {
		accounts, rewards, missSteps = opt.GetAccountsAndRewardsFromDB()
	} else {
		accounts, rewards, err = opt.GetAccountsAndRewardsFromFile()
	}
	return accounts, rewards, missSteps, err
}

// GetAccountsAndRewardsFromDB get from database
func (opt *Option) GetAccountsAndRewardsFromDB() (accounts []common.Address, rewards []*big.Int, missSteps uint64) {
	exchange := opt.Exchange
	step := opt.StepCount
	if step == 0 {
		accounts, rewards = opt.getSingleCycleRewardsFromDB(opt.TotalValue, exchange, opt.StartHeight, opt.EndHeight)
		return accounts, rewards, 0
	}
	rewardsMap := make(map[common.Address]*big.Int)
	steps := (opt.EndHeight - opt.StartHeight) / step
	stepRewards := new(big.Int).Div(opt.TotalValue, new(big.Int).SetUint64(steps))
	for start := opt.StartHeight; start < opt.EndHeight; start += step {
		accounts, rewards = opt.getSingleCycleRewardsFromDB(stepRewards, exchange, start, start+step)
		if len(accounts) == 0 {
			_ = opt.WriteNoVolumeOutput(exchange, start, start+step)
			missSteps++
			continue
		}
		for i, account := range accounts {
			reward := rewards[i]
			if reward == nil || reward.Sign() <= 0 {
				log.Warn("non positive reward exist, please check")
				continue
			}
			old, exist := rewardsMap[account]
			if exist {
				rewardsMap[account].Add(old, reward)
			} else {
				rewardsMap[account] = reward
			}
		}
	}
	// convert map to slice
	length := len(rewardsMap)
	accounts = make([]common.Address, 0, length)
	rewards = make([]*big.Int, 0, length)
	for acc, reward := range rewardsMap {
		accounts = append(accounts, acc)
		rewards = append(rewards, reward)
	}
	log.Info("get account volumes from db success", "exchange", exchange, "start", opt.StartHeight, "end", opt.EndHeight, "step", opt.StepCount, "missSteps", missSteps)
	_ = opt.WriteNoVolumeSummary(exchange, opt.StartHeight, opt.EndHeight, missSteps)
	return accounts, rewards, missSteps
}

func (opt *Option) getSingleCycleRewardsFromDB(totalRewards *big.Int, exchange string, startHeight, endHeight uint64) (accounts []common.Address, rewards []*big.Int) {
	var volumes []*big.Int
	accounts, volumes = mongodb.FindAccountVolumes(exchange, startHeight, endHeight)
	if len(accounts) == 0 {
		return nil, nil
	}
	rewards = CalcRewardsByShares(totalRewards, accounts, volumes)
	opt.writeRewards(accounts, rewards, volumes, exchange, startHeight, endHeight, totalRewards)
	return accounts, rewards
}

func (opt *Option) writeRewards(accounts []common.Address, rewards, shares []*big.Int, exchange string, startHeight, endHeight uint64, totalRewards *big.Int) {
	subject := fmt.Sprintf("calcRewards exchange=%v start=%v end=%v rewards=%v accounts=%v", exchange, startHeight, endHeight, totalRewards, len(accounts))
	_ = opt.WriteOutputLine(subject)
	for i, account := range accounts {
		line := fmt.Sprintf("calcRewards %v %v start=%v end=%v share=%v", strings.ToLower(account.String()), rewards[i], startHeight, endHeight, shares[i])
		_ = opt.WriteOutputLine(line)
	}
}

// GetAccountsAndRewardsFromFile pass line format "<address> <amount>" from input file
func (opt *Option) GetAccountsAndRewardsFromFile() (accounts []common.Address, rewards []*big.Int, err error) {
	if opt.InputFile == "" {
		return nil, nil, fmt.Errorf("get account rewards from file error, no input file specified")
	}
	file, err := os.Open(opt.InputFile)
	if err != nil {
		return nil, nil, fmt.Errorf("open %v failed. %v)", opt.InputFile, err)
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
		parts := strings.Split(line, " ")
		if len(parts) < 2 {
			return nil, nil, fmt.Errorf("less than 2 parts in line %v", line)
		}
		accountStr := parts[0]
		rewardStr := parts[1]
		if !common.IsHexAddress(accountStr) {
			return nil, nil, fmt.Errorf("wrong address in line %v", line)
		}
		reward, err := tools.GetBigIntFromString(rewardStr)
		if err != nil {
			return nil, nil, err
		}
		if reward.Sign() <= 0 {
			continue
		}
		account := common.HexToAddress(line)
		accounts = append(accounts, account)
		rewards = append(rewards, reward)
	}

	return accounts, rewards, nil
}

func isCommentedLine(line string) bool {
	return strings.HasPrefix(line, "#")
}
