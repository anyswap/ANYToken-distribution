package mongodb

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/tools"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
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
)

// do this when reconnect to the database
func deinintCollections() {
	collectionBlock = nil
	collectionTransaction = nil
	collectionSyncInfo = nil
	collectionLiquidity = nil
	collectionVolume = nil
	collectionAccount = nil
	collectionLiquidityBalance = nil
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
			_ = (*collection).EnsureIndexKey(indexKey...)
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
	case tbVolumeHistory:
		return getOrInitCollection(table, &collectionVolumeHistory, "exchange", "account", "blockNumber")
	case tbAccounts:
		return getOrInitCollection(table, &collectionAccount, "exchange")
	case tbLiquidityBalance:
		return getOrInitCollection(table, &collectionLiquidityBalance, "exchange", "account", "blockNumber")
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
	} else if !mgo.IsDup(err) {
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

// AddVolumeHistory add volume history
func AddVolumeHistory(mv *MgoVolumeHistory) error {
	err := getCollection(tbVolumeHistory).Insert(mv)
	switch {
	case err == nil:
		log.Info("[mongodb] AddVolumeHistory success", "volume", mv)
	case mgo.IsDup(err):
		return nil
	default:
		log.Info("[mongodb] AddVolumeHistory failed", "volume", mv, "err", err)
	}
	return err
}

// AddAccount add exchange account
func AddAccount(ma *MgoAccount) error {
	err := getCollection(tbAccounts).Insert(ma)
	switch {
	case err == nil:
		log.Info("[mongodb] AddAccount success", "account", ma)
	case mgo.IsDup(err):
		return nil
	default:
		log.Info("[mongodb] AddAccount failed", "account", ma, "err", err)
	}
	return err
}

// AddLiquidityBalance add liquidity balance
func AddLiquidityBalance(ma *MgoLiquidityBalance) error {
	err := getCollection(tbLiquidityBalance).Insert(ma)
	switch {
	case err == nil:
		log.Info("[mongodb] AddLiquidityBalance success", "balance", ma)
	case mgo.IsDup(err):
		return nil
	default:
		log.Info("[mongodb] AddLiquidityBalance failed", "balance", ma, "err", err)
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

// UpdateVolumeWithReceipt update volume
func UpdateVolumeWithReceipt(exr *ExchangeReceipt, blockHash string, blockNumber, timestamp uint64) error {
	key := GetKeyOfExchangeAndTimestamp(exr.Exchange, timestamp)
	curVol, err := FindVolume(key)

	if curVol == nil && err != mgo.ErrNotFound {
		return err
	}

	tokenFromAmount, _ := tools.GetBigIntFromString(exr.TokenFromAmount)
	tokenToAmount, _ := tools.GetBigIntFromString(exr.TokenToAmount)

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
		oldCoinVal, _ := tools.GetBigIntFromString(curVol.CoinVolume24h)
		oldTokenVal, _ := tools.GetBigIntFromString(curVol.TokenVolume24h)
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

// FindAllAccounts find accounts
func FindAllAccounts(exchange string) (accounts []common.Address) {
	iter := getCollection(tbAccounts).Find(bson.M{"exchange": strings.ToLower(exchange)}).Iter()
	var result MgoAccount
	for iter.Next(&result) {
		accounts = append(accounts, common.HexToAddress(result.Account))
	}
	return accounts
}

// FindLiquidityBalance find liquidity balance
func FindLiquidityBalance(exchange, account string, blockNumber uint64) (string, error) {
	var res MgoLiquidityBalance
	key := GetKeyOfLiquidityBalance(exchange, account, blockNumber)
	err := getCollection(tbLiquidityBalance).FindId(key).One(&res)
	if err != nil {
		return "0", err
	}
	return res.Liquidity, nil
}

// FindAccountVolumes find account volumes
func FindAccountVolumes(exchange string, startHeight, endHeight uint64) (accounts []common.Address, volumes []*big.Int) {
	qexchange := bson.M{"exchange": strings.ToLower(exchange)}
	qsheight := bson.M{"blockNumber": bson.M{"$gte": startHeight}}
	qeheight := bson.M{"blockNumber": bson.M{"$lt": endHeight}}
	queries := []bson.M{qexchange, qsheight, qeheight}
	iter := getCollection(tbVolumeHistory).Find(bson.M{"$and": queries}).Iter()
	var (
		accountVolumesMap = make(map[common.Address]*big.Int)
		account           common.Address
		volume            *big.Int
		result            MgoVolumeHistory
	)
	for iter.Next(&result) {
		log.Info("find volume record", "account", result.Account, "coinAmount", result.CoinAmount, "tokenAmount", result.TokenAmount, "blockNumber", result.BlockNumber, "logIndex", result.LogIndex)
		volume, _ = tools.GetBigIntFromString(result.CoinAmount)
		if volume == nil || volume.Sign() <= 0 {
			continue
		}
		account = common.HexToAddress(result.Account)
		old, exist := accountVolumesMap[account]
		if exist {
			accountVolumesMap[account] = old.Add(old, volume)
		} else {
			accountVolumesMap[account] = volume
		}
	}
	for acc, vol := range accountVolumesMap {
		accounts = append(accounts, acc)
		volumes = append(volumes, vol)
		log.Info("find volume result", "account", acc.String(), "volume", vol)
	}
	return accounts, volumes
}
