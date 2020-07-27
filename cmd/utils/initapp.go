package utils

import (
	"fmt"
	"time"

	"github.com/anyswap/ANYToken-distribution/callapi"
	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/mongodb"
	"github.com/anyswap/ANYToken-distribution/params"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
	"github.com/urfave/cli/v2"
)

// InitApp init app (remember close client in the caller)
func InitApp(ctx *cli.Context, withMongodb bool) *callapi.APICaller {
	return initApp(ctx, withMongodb, "")
}

// InitAppWithURL init app for library use (remember close client in the caller)
func InitAppWithURL(ctx *cli.Context, withMongodb bool, serverURL string) *callapi.APICaller {
	return initApp(ctx, withMongodb, serverURL)
}

func initApp(ctx *cli.Context, withMongodb bool, serverURL string) *callapi.APICaller {
	SetLogger(ctx)
	InitSyncArguments(ctx)

	configFile := GetConfigFilePath(ctx)
	params.LoadConfig(configFile)

	if withMongodb {
		InitMongodb()
	}

	if serverURL == "" {
		serverURL = params.GetConfig().Gateway.APIAddress
	}

	capi := DialServer(serverURL)

	if err := verifyConfig(capi); err != nil {
		log.Fatalf("verifyConfig error. %v", err)
	}

	return capi
}

// DialServer connect to serverURL
func DialServer(serverURL string) *callapi.APICaller {
	capi := callapi.NewDefaultAPICaller()
	for {
		err := capi.DialServer(serverURL)
		if err == nil {
			break
		}
		time.Sleep(3 * time.Second)
	}
	return capi
}

// InitMongodb init mongodb by config
func InitMongodb() {
	config := params.GetConfig()
	dbConfig := config.MongoDB
	mongodb.MongoServerInit([]string{dbConfig.DBURL}, dbConfig.DBName, dbConfig.UserName, dbConfig.Password)
}

func verifyConfig(capi *callapi.APICaller) error {
	config := params.GetConfig()
	for _, ex := range config.Exchanges {
		exchange := common.HexToAddress(ex.Exchange)
		token := common.HexToAddress(ex.Token)
		wantToken := capi.GetExchangeTokenAddress(exchange)
		if token != wantToken {
			return fmt.Errorf("exchange token mismatch. exchange %v want token %v, but have %v", ex.Exchange, wantToken.String(), ex.Token)
		}
		log.Info("verify exchange token success", "exchange", ex.Exchange, "token", ex.Token)
	}
	return nil
}
