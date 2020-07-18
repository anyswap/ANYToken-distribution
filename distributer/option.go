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

func (opt *Option) String() string {
	return fmt.Sprintf("%v TotalValue %v StartHeight %v EndHeight %v Exchange %v RewardToken %v DryRun %v", opt.byWhat, opt.TotalValue, opt.StartHeight, opt.EndHeight, opt.Exchange, opt.RewardToken, opt.DryRun)
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
			return fmt.Errorf("not enough reward token balance, %v < %v", senderTokenBalance, opt.TotalValue)
		}
		break
	}
	log.Info("sender reward token balance is enough", "token", rewardTokenAddr.String(), "balance", senderTokenBalance, "needed", opt.TotalValue)
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

// GetAccountsAndVolumes pass line format "<address> <amount>" from input file
func (opt *Option) GetAccountsAndVolumes() (accounts []common.Address, volumes []*big.Int, err error) {
	if opt.InputFile == "" {
		accounts, volumes = mongodb.FindAccountVolumes(opt.Exchange, opt.StartHeight, opt.EndHeight)
		return accounts, volumes, nil
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
