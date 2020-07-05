package worker

import (
	"fmt"

	"github.com/anyswap/ANYToken-distribution/distribute"
	"github.com/anyswap/ANYToken-distribution/mongodb"
	"github.com/anyswap/ANYToken-distribution/params"
	"github.com/anyswap/ANYToken-distribution/syncer"
)

// StartWork start all work
func StartWork() {
	initMongodb()

	go syncer.Start()
	go distribute.Start()
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
