package main

import (
	"bufio"
	"fmt"
	"io"
	"math/big"
	"os"
	"regexp"
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

	exchange    = strings.ToLower("0x049ddc3cd20ac7a2f6c867680f7e21de70aca9c3")
	rewardToken = strings.ToLower("0x0c74199d22f732039e843366a236ff4f61986b32")

	cycleLen               uint64 = 6600
	startHeight, endHeight uint64

	re = regexp.MustCompile(`[\s,]+`) // blank or comma separated
)

var (
	importRewardsCommand = &cli.Command{
		Action:    importRewards,
		Name:      "importrewards",
		Usage:     "import rewards result to database",
		ArgsUsage: " ",
		Description: `
import rewards result to database.
input file line format is: <account>,<reward>.<share>,<number>[,<txhash>]
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

	if ctx.IsSet(utils.ExchangeFlag.Name) {
		exchange = ctx.String(utils.ExchangeFlag.Name)
	}

	if ctx.IsSet(utils.RewardTokenFlag.Name) {
		rewardToken = ctx.String(utils.RewardTokenFlag.Name)
	}

	if ctx.IsSet(utils.RewardTyepFlag.Name) {
		rewardTypeStr := ctx.String(utils.RewardTyepFlag.Name)
		checkRewardType(rewardTypeStr)
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

	titleLineData, _, _ := inputReader.ReadLine()
	titleLine := strings.TrimSpace(string(titleLineData))
	chekcTitleLine(titleLine)
}

func closeFile() {
	inputFile.Close()
}

func checkRewardType(rewardTypeStr string) {
	var mismatch bool
	switch rewardTypeStr {
	case "liquid", "liquidity":
		isLiquidReward = true
		if isVolumeReward {
			mismatch = true
		}
	case "volume", "trade":
		isVolumeReward = true
		if isLiquidReward {
			mismatch = true
		}
	default:
		log.Fatalf("unknown reward type '%v'", rewardTypeStr)
	}
	if rewardType == "" {
		rewardType = rewardTypeStr
	}
	if mismatch {
		log.Fatalf("reward type mismatch. from arg %v, from file %v", rewardType, rewardTypeStr)
	}
}

func chekcTitleLine(titleLine string) {
	if !isCommentedLine(titleLine) {
		log.Fatalf("no title line in input file %v", inputFileName)
	}
	titleParts := re.Split(titleLine, -1)
	if len(titleParts) < 5 {
		log.Fatalf("input file title line parts is less than 5. line: %v", titleLine)
	}
	rewardTypeStr := titleParts[2]
	checkRewardType(rewardTypeStr)
	log.Info("check reward type in title line success", "rewardType", rewardTypeStr)
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

func processLine(line string, addToDB bool) {
	parts := re.Split(line, -1)
	if len(parts) < 4 {
		log.Fatalf("wrong parts of input line: %v", line)
	}
	accountStr := parts[0]
	rewardStr := parts[1]
	shareStr := parts[2]
	numberStr := parts[3]
	if !common.IsHexAddress(accountStr) {
		log.Fatalf("wrong address in input line: %v", line)
	}
	account := common.HexToAddress(accountStr)
	reward, ok := new(big.Int).SetString(rewardStr, 10)
	if !ok {
		log.Fatalf("wrong reward in input line: %v", line)
	}
	share, ok := new(big.Int).SetString(shareStr, 10)
	if !ok {
		log.Fatalf("wrong share in input line: %v", line)
	}
	number, ok := new(big.Int).SetString(numberStr, 10)
	if !ok {
		log.Fatalf("wrong number in input line: %v", line)
	}
	var txhash *common.Hash
	if len(parts) >= 4 {
		txHashStr := parts[4]
		hash := common.HexToHash(txHashStr)
		if hash.String() != txHashStr {
			log.Fatalf("wrong txhash in input line: %v", line)
		}
		txhash = &hash
	}

	if addToDB {
		accountStr := strings.ToLower(account.String())
		txHashStr := ""
		if txhash != nil {
			txHashStr = txhash.String()
		}
		log.Trace("read input line success", "account", accountStr, "reward", reward, "share", share, "number", number, "txhash", txHashStr)
		addRewardResultToDB(account, reward, share, number, txhash)
	}
}

func addRewardResultToDB(account common.Address, reward, share, number *big.Int, txhash *common.Hash) {
	accoutStr := strings.ToLower(account.String())
	rewardStr := reward.String()
	pairs := params.GetExchangePairs(exchange)
	shareStr := share.String()
	numVal := number.Uint64()

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
