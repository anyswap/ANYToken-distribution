package distributer

import (
	"bufio"
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"

	"github.com/anyswap/ANYToken-distribution/mongodb"
	"github.com/anyswap/ANYToken-distribution/params"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
)

const sampleCount = 4

var outputFile *os.File

// Option ditribute options
type Option struct {
	TotalValue   *big.Int
	StartHeight  uint64
	EndHeight    uint64
	Exchange     string
	AccountsFile string
	OutputFile   string
	DryRun       bool
}

func (opt *Option) deinit() {
	if outputFile != nil {
		outputFile.Close()
	}
}

func (opt *Option) checkAndInit() (err error) {
	if opt.TotalValue == nil || opt.TotalValue.Sign() <= 0 {
		return fmt.Errorf("wrong total value %v", opt.TotalValue)
	}
	if opt.StartHeight+sampleCount > opt.EndHeight {
		return fmt.Errorf("start height %v + sample count %v is greater than end height %v", opt.StartHeight, sampleCount, opt.EndHeight)
	}
	if !params.IsConfigedExchange(opt.Exchange) {
		return fmt.Errorf("exchange %v is not configed", opt.Exchange)
	}
	latestBlock := capi.LoopGetLatestBlockHeader()
	if latestBlock.Number.Uint64() < opt.EndHeight {
		return fmt.Errorf("latest height %v is lower than end height %v", latestBlock.Number, opt.EndHeight)
	}
	if opt.OutputFile != "" {
		outputFile, err = os.OpenFile(opt.OutputFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
	}
	return nil
}

func (opt *Option) getAccounts() (accounts []common.Address, err error) {
	if opt.AccountsFile == "" {
		accounts = mongodb.FindAllAccounts(opt.Exchange)
		return accounts, nil
	}

	file, err := os.Open(opt.AccountsFile)
	if err != nil {
		return nil, fmt.Errorf("open %v failed. %v)", opt.AccountsFile, err)
	}

	reader := bufio.NewReader(file)
	for {
		lineData, _, errf := reader.ReadLine()
		if errf == io.EOF {
			break
		}
		line := strings.TrimSpace(string(lineData))
		if !common.IsHexAddress(line) {
			return nil, fmt.Errorf("found wrong address line %v", line)
		}
		account := common.HexToAddress(line)
		accounts = append(accounts, account)
	}

	return accounts, nil
}
