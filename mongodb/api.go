package mongodb

import (
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/params"
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
		log.Warn("[mongodb] AddLiquidity failed", "liquidity", ml, "err", err)
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
		log.Debug("[mongodb] AddVolume success", "volume", mv)
	} else {
		log.Warn("[mongodb] AddVolume failed", "volume", mv, "err", err)
	}
	return err
}

// AddVolumeHistory add volume history
func AddVolumeHistory(mv *MgoVolumeHistory, overwrite bool) (err error) {
	if overwrite {
		_, err = collectionVolumeHistory.UpsertId(mv.Key, mv)
	} else {
		err = collectionVolumeHistory.Insert(mv)
	}
	switch {
	case err == nil:
		log.Info("[mongodb] AddVolumeHistory success", "volume", mv)
	case mgo.IsDup(err):
		return nil
	default:
		log.Warn("[mongodb] AddVolumeHistory failed", "volume", mv, "err", err)
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
		log.Warn("[mongodb] AddAccount failed", "account", ma, "err", err)
	}
	return err
}

// AddTokenAccount add token account
func AddTokenAccount(ma *MgoTokenAccount) error {
	err := collectionTokenAccount.Insert(ma)
	switch {
	case err == nil:
		log.Info("[mongodb] AddTokenAccount success", "account", ma)
	case mgo.IsDup(err):
		return nil
	default:
		log.Warn("[mongodb] AddTokenAccount failed", "account", ma, "err", err)
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
		log.Warn("[mongodb] AddLiquidityBalance failed", "balance", ma, "err", err)
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
		log.Warn("[mongodb] AddDistributeInfo failed", "distribute", ma, "err", err)
	}
	return err
}

func getVolumeRewardUpdateItems(mr *MgoVolumeRewardResult) bson.M {
	updates := bson.M{}
	if mr.Reward != "" {
		updates["reward"] = mr.Reward
	}
	if mr.Volume != "" {
		updates["volume"] = mr.Volume
	}
	if mr.TxCount != 0 {
		updates["txcount"] = mr.TxCount
	}
	if mr.RewardTx != "" {
		updates["rewardTx"] = mr.RewardTx
	}
	return updates
}

// AddVolumeRewardResult add volume reward result
func AddVolumeRewardResult(mr *MgoVolumeRewardResult) (err error) {
	old, _ := FindVolumeRewardResult(mr.Key)
	if old == nil {
		err = collectionVolumeRewardResult.Insert(mr)
	} else {
		updates := getVolumeRewardUpdateItems(mr)
		err = collectionVolumeRewardResult.UpdateId(mr.Key, bson.M{"$set": updates})
	}
	switch {
	case err == nil:
		log.Info("[mongodb] AddVolumeRewardResult success", "reward", mr, "isUpdate", old != nil)
	default:
		log.Warn("[mongodb] AddVolumeRewardResult failed", "reward", mr, "isUpdate", old != nil, "err", err)
	}
	return err
}

func getLiquidRewardUpdateItems(mr *MgoLiquidRewardResult) bson.M {
	updates := bson.M{}
	if mr.Reward != "" {
		updates["reward"] = mr.Reward
	}
	if mr.Liquidity != "" {
		updates["liquidity"] = mr.Liquidity
	}
	if mr.Height != 0 {
		updates["height"] = mr.Height
	}
	if mr.RewardTx != "" {
		updates["rewardTx"] = mr.RewardTx
	}
	return updates
}

// AddLiquidRewardResult add volume reward result
func AddLiquidRewardResult(mr *MgoLiquidRewardResult) (err error) {
	old, _ := FindLiquidRewardResult(mr.Key)
	if old == nil {
		err = collectionLiquidRewardResult.Insert(mr)
	} else {
		updates := getLiquidRewardUpdateItems(mr)
		err = collectionLiquidRewardResult.UpdateId(mr.Key, bson.M{"$set": updates})
	}
	switch {
	case err == nil:
		log.Info("[mongodb] AddLiquidRewardResult success", "reward", mr, "isUpdate", old != nil)
	default:
		log.Warn("[mongodb] AddLiquidRewardResult failed", "reward", mr, "isUpdate", old != nil, "err", err)
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
		log.Debug("[mongodb] update volume", "pairs", exr.Pairs, "logType", exr.LogType, "oldCoins", oldCoinVal, "newCoins", coinVal, "oldTokens", oldTokenVal, "newTokens", tokenVal)
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

// FindVolumeRewardResult find volume reward result
func FindVolumeRewardResult(key string) (*MgoVolumeRewardResult, error) {
	var res MgoVolumeRewardResult
	err := collectionVolumeRewardResult.FindId(key).One(&res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// FindLiquidRewardResult find liquid reward result
func FindLiquidRewardResult(key string) (*MgoLiquidRewardResult, error) {
	var res MgoLiquidRewardResult
	err := collectionLiquidRewardResult.FindId(key).One(&res)
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
		account := common.HexToAddress(result.Account)
		accounts = append(accounts, account)
	}
	return accounts
}

// FindAllTokenAccounts find accounts
func FindAllTokenAccounts(token string) (accounts []common.Address) {
	iter := collectionTokenAccount.Find(bson.M{"token": strings.ToLower(token)}).Iter()
	var result MgoTokenAccount
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
func FindAccountVolumes(exchange string, startHeight, endHeight uint64, useTimestamp bool) AccountStatSlice {
	var queries []bson.M
	qexchange := bson.M{"exchange": strings.ToLower(exchange)}
	queries = append(queries, qexchange)
	if useTimestamp {
		qstime := bson.M{"timestamp": bson.M{"$gte": startHeight}}
		qetime := bson.M{"timestamp": bson.M{"$lt": endHeight}}
		queries = append(queries, qstime, qetime)
	} else {
		qsheight := bson.M{"blockNumber": bson.M{"$gte": startHeight}}
		qeheight := bson.M{"blockNumber": bson.M{"$lt": endHeight}}
		queries = append(queries, qsheight, qeheight)
	}

	iter := collectionVolumeHistory.Find(bson.M{"$and": queries}).Iter()

	statMap := make(map[common.Address]*AccountStat)

	var mh MgoVolumeHistory
	for iter.Next(&mh) {
		log.Info("find volume record", "account", mh.Account, "coinAmount", mh.CoinAmount, "tokenAmount", mh.TokenAmount, "blockNumber", mh.BlockNumber, "logIndex", mh.LogIndex)
		volume, _ := tools.GetBigIntFromString(mh.CoinAmount)
		if volume == nil || volume.Sign() <= 0 {
			continue
		}
		account := common.HexToAddress(mh.Account)
		if params.IsExcludedRewardAccount(account) {
			continue
		}
		stat, exist := statMap[account]
		if exist {
			stat.Share.Add(stat.Share, volume)
			stat.Number++
		} else {
			statMap[account] = &AccountStat{
				Account: account,
				Share:   volume,
				Number:  1,
			}
		}
	}
	result := ConvertToSortedSlice(statMap)
	for _, stat := range result {
		log.Info("find volume result", "account", stat.Account.String(), "volume", stat.Share, "txcount", stat.Number, "start", startHeight, "end", endHeight)
	}
	return result
}
