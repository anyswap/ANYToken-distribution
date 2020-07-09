package mongodb

import (
	"fmt"
	"math/big"

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
				log.Error("[mongodb] EnsureIndexKey error", "table", table, "indexKey", indexKey, "err", err)
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
		return getOrInitCollection(table, &collectionLiquidity, "exchange", "timestamp")
	case tbVolume:
		return getOrInitCollection(table, &collectionVolume, "exchange", "timestamp")
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
		log.Info("[mongodb] AddBlock success", "number", mb.Number, "hash", mb.Hash)
		_ = UpdateSyncInfo(mb.Number, mb.Hash, mb.Timestamp)
	} else {
		log.Warn("[mongodb] AddBlock failed", "number", mb.Number, "hash", mb.Hash, "err", err)
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
		log.Info("[mongodb] AddLiquidity success", "liquidity", ml)
	} else {
		log.Info("[mongodb] AddLiquidity failed", "liquidity", ml, "err", err)
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
		log.Info("[mongodb] AddVolume success", "volume", mv)
	} else {
		log.Info("[mongodb] AddVolume failed", "volume", mv, "err", err)
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

func getBigIntFromString(str string) *big.Int {
	bi, _ := new(big.Int).SetString(str, 0)
	return bi
}

// UpdateVolumeWithReceipt update volume
func UpdateVolumeWithReceipt(exr *ExchangeReceipt, blockHash string, blockNumber, timestamp uint64) error {
	key := GetKeyOfExchangeAndTimestamp(exr.Exchange, timestamp)
	curVol, err := FindVolume(key)

	if curVol == nil && err != mgo.ErrNotFound {
		return err
	}

	tokenFromAmount := getBigIntFromString(exr.TokenFromAmount)
	tokenToAmount := getBigIntFromString(exr.TokenToAmount)

	var coinVal, tokenVal *big.Int

	switch {
	case exr.LogType == "TokenPurchase":
		coinVal = tokenFromAmount
		tokenVal = tokenToAmount
	case exr.LogType == "EthPurchase":
		tokenVal = tokenFromAmount
		coinVal = tokenToAmount
	default:
		return fmt.Errorf("[mongodb] update volume with wrong log type %v", exr.LogType)
	}

	if curVol != nil {
		oldCoinVal := getBigIntFromString(curVol.CoinVolume24h)
		oldTokenVal := getBigIntFromString(curVol.TokenVolume24h)
		coinVal.Add(coinVal, oldCoinVal)
		tokenVal.Add(tokenVal, oldTokenVal)
		log.Info("[mongodb] update volume", "oldCoins", oldCoinVal, "newCoins", coinVal, "oldTokens", oldTokenVal, "newTokens", tokenVal)
	}

	return AddVolume(&MgoVolume{
		Key:            key,
		Exchange:       exr.Exchange,
		Pairs:          exr.Pairs,
		CoinVolume24h:  coinVal.String(),
		TokenVolume24h: tokenVal.String(),
		BlockNumber:    blockNumber,
		BlockHash:      blockHash,
		Timestamp:      timestamp,
	}, true)
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

// FindLatestLiquidity find latest liquidity
func FindLatestLiquidity(exchange string) (*MgoLiquidity, error) {
	var res MgoLiquidity
	err := getCollection(tbLiquidity).Find(bson.M{"exchange": exchange}).Sort("-timestamp").Limit(1).One(&res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// FindLiquidity find by key
func FindLiquidity(key string) (*MgoLiquidity, error) {
	var res MgoLiquidity
	err := getCollection(tbLiquidity).FindId(key).One(&res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// FindLatestVolume find latest volume
func FindLatestVolume(exchange string) (*MgoVolume, error) {
	var res MgoVolume
	err := getCollection(tbVolume).Find(bson.M{"exchange": exchange}).Sort("-timestamp").Limit(1).One(&res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// FindVolume find by key
func FindVolume(key string) (*MgoVolume, error) {
	var res MgoVolume
	err := getCollection(tbVolume).FindId(key).One(&res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}
