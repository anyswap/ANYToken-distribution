package mongodb

import (
	"gopkg.in/mgo.v2"
)

var (
	collectionBlock            *mgo.Collection
	collectionTransaction      *mgo.Collection
	collectionSyncInfo         *mgo.Collection
	collectionLiquidity        *mgo.Collection
	collectionVolume           *mgo.Collection
	collectionVolumeHistory    *mgo.Collection
	collectionAccount          *mgo.Collection
	collectionLiquidityBalance *mgo.Collection
	collectionDistributeInfo   *mgo.Collection
)

// do this when reconnect to the database
func deinintCollections() {
	collectionBlock = database.C(tbBlocks)
	collectionTransaction = database.C(tbTransactions)
	collectionSyncInfo = database.C(tbSyncInfo)
	collectionLiquidity = database.C(tbLiquidity)
	collectionVolume = database.C(tbVolume)
	collectionVolumeHistory = database.C(tbVolumeHistory)
	collectionAccount = database.C(tbAccounts)
	collectionLiquidityBalance = database.C(tbLiquidityBalance)
	collectionDistributeInfo = database.C(tbDistributeInfo)
}

func initCollections() {
	initCollection(tbBlocks, &collectionBlock, "number")
	initCollection(tbTransactions, &collectionTransaction, "blockNumber")
	initCollection(tbSyncInfo, &collectionSyncInfo)
	initCollection(tbLiquidity, &collectionLiquidity, "exchange", "timestamp")
	initCollection(tbVolume, &collectionVolume, "exchange", "timestamp")
	initCollection(tbVolumeHistory, &collectionVolumeHistory, "exchange", "account", "blockNumber")
	initCollection(tbAccounts, &collectionAccount, "exchange")
	initCollection(tbLiquidityBalance, &collectionLiquidityBalance, "exchange", "account", "blockNumber")
	initCollection(tbDistributeInfo, &collectionDistributeInfo, "exchange", "bywhat")

	_ = initLatestSyncInfo()
}

func initCollection(table string, collection **mgo.Collection, indexKey ...string) {
	*collection = database.C(table)
	if len(indexKey) != 0 && indexKey[0] != "" {
		_ = (*collection).EnsureIndexKey(indexKey...)
	}
}

func initLatestSyncInfo() error {
	_, err := FindLatestSyncInfo()
	if err == nil {
		return nil
	}
	return collectionSyncInfo.Insert(
		&MgoSyncInfo{
			Key: KeyOfLatestSyncInfo,
		},
	)
}
