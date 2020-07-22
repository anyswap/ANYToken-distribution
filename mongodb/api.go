package mongodb

import (
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/tools"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

const (
	retryDBCount    = 3
	retryDBInterval = 1 * time.Second
)

// TryDoTimes try do again if meet error
func TryDoTimes(name string, f func() error) (err error) {
	for i := 0; i < retryDBCount; i++ {
		err = f()
		if err == nil || mgo.IsDup(err) {
			return nil
		}
		time.Sleep(retryDBInterval)
	}
	log.Warn("[mongodb] TryDoTimes", "name", name, "times", retryDBCount, "err", err)
	return err
}

// --------------- add ---------------------------------

// AddBlock add block
func AddBlock(mb *MgoBlock, overwrite bool) (err error) {
	if overwrite {
		_, err = collectionBlock.UpsertId(mb.Key, mb)
	} else {
		err = collectionBlock.Insert(mb)
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
		_, err := collectionTransaction.UpsertId(mt.Key, mt)
		return err
	}
	return collectionTransaction.Insert(mt)
}

// AddLiquidity add liquidity
func AddLiquidity(ml *MgoLiquidity, overwrite bool) (err error) {
	if overwrite {
		_, err = collectionLiquidity.UpsertId(ml.Key, ml)
	} else {
		err = collectionLiquidity.Insert(ml)
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
		_, err = collectionVolume.UpsertId(mv.Key, mv)
	} else {
		err = collectionVolume.Insert(mv)
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
	err := collectionVolumeHistory.Insert(mv)
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
	err := collectionAccount.Insert(ma)
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
	err := collectionLiquidityBalance.Insert(ma)
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

// AddDistributeInfo add distributeInfo
func AddDistributeInfo(ma *MgoDistributeInfo) error {
	ma.Key = bson.NewObjectId()
	err := collectionDistributeInfo.Insert(ma)
	switch {
	case err == nil:
		log.Info("[mongodb] AddDistributeInfo success", "distribute", ma)
	default:
		log.Info("[mongodb] AddDistributeInfo failed", "distribute", ma, "err", err)
	}
	return err
}

// --------------- update ---------------------------------

// UpdateSyncInfo update sync info
func UpdateSyncInfo(number uint64, hash string, timestamp uint64) error {
	return collectionSyncInfo.UpdateId(KeyOfLatestSyncInfo,
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
	iter := collectionBlock.Find(bson.M{"number": bson.M{"$gte": start, "$lte": end}}).Limit(count).Iter()
	err := iter.All(&blocks)
	if err != nil {
		return nil, err
	}
	return blocks, nil
}

// FindLatestSyncInfo find latest sync info
func FindLatestSyncInfo() (*MgoSyncInfo, error) {
	var info MgoSyncInfo
	err := collectionSyncInfo.FindId(KeyOfLatestSyncInfo).One(&info)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

// FindLatestLiquidity find latest liquidity
func FindLatestLiquidity(exchange string) (*MgoLiquidity, error) {
	var res MgoLiquidity
	err := collectionLiquidity.Find(bson.M{"exchange": exchange}).Sort("-timestamp").Limit(1).One(&res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// FindLiquidity find by key
func FindLiquidity(key string) (*MgoLiquidity, error) {
	var res MgoLiquidity
	err := collectionLiquidity.FindId(key).One(&res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// FindLatestVolume find latest volume
func FindLatestVolume(exchange string) (*MgoVolume, error) {
	var res MgoVolume
	err := collectionVolume.Find(bson.M{"exchange": exchange}).Sort("-timestamp").Limit(1).One(&res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// FindVolume find by key
func FindVolume(key string) (*MgoVolume, error) {
	var res MgoVolume
	err := collectionVolume.FindId(key).One(&res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// FindAllAccounts find accounts
func FindAllAccounts(exchange string) (accounts []common.Address) {
	iter := collectionAccount.Find(bson.M{"exchange": strings.ToLower(exchange)}).Iter()
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
	err := collectionLiquidityBalance.FindId(key).One(&res)
	if err != nil {
		return "0", err
	}
	return res.Liquidity, nil
}

// FindAccountVolumes find account volumes
func FindAccountVolumes(exchange string, startHeight, endHeight uint64) (accounts []common.Address, volumes []*big.Int, txcounts []uint64) {
	qexchange := bson.M{"exchange": strings.ToLower(exchange)}
	qsheight := bson.M{"blockNumber": bson.M{"$gte": startHeight}}
	qeheight := bson.M{"blockNumber": bson.M{"$lt": endHeight}}
	queries := []bson.M{qexchange, qsheight, qeheight}
	iter := collectionVolumeHistory.Find(bson.M{"$and": queries}).Iter()
	var (
		accountVolumesMap  = make(map[common.Address]*big.Int)
		accountTxsCountMap = make(map[common.Address]uint64)
		account            common.Address
		volume             *big.Int
		result             MgoVolumeHistory
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
			accountVolumesMap[account].Add(old, volume)
			accountTxsCountMap[account]++
		} else {
			accountVolumesMap[account] = volume
			accountTxsCountMap[account] = 1
		}
	}
	for acc, vol := range accountVolumesMap {
		accounts = append(accounts, acc)
		volumes = append(volumes, vol)
		txcount := accountTxsCountMap[acc]
		txcounts = append(txcounts, txcount)
		log.Info("find volume result", "account", acc.String(), "volume", vol, "txcount", txcount)
	}
	return accounts, volumes, txcounts
}
