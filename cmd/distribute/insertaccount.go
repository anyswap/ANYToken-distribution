package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/anyswap/ANYToken-distribution/cmd/utils"
	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/mongodb"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
	"github.com/urfave/cli/v2"
)

var (
	insertAccountCommand = &cli.Command{
		Action:    insertAccount,
		Name:      "insertaccount",
		Usage:     "insert account from file",
		ArgsUsage: " ",
		Description: `
insert account from input file with line format: <address>
support two kind of account: exchange or token account.

if --exchange is specified then it's exchange account,
otherwise if --token is specified then it's token account.
`,
		Flags: []cli.Flag{
			mongoURLFlag,
			dbNameFlag,
			dbUserFlag,
			dbPassFlag,
			utils.ExchangeFlag,
			utils.PairsFlag,
			utils.TokenFlag,
			utils.InputFileFlag,
			utils.DryRunFlag,
		},
	}

	mongoURLFlag = &cli.StringFlag{
		Name:  "mongoURL",
		Usage: "mongodb URL",
		Value: "localhost:27017",
	}
	dbNameFlag = &cli.StringFlag{
		Name:  "dbName",
		Usage: "database name",
	}
	dbUserFlag = &cli.StringFlag{
		Name:  "dbUser",
		Usage: "database user name",
	}
	dbPassFlag = &cli.StringFlag{
		Name:  "dbPass",
		Usage: "database password",
	}
)

var (
	token string
	pairs string
)

func insertAccount(ctx *cli.Context) error {
	inputFileName = ctx.String(utils.InputFileFlag.Name)
	exchange = ctx.String(utils.ExchangeFlag.Name)
	token = ctx.String(utils.TokenFlag.Name)
	pairs = ctx.String(utils.PairsFlag.Name)
	dryRun = ctx.Bool(utils.DryRunFlag.Name)

	if inputFileName == "" {
		log.Fatal("must specify input file")
	}
	if exchange == "" && token == "" {
		log.Fatal("must specify exchange or token")
	}
	if exchange != "" && token != "" {
		log.Fatal("can not specify both exchange and token")
	}
	if exchange != "" && pairs == "" {
		log.Fatal("must specify pairs for exchange")
	}

	initMongodb(ctx)
	insertAccountFromFile()
	return nil
}

func initMongodb(ctx *cli.Context) {
	dbURL := ctx.String(mongoURLFlag.Name)
	dbName := ctx.String(dbNameFlag.Name)
	userName := ctx.String(dbUserFlag.Name)
	passwd := ctx.String(dbPassFlag.Name)
	if dbName == "" {
		log.Fatal("must specify database name")
	}
	mongodb.MongoServerInit([]string{dbURL}, dbName, userName, passwd)
}

func insertAccountFromFile() {
	file, err := os.Open(inputFileName)
	if err != nil {
		log.Fatalf("open '%v' failed. %v", inputFileName, err)
	}
	defer file.Close()
	log.Info("open accounts file success", "file", inputFileName)

	accountMap := make(map[common.Address]struct{})

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
			log.Warn("ignore wrong address line", "line", line)
			continue
		}
		account := common.HexToAddress(line)
		if _, exist := accountMap[account]; exist {
			log.Warn("ignore duplicate account", "account", account.String())
			continue
		}
		accountMap[account] = struct{}{}
		err = addAccountToDB(strings.ToLower(account.String()))
		if err != nil {
			log.Warn("add account error", "err", err)
		}
	}
}

func addAccountToDB(account string) error {
	switch {
	case exchange != "":
		if dryRun {
			fmt.Printf("insertAccount: %v %v %v", exchange, pairs, account)
			return nil
		}
		return mongodb.AddAccount(
			&mongodb.MgoAccount{
				Key:      mongodb.GetKeyOfExchangeAndAccount(exchange, account),
				Exchange: strings.ToLower(exchange),
				Pairs:    pairs,
				Account:  strings.ToLower(account),
			})
	case token != "":
		if dryRun {
			fmt.Printf("insertAccount: %v %v", token, account)
			return nil
		}
		return mongodb.AddTokenAccount(
			&mongodb.MgoTokenAccount{
				Key:     mongodb.GetKeyOfTokenAndAccount(token, account),
				Token:   strings.ToLower(token),
				Account: strings.ToLower(account),
			})
	default:
		return fmt.Errorf("no exchange or token is specified")
	}
}
