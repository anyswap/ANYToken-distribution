package syncer

import (
	"math/big"
	"time"

	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/mongodb"
	"github.com/anyswap/ANYToken-distribution/params"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
	"github.com/fsn-dev/fsn-go-sdk/efsn/core/types"
)

var (
	topicTokenPurchase   = common.HexToHash("0xcd60aa75dea3072fbc07ae6d7d856b5dc5f4eee88854f5b4abf7b680ef8bc50f")
	topicEthPurchase     = common.HexToHash("0x7f4091b46c33e918a0f3aa42307641d17bb67029427a5369e54b353984238705")
	topicAddLiquidity    = common.HexToHash("0x06239653922ac7bea6aa2b19dc486b9361821d37712eb796adfd38d81de278ca")
	topicRemoveLiquidity = common.HexToHash("0x0fbf06c058b90cb038a618f8c2acbf6145f8b3570fd1fa56abb8f0f3f05b36e8")
	topicTransfer        = common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")
)

const secondsPerDay = 24 * 3600

func getDayBegin(timestamp uint64) uint64 {
	return timestamp - timestamp%secondsPerDay
}

func timestampToDate(timestamp uint64) string {
	return time.Unix(int64(timestamp), 0).Format("2006-01-02 15:04:05")
}

func parseReceipt(mt *mongodb.MgoTransaction, receipt *types.Receipt) {
	if receipt == nil {
		return
	}
	// only process configed exchange contract
	if params.GetExchangePairs(mt.To) == "" {
		return
	}
	for idx, rlog := range receipt.Logs {
		if len(rlog.Topics) == 0 {
			continue
		}

		// only process configed exchange contract
		if params.GetExchangePairs(rlog.Address.String()) == "" {
			continue
		}

		exReceipt := &mongodb.ExchangeReceipt{}
		updateVolume := false

		switch rlog.Topics[0] {
		case topicAddLiquidity:
			parseAddLiquidity(exReceipt, rlog)
		case topicRemoveLiquidity:
			parseRemoveLiquidity(exReceipt, rlog)
		case topicTokenPurchase:
			updateVolume = true
			parseTokenPurchase(exReceipt, rlog)
		case topicEthPurchase:
			updateVolume = true
			parseEthPurchase(exReceipt, rlog)
		case topicTransfer:
			parseTransfer(rlog)
		}

		if exReceipt.LogType != "" {
			exReceipt.Exchange = rlog.Address.String()
			exReceipt.Pairs = params.GetExchangePairs(exReceipt.Exchange)
			exReceipt.LogIndex = idx
			mt.ExchangeReceipts = append(mt.ExchangeReceipts, exReceipt)
			log.Info("[parse] add exchange tx receipt", "receipt", exReceipt)
		}

		if updateVolume {
			timestamp := getDayBegin(mt.Timestamp)
			log.Info("[parse] update volume", "txHash", mt.Hash, "logIndex", idx, "logType", exReceipt.LogType, "exchange", exReceipt.Exchange, "pairs", exReceipt.Pairs, "timestamp", timestampToDate(mt.Timestamp))
			tryDoTimes("[parse] UpdateVolume "+mt.Hash, func() error {
				return mongodb.UpdateVolumeWithReceipt(exReceipt, mt.BlockHash, mt.BlockNumber, timestamp)
			})
		}
	}
}

func parseAddLiquidity(mt *mongodb.ExchangeReceipt, rlog *types.Log) {
	topics := rlog.Topics
	provider := common.BytesToAddress(topics[1].Bytes())
	ethAmount := new(big.Int).SetBytes(topics[2].Bytes())
	tokenAmount := new(big.Int).SetBytes(topics[3].Bytes())

	mt.LogType = "AddLiquidity"
	mt.Address = provider.String()
	mt.TokenFromAmount = ethAmount.String()
	mt.TokenToAmount = tokenAmount.String()
}

func parseRemoveLiquidity(mt *mongodb.ExchangeReceipt, rlog *types.Log) {
	topics := rlog.Topics
	provider := common.BytesToAddress(topics[1].Bytes())
	ethAmount := new(big.Int).SetBytes(topics[2].Bytes())
	tokenAmount := new(big.Int).SetBytes(topics[3].Bytes())

	mt.LogType = "RemoveLiquidity"
	mt.Address = provider.String()
	mt.TokenFromAmount = ethAmount.String()
	mt.TokenToAmount = tokenAmount.String()
}

func parseTokenPurchase(mt *mongodb.ExchangeReceipt, rlog *types.Log) {
	topics := rlog.Topics
	buyer := common.BytesToAddress(topics[1].Bytes())
	ethSold := new(big.Int).SetBytes(topics[2].Bytes())
	tokensBought := new(big.Int).SetBytes(topics[3].Bytes())

	mt.LogType = "TokenPurchase"
	mt.Address = buyer.String()
	mt.TokenFromAmount = ethSold.String()
	mt.TokenToAmount = tokensBought.String()
}

func parseEthPurchase(mt *mongodb.ExchangeReceipt, rlog *types.Log) {
	topics := rlog.Topics
	buyer := common.BytesToAddress(topics[1].Bytes())
	tokensSold := new(big.Int).SetBytes(topics[2].Bytes())
	ethBought := new(big.Int).SetBytes(topics[3].Bytes())

	mt.LogType = "EthPurchase"
	mt.Address = buyer.String()
	mt.TokenFromAmount = tokensSold.String()
	mt.TokenToAmount = ethBought.String()
}

func parseTransfer(rlog *types.Log) {
	contract := rlog.Address
	topics := rlog.Topics
	from := common.BytesToAddress(topics[1].Bytes())
	to := common.BytesToAddress(topics[2].Bytes())
	value := new(big.Int).SetBytes(rlog.Data)

	updateLiquidity(contract, from, to, value)
}

func updateLiquidity(exchange, from, to common.Address, value *big.Int) {
	log.Info("[parse] updateLiquidity", "exchange", exchange.String(), "from", from.String(), "to", to.String(), "value", value)
}
