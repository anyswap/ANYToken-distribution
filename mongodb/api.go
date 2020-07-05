package mongodb

import (
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var (
	collectionBlock       *mgo.Collection
	collectionTransaction *mgo.Collection
	collectionSyncInfo    *mgo.Collection
	collectionTrades      *mgo.Collection
)

// do this when reconnect to the database
func deinintCollections() {
	collectionBlock = nil
	collectionTransaction = nil
	collectionSyncInfo = nil
	collectionTrades = nil
}

func initCollections() {
	_ = getCollection(tbSyncInfo).Insert(
		&MgoSyncInfo{
			Key: KeyOfLatestSyncInfo,
		},
	)
}

func getOrInitCollection(table string, collection **mgo.Collection, indexKey string) *mgo.Collection {
	if *collection == nil {
		*collection = database.C(table)
		if indexKey != "" {
			_ = (*collection).EnsureIndexKey(indexKey)
		}
	}
	return *collection
}

func getCollection(table string) *mgo.Collection {
	switch table {
	case tbBlocks:
		return getOrInitCollection(table, &collectionBlock, "number")
	case tbTransactions:
		return getOrInitCollection(table, &collectionTransaction, "blockNumber")
	case tbSyncInfo:
		return getOrInitCollection(table, &collectionSyncInfo, "")
	case tbTrades:
		return getOrInitCollection(table, &collectionTrades, "pairs")
	}
	panic("unknown talbe " + table)
}

// --------------- add ---------------------------------

// AddBlock add block
func AddBlock(mb *MgoBlock, overwrite bool) (err error) {
	if overwrite {
		_, err = getCollection(tbBlocks).UpsertId(mb.Key, mb)
	} else {
		err = getCollection(tbBlocks).Insert(mb)
	}
	if err == nil {
		_ = UpdateSyncInfo(mb.Number, mb.Hash, mb.Timestamp)
	}
	return err
}

// AddTransaction add tx
func AddTransaction(mt *MgoTransaction, overwrite bool) error {
	if overwrite {
		_, err := getCollection(tbTransactions).UpsertId(mt.Key, mt)
		return err
	}
	return getCollection(tbTransactions).Insert(mt)
}

// AddTrade add tx
func AddTrade(mt *MgoTrade, overwrite bool) error {
	if overwrite {
		_, err := getCollection(tbTrades).UpsertId(mt.Key, mt)
		return err
	}
	return getCollection(tbTrades).Insert(mt)
}

// --------------- update ---------------------------------

// UpdateSyncInfo update sync info
func UpdateSyncInfo(number uint64, hash string, timestamp uint64) error {
	return getCollection(tbSyncInfo).UpdateId(KeyOfLatestSyncInfo,
		bson.M{"$set": bson.M{
			"number":    number,
			"timestamp": timestamp,
			"hash":      hash,
		}})
}

// --------------- find ---------------------------------

// FindBlocksInRange find blocks
func FindBlocksInRange(start, end uint64) ([]*MgoBlock, error) {
	count := int(end - start + 1)
	blocks := make([]*MgoBlock, count)
	iter := getCollection(tbBlocks).Find(bson.M{"number": bson.M{"$gte": start, "$lte": end}}).Limit(count).Iter()
	err := iter.All(&blocks)
	if err != nil {
		return nil, err
	}
	return blocks, nil
}

// FindLatestSyncInfo find latest sync info
func FindLatestSyncInfo() (*MgoSyncInfo, error) {
	var info MgoSyncInfo
	err := getCollection(tbSyncInfo).FindId(KeyOfLatestSyncInfo).One(&info)
	if err != nil {
		return nil, err
	}
	return &info, nil
}
