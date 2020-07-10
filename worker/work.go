package worker

import (
	"fmt"
	"time"

	"github.com/anyswap/ANYToken-distribution/distribute"
	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/mongodb"
	"github.com/anyswap/ANYToken-distribution/params"
	"github.com/anyswap/ANYToken-distribution/syncer"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
)

// StartWork start all work
func StartWork() {
	for {
		err := dialServer()
		if err == nil {
			break
		}
		time.Sleep(3 * time.Second)
	}
	defer closeClient()

	if err := verifyConfig(); err != nil {
		panic(err)
	}

	initMongodb()
	go syncer.Start()

	go updateLiquidityDaily()

	go distribute.Start()

	exitCh := make(chan struct{})
	<-exitCh
}

func initMongodb() {
	config := params.GetConfig()
	dbConfig := config.MongoDB
	mongoURL := dbConfig.DBURL
	if dbConfig.UserName != "" || dbConfig.Password != "" {
		mongoURL = fmt.Sprintf("%s:%s@%s", dbConfig.UserName, dbConfig.Password, dbConfig.DBURL)
	}
	dbName := dbConfig.DBName
	mongodb.MongoServerInit(mongoURL, dbName)
}

func verifyConfig() error {
	config := params.GetConfig()
	for _, ex := range config.Exchanges {
		exchange := common.HexToAddress(ex.Exchange)
		token := common.HexToAddress(ex.Token)
		wantToken := getExchangeTokenAddress(exchange)
		if token != wantToken {
			return fmt.Errorf("exchange token mismatch. exchange %v want token %v, but have %v", ex.Exchange, wantToken.String(), ex.Token)
		}
		log.Info("verify exchange token success", "exchange", ex.Exchange, "token", ex.Token)
	}
	return nil
}
