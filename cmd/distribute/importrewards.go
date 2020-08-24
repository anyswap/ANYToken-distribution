package main

import (
	"bufio"
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/anyswap/ANYToken-distribution/cmd/utils"
	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/mongodb"
	"github.com/anyswap/ANYToken-distribution/params"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
	"github.com/urfave/cli/v2"
)

var (
	inputFileName string
	inputFile     *os.File
	inputReader   *bufio.Reader

	dryRun         bool
	rewardType     string
	isLiquidReward bool
	isVolumeReward bool

	exchange    string
	rewardToken string

	cycleLen               uint64 = 6600
	startHeight, endHeight uint64
)

var (
	importRewardsCommand = &cli.Command{
		Action:    importRewards,
		Name:      "importrewards",
		Usage:     "import rewards result to database",
		ArgsUsage: " ",
		Description: `
import rewards result to database.
input file line format is: <account>,<reward>,[<share>,<number>],[<txhash>]
`,
		Flags: []cli.Flag{
			utils.RewardTyepFlag,
			utils.ExchangeFlag,
			utils.RewardTokenFlag,
			utils.StartHeightFlag,
			utils.EndHeightFlag,
			utils.InputFileFlag,
			utils.DryRunFlag,
		},
	}
)

func importRewards(ctx *cli.Context) error {
	inputFileName = ctx.String(utils.InputFileFlag.Name)
	if inputFileName == "" {
		log.Fatal("must specify --input option to specify input file")
	}

	rewardType = ctx.String(utils.RewardTyepFlag.Name)
	if rewardType == "" {
		return fmt.Errorf("must specify rewardType")
	}
	checkRewardType(rewardType)

	isStartHeightSet := ctx.IsSet(utils.StartHeightFlag.Name)
	isEndHeightSet := ctx.IsSet(utils.EndHeightFlag.Name)
	if !isStartHeightSet && !isEndHeightSet {
		log.Fatal("must specify --start or --end to specify cycle range")
	}

	if isStartHeightSet {
		startHeight = ctx.Uint64(utils.StartHeightFlag.Name)
		if !isEndHeightSet {
			endHeight = startHeight + cycleLen
		}
	}

	if isEndHeightSet {
		endHeight = ctx.Uint64(utils.EndHeightFlag.Name)
		if !isStartHeightSet {
			startHeight = endHeight - cycleLen
		}
	}

	if endHeight-startHeight != cycleLen {
		log.Fatalf("cycle length %v = end %v - start %v is not equal to %v", endHeight-startHeight, endHeight, startHeight, cycleLen)
	}

	exchange = ctx.String(utils.ExchangeFlag.Name)
	if exchange == "" {
		return fmt.Errorf("must specify exchange")
	}

	rewardToken = ctx.String(utils.RewardTokenFlag.Name)
	if rewardToken == "" {
		return fmt.Errorf("must specify rewardToken")
	}

	dryRun = ctx.Bool(utils.DryRunFlag.Name)

	capi := utils.InitApp(ctx, true)
	defer capi.CloseClient()

	log.Info("run importRewards", "exchange", exchange, "rewardToken", rewardToken, "startHeight", startHeight, "endHeight", endHeight, "dryRun", dryRun)

	processFile()

	return nil
}

func openFile() {
	var err error
	inputFile, err = os.Open(inputFileName)
	if err != nil {
		log.Fatalf("open %v failed. %v)", inputFileName, err)
	}
	log.Info("open input file success", "file", inputFileName)
	inputReader = bufio.NewReader(inputFile)
}

func closeFile() {
	inputFile.Close()
}

func checkRewardType(rewardTypeStr string) {
	switch rewardTypeStr {
	case "liquid", "liquidity":
		isLiquidReward = true
	case "volume", "trade":
		isVolumeReward = true
	default:
		log.Fatalf("unknown reward type '%v'", rewardTypeStr)
	}
}

func isCommentedLine(line string) bool {
	return strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//")
}

func readOneLine(reader *bufio.Reader) string {
	for {
		slineData, _, err := reader.ReadLine()
		if err == io.EOF {
			break
		}
		line := strings.TrimSpace(string(slineData))
		if line != "" && !isCommentedLine(line) {
			return line
		}
	}
	return ""
}

func processFile() {
	log.Info("start process file", "inputFileName", inputFileName, "dryRun", dryRun)
	openFile()
	defer closeFile()

	verifyFile()

	for {
		line := readOneLine(inputReader)
		if line == "" {
			log.Info("read to file end")
			break
		}
		processLine(line, true)
	}
	log.Info("process file finish.", "rewardType", rewardType, "inputFileName", inputFileName, "dryRun", dryRun)
}

func verifyFile() {
	for {
		line := readOneLine(inputReader)
		if line == "" {
			break
		}
		processLine(line, false)
	}
	log.Info("verify input file success")
	var err error
	_, err = inputFile.Seek(0, 0)
	if err != nil {
		log.Warn("seek input file failed", "err", err)
	}
}

// line format is: <account>,<reward>
// or: <account>,<reward>,<txhash>
// or: <account>,<reward>,<share>,<number>
// or: <account>,<reward>,<share>,<number>,<txhash>
func processLine(line string, addToDB bool) {
	parts := blankOrCommaSepRegexp.Split(line, -1)
	if len(parts) < 2 {
		log.Fatalf("wrong parts of input line: %v", line)
	}
	var accountStr, rewardStr, shareStr, numberStr, txHashStr string
	accountStr = parts[0]
	rewardStr = parts[1]
	if len(parts) > 3 {
		shareStr = parts[2]
		numberStr = parts[3]
		if len(parts) > 4 {
			txHashStr = parts[4]
		}
	} else if len(parts) == 3 {
		txHashStr = parts[2]
	}
	if !common.IsHexAddress(accountStr) {
		log.Fatalf("wrong address in input line: %v", line)
	}
	account := common.HexToAddress(accountStr)
	reward, ok := new(big.Int).SetString(rewardStr, 10)
	if !ok {
		log.Fatalf("wrong reward in input line: %v", line)
	}
	var share *big.Int
	if shareStr != "" {
		share, ok = new(big.Int).SetString(shareStr, 10)
		if !ok {
			log.Fatalf("wrong share in input line: %v", line)
		}
	}
	var number *big.Int
	if numberStr != "" {
		number, ok = new(big.Int).SetString(numberStr, 10)
		if !ok {
			log.Fatalf("wrong number in input line: %v", line)
		}
	}
	var txhash *common.Hash
	if txHashStr != "" {
		hash := common.HexToHash(txHashStr)
		if hash.String() != txHashStr {
			log.Fatalf("wrong txhash in input line: %v", line)
		}
		txhash = &hash
	}

	if addToDB {
		accountStr := strings.ToLower(account.String())
		log.Trace("read input line success", "account", accountStr, "reward", reward, "share", share, "number", number, "txhash", txHashStr)
		addRewardResultToDB(account, reward, share, number, txhash)
	}
}

func addRewardResultToDB(account common.Address, reward, share, number *big.Int, txhash *common.Hash) {
	accoutStr := strings.ToLower(account.String())
	rewardStr := reward.String()
	pairs := params.GetExchangePairs(exchange)

	var shareStr string
	if share != nil {
		shareStr = share.String()
	}

	var numVal uint64
	if number != nil {
		numVal = number.Uint64()
	}

	var hashStr string
	if txhash != nil {
		hashStr = txhash.Hex()
	}

	if dryRun {
		subject := fmt.Sprintf("[dryRun] add %v reward result", rewardType)
		log.Info(subject,
			"account", accoutStr, "reward", reward,
			"share", share, "number", number, "txhash", hashStr,
			"exchange", exchange, "pairs", pairs, "rewardToken", rewardToken,
			"startHeight", startHeight, "endHeight", endHeight,
		)
		return
	}

	switch {
	case isVolumeReward:
		mr := &mongodb.MgoVolumeRewardResult{
			Key:         mongodb.GetKeyOfRewardResult(exchange, accoutStr, startHeight),
			Exchange:    exchange,
			Pairs:       pairs,
			Start:       startHeight,
			End:         endHeight,
			RewardToken: rewardToken,
			Account:     accoutStr,
			Reward:      rewardStr,
			Volume:      shareStr,
			TxCount:     numVal,
			RewardTx:    hashStr,
			Timestamp:   uint64(time.Now().Unix()),
		}
		_ = mongodb.TryDoTimes("AddVolumeRewardResult "+mr.Key, func() error {
			return mongodb.AddVolumeRewardResult(mr)
		})
	case isLiquidReward:
		mr := &mongodb.MgoLiquidRewardResult{
			Key:         mongodb.GetKeyOfRewardResult(exchange, accoutStr, startHeight),
			Exchange:    exchange,
			Pairs:       pairs,
			Start:       startHeight,
			End:         endHeight,
			RewardToken: rewardToken,
			Account:     accoutStr,
			Reward:      rewardStr,
			Liquidity:   shareStr,
			Height:      numVal,
			RewardTx:    hashStr,
			Timestamp:   uint64(time.Now().Unix()),
		}
		_ = mongodb.TryDoTimes("AddLiquidRewardResult "+mr.Key, func() error {
			return mongodb.AddLiquidRewardResult(mr)
		})
	default:
		log.Fatalf("can only import liquid or volume rewards. wrong reward type %v", rewardType)
	}
}
