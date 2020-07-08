package mongodb

import (
	"github.com/anyswap/ANYToken-distribution/log"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var (
	collectionBlock       *mgo.Collection
	collectionTransaction *mgo.Collection
	collectionSyncInfo    *mgo.Collection
	collectionLiquidity   *mgo.Collection
	collectionVolume      *mgo.Collection
)

// do this when reconnect to the database
func deinintCollections() {
	collectionBlock = nil
	collectionTransaction = nil
	collectionSyncInfo = nil
	collectionLiquidity = nil
	collectionVolume = nil
}

func initCollections() {
	_ = getCollection(tbSyncInfo).Insert(
		&MgoSyncInfo{
			Key: KeyOfLatestSyncInfo,
		},
	)
}

func getOrInitCollection(table string, collection **mgo.Collection, indexKey ...string) *mgo.Collection {
	if *collection == nil {
		*collection = database.C(table)
		if len(indexKey) != 0 && indexKey[0] != "" {
			err := (*collection).EnsureIndexKey(indexKey...)
			if err != nil {
				log.Error("EnsureIndexKey error", "table", table, "indexKey", indexKey)
			}
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
	case tbLiquidity:
		return getOrInitCollection(table, &collectionLiquidity, "exchange", "blockNumber")
	case tbVolume:
		return getOrInitCollection(table, &collectionVolume, "exchange", "blockNumber")
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

// AddLiquidity add liquidity
func AddLiquidity(ml *MgoLiquidity, overwrite bool) (err error) {
	if overwrite {
		_, err = getCollection(tbLiquidity).UpsertId(ml.Key, ml)
	} else {
		err = getCollection(tbLiquidity).Insert(ml)
	}
	if err == nil {
		log.Info("AddLiquidity success", "liquidity", ml)
	} else {
		log.Info("AddLiquidity failed", "liquidity", ml, "err", err)
	}
	return err
}

// AddVolume add volume
func AddVolume(mv *MgoVolume, overwrite bool) (err error) {
	if overwrite {
		_, err = getCollection(tbVolume).UpsertId(mv.Key, mv)
	} else {
		err = getCollection(tbVolume).Insert(mv)
	}
	if err == nil {
		log.Info("AddVolume success", "volume", mv)
	} else {
		log.Info("AddVolume failed", "volume", mv, "err", err)
	}
	return err
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
