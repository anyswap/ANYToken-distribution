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

	if !params.IsConfigedExchange(opt.Exchange) {
		return fmt.Errorf("exchange %v is not configed", opt.Exchange)
	}
	if !common.IsHexAddress(opt.RewardToken) {
		return fmt.Errorf("wrong reward token: '%v'", opt.RewardToken)
	}
	err = opt.CheckSenderRewardTokenBalance()
	if err != nil {
		return err
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
	if opt.OutputFile == "" {
		opt.OutputFile = opt.getDefaultOutputFile()
	}
	opt.outputFile, err = os.OpenFile(opt.OutputFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Info("open output file error", "file", opt.OutputFile)
	}
	return err
}

// WriteOutput write output
func (opt *Option) WriteOutput(account, reward, txHash string) error {
	if opt.outputFile == nil {
		err := opt.openOutputFile()
		if err != nil {
			return err
		}
	}
	msg := fmt.Sprintf("%s %s %s\n", account, reward, txHash)
	_, err := opt.outputFile.Write([]byte(msg))
	if err != nil {
		log.Info("write output error", "msg", msg, "err", err)
	}
	return err
}

// WriteNoVolumeOutput write output
func (opt *Option) WriteNoVolumeOutput(exchange string, start, end uint64) error {
	if opt.outputFile == nil {
		err := opt.openOutputFile()
		if err != nil {
			return err
		}
	}
	msg := fmt.Sprintf("novolume %s %d %d\n", exchange, start, end)
	_, err := opt.outputFile.Write([]byte(msg))
	if err != nil {
		log.Info("write output error", "msg", msg, "err", err)
	}
	return err
}

// SendRewardsTransaction send rewards
func (opt *Option) SendRewardsTransaction(account common.Address, reward *big.Int) (txHash common.Hash, err error) {
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

// GetAccountsAndVolumes get from file if input file exist, or else from database
func (opt *Option) GetAccountsAndVolumes() (accounts []common.Address, volumes []*big.Int, missSteps uint64, err error) {
	if opt.InputFile == "" {
		return opt.GetAccountsAndVolumesFromDB()
	}
	accounts, volumes, err = opt.GetAccountsAndVolumesFromFile()
	return accounts, volumes, 0, err
}

// GetAccountsAndVolumesFromDB get from database
func (opt *Option) GetAccountsAndVolumesFromDB() (accounts []common.Address, volumes []*big.Int, missSteps uint64, err error) {
	exchange := opt.Exchange
	step := opt.StepCount
	if step == 0 {
		accounts, volumes = mongodb.FindAccountVolumes(exchange, opt.StartHeight, opt.EndHeight)
		return accounts, volumes, 0, nil
	}
	volumeMap := make(map[common.Address]*big.Int)
	for start := opt.StartHeight; start < opt.EndHeight; start += step {
		accounts, volumes = mongodb.FindAccountVolumes(exchange, start, start+step)
		if len(accounts) == 0 {
			log.Info("find miss volume", "exchange", exchange, "start", start, "end", start+step)
			missSteps++
			_ = opt.WriteNoVolumeOutput(exchange, start, start+step)
			continue
		}
		for i, account := range accounts {
			volume := volumes[i]
			if volume == nil || volume.Sign() <= 0 {
				continue
			}
			old, exist := volumeMap[account]
			if exist {
				volumeMap[account].Add(old, volume)
			} else {
				volumeMap[account] = volume
			}
		}
	}
	length := len(volumeMap)
	accounts = make([]common.Address, 0, length)
	volumes = make([]*big.Int, 0, length)
	for acc, vol := range volumeMap {
		accounts = append(accounts, acc)
		volumes = append(volumes, vol)
	}
	log.Info("get account volumes from db success", "exchange", exchange, "start", opt.StartHeight, "end", opt.EndHeight, "step", opt.StepCount, "missSteps", missSteps)
	return accounts, volumes, missSteps, nil
}

// GetAccountsAndVolumesFromFile pass line format "<address> <amount>" from input file
func (opt *Option) GetAccountsAndVolumesFromFile() (accounts []common.Address, volumes []*big.Int, err error) {
	if opt.InputFile == "" {
		return nil, nil, fmt.Errorf("get account volumes from file error, no input file specified")
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
		volumeStr := parts[1]
		if !common.IsHexAddress(accountStr) {
			return nil, nil, fmt.Errorf("wrong address in line %v", line)
		}
		volume, err := tools.GetBigIntFromString(volumeStr)
		if err != nil {
			return nil, nil, err
		}
		if volume.Sign() <= 0 {
			continue
		}
		account := common.HexToAddress(line)
		accounts = append(accounts, account)
		volumes = append(volumes, volume)
	}

	return accounts, volumes, nil
}

func isCommentedLine(line string) bool {
	return strings.HasPrefix(line, "#")
}
