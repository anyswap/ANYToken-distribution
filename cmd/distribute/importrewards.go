package main

import (
	"bufio"
	"fmt"
	"io"
	"math/big"
	"os"
	"regexp"
	"strings"

	"github.com/anyswap/ANYToken-distribution/cmd/utils"
	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/mongodb"
	"github.com/anyswap/ANYToken-distribution/params"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
	"github.com/urfave/cli/v2"
)

var (
	statisticFile string
	resultFile    string
	statFile      *os.File
	resFile       *os.File
	statReader    *bufio.Reader
	resReader     *bufio.Reader

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
		Usage:     "import send rewards result to database",
		ArgsUsage: " ",
		Description: `
import send rewards result to database.
`,
		Flags: []cli.Flag{
			utils.ExchangeFlag,
			utils.RewardTokenFlag,
			utils.StartHeightFlag,
			utils.EndHeightFlag,
			utils.StatisticFileFlag,
			utils.ResultFileFlag,
			utils.DryRunFlag,
		},
	}
)

func importRewards(ctx *cli.Context) error {
	utils.SetLogger(ctx)

	configFile := utils.GetConfigFilePath(ctx)
	params.LoadConfig(configFile)

	statisticFile = ctx.String(utils.StatisticFileFlag.Name)
	if statisticFile == "" {
		log.Fatal("must specify --statistic option to specify statistic file")
	}

	resultFile = ctx.String(utils.ResultFileFlag.Name)
	if resultFile == "" {
		log.Fatal("must specify --result option to specify result file")
	}

	if ctx.IsSet(utils.ExchangeFlag.Name) {
		exchange = ctx.String(utils.ExchangeFlag.Name)
	}

	if ctx.IsSet(utils.RewardTokenFlag.Name) {
		rewardToken = ctx.String(utils.RewardTokenFlag.Name)
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

	dryRun = ctx.Bool(utils.DryRunFlag.Name)
	if !dryRun {
		utils.InitMongodb()
	}

	log.Info("run importRewards", "exchange", exchange, "rewardToken", rewardToken, "startHeight", startHeight, "endHeight", endHeight, "dryRun", dryRun)

	processFile()

	return nil
}

func openFile() {
	var err error

	if statisticFile != "" {
		statFile, err = os.Open(statisticFile)
		if err != nil {
			log.Fatalf("open %v failed. %v)", statisticFile, err)
		}
		log.Info("open statistic file success", "file", statisticFile)
		statReader = bufio.NewReader(statFile)

		titleLineData, _, _ := statReader.ReadLine()
		titleLine := strings.TrimSpace(string(titleLineData))
		chekcStatisticTitleLine(titleLine)
	}

	if resultFile != "" {
		resFile, err = os.Open(resultFile)
		if err != nil {
			log.Fatalf("open %v failed. %v)", resultFile, err)
		}
		log.Info("open result file success", "file", resultFile)
		resReader = bufio.NewReader(resFile)
	}
}

func closeFile() {
	if statFile != nil {
		statFile.Close()
	}
	if resFile != nil {
		resFile.Close()
	}
}

func chekcStatisticTitleLine(titleLine string) {
	if !isCommentedLine(titleLine) {
		log.Fatalf("no title line in statistic file %v", statisticFile)
	}
	titleParts := re.Split(titleLine, -1)
	if len(titleParts) != 5 {
		log.Fatalf("statistic file title line parts is not 5. line: %v", titleLine)
	}
	rewardType = titleParts[2]
	switch rewardType {
	case "liquid", "liquidity":
		isLiquidReward = true
	case "volume", "trade":
		isVolumeReward = true
	default:
		log.Fatalf("unknown reward type '%v' in title line %v", rewardType, titleLine)
	}
	log.Info("get reward type success", "rewardType", rewardType)
}

func isCommentedLine(line string) bool {
	return strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//")
}

func readOneLine(reader *bufio.Reader) string {
	if reader == nil {
		return ""
	}
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
	log.Info("start process file", "statisticFile", statisticFile, "resultFile", resultFile, "dryRun", dryRun)
	openFile()
	defer closeFile()

	if statFile != nil && resFile != nil {
		compareAndVerifyFile()
	}

	for {
		statLine := readOneLine(statReader)
		resLine := readOneLine(resReader)
		if statLine == "" && resLine == "" {
			log.Info("read to file end")
			break
		}
		processLine(statLine, resLine)
	}
	log.Info("process file finish.", "rewardType", rewardType, "statisticFile", statisticFile, "resultFile", resultFile, "dryRun", dryRun)
}

func compareAndVerifyFile() {
	for {
		statLine := readOneLine(statReader)
		resLine := readOneLine(resReader)
		if statLine == "" && resLine == "" {
			break
		}
		statParts := re.Split(statLine, -1)
		resParts := re.Split(resLine, -1)
		if len(statParts) != 4 {
			log.Fatalf("wrong parts of statistic line: %v", statLine)
		}
		if len(resParts) != 3 {
			log.Fatalf("wrong parts of result line: %v", resLine)
		}
		if statParts[0] != resParts[0] || statParts[1] != resParts[1] {
			log.Fatalf("line mismatch. statLine '%v', resLine '%v'", statLine, resLine)
		}
	}
	log.Info("compare and verify statistics and result file succeess")
	var err error
	_, err = statFile.Seek(0, 0)
	if err != nil {
		log.Warn("seek statistic file failed", "err", err)
	}
	_, err = resFile.Seek(0, 0)
	if err != nil {
		log.Warn("seek result file failed", "err", err)
	}
}

func processLine(statLine, resLine string) {
	var (
		statAccount                       common.Address
		statReward, statShare, statNumber *big.Int

		resAccount common.Address
		resReward  *big.Int
		txhash     common.Hash
	)
	if statLine != "" {
		statAccount, statReward, statShare, statNumber = parseStatisticLine(statLine)
		accountStr := strings.ToLower(statAccount.String())
		log.Trace("read statistic line success", "account", accountStr, "reward", statReward, "share", statShare, "number", statNumber)
	}

	if resLine != "" {
		resAccount, resReward, txhash = parseResultLine(resLine)
		accountStr := strings.ToLower(resAccount.String())
		log.Trace("read result line success", "account", accountStr, "reward", "resReward", "txhash", txhash.String())
	}

	if statLine != "" && resLine != "" {
		if statAccount != resAccount || statReward.Cmp(resReward) != 0 {
			log.Fatalf("line mismatch. statLine %v, resLine %v", statLine, resLine)
		}
	}

	var account common.Address
	var reward, share, number *big.Int
	if statReward != nil {
		account = statAccount
		reward = statReward
		share = statShare
		number = statNumber
	} else {
		account = resAccount
		reward = resReward
	}
	addRewardResultToDB(account, reward, share, number, txhash)
}

func parseResultLine(line string) (account common.Address, reward *big.Int, txhash common.Hash) {
	parts := re.Split(line, -1)
	if len(parts) != 3 {
		log.Fatalf("wrong parts of result line: %v", line)
	}
	accountStr := parts[0]
	rewardStr := parts[1]
	txHashStr := parts[2]
	if !common.IsHexAddress(accountStr) {
		log.Fatalf("wrong address in result line: %v", line)
	}
	account = common.HexToAddress(accountStr)
	var ok bool
	reward, ok = new(big.Int).SetString(rewardStr, 10)
	if !ok {
		log.Fatalf("wrong reward in result line: %v", line)
	}
	txhash = common.HexToHash(txHashStr)
	if txhash.String() != txHashStr {
		log.Fatalf("wrong txhash in result line: %v", line)
	}
	return account, reward, txhash
}

func parseStatisticLine(line string) (account common.Address, reward, share, number *big.Int) {
	parts := re.Split(line, -1)
	if len(parts) != 4 {
		log.Fatalf("wrong parts of statistic line: %v", line)
	}
	accountStr := parts[0]
	rewardStr := parts[1]
	shareStr := parts[2]
	numberStr := parts[3]
	if !common.IsHexAddress(accountStr) {
		log.Fatalf("wrong address in statistic line: %v", line)
	}
	account = common.HexToAddress(accountStr)
	var ok bool
	reward, ok = new(big.Int).SetString(rewardStr, 10)
	if !ok {
		log.Fatalf("wrong reward in statistic line: %v", line)
	}
	share, ok = new(big.Int).SetString(shareStr, 10)
	if !ok {
		log.Fatalf("wrong share in statistic line: %v", line)
	}
	number, ok = new(big.Int).SetString(numberStr, 10)
	if !ok {
		log.Fatalf("wrong number in statistic line: %v", line)
	}
	return account, reward, share, number
}

func addRewardResultToDB(account common.Address, reward, share, number *big.Int, txhash common.Hash) {
	accoutStr := strings.ToLower(account.String())
	rewardStr := reward.String()
	pairs := params.GetExchangePairs(exchange)

	if dryRun {
		subject := fmt.Sprintf("[dryRun] add %v reward result", rewardType)
		log.Info(subject,
			"account", accoutStr, "reward", reward,
			"share", share, "number", number, "txhash", txhash.String(),
			"exchange", exchange, "pairs", pairs, "rewardToken", rewardToken,
			"startHeight", startHeight, "endHeight", endHeight,
		)
		return
	}

	var shareStr string
	if share != nil {
		shareStr = share.String()
	}
	var numVal uint64
	if number != nil {
		numVal = number.Uint64()
	}
	var hashStr string
	if txhash != (common.Hash{}) {
		hashStr = txhash.String()
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
		}
		_ = mongodb.TryDoTimes("AddLiquidRewardResult "+mr.Key, func() error {
			return mongodb.AddLiquidRewardResult(mr)
		})
	default:
		log.Fatalf("can only import liquid or volume rewards. wrong reward type %v", rewardType)
	}
}
