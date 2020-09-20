package syncer

import (
	"math/big"
	"strings"
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
	topicApproval        = common.HexToHash("0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925")
)

const secondsPerDay = 24 * 3600

func getDayBegin(timestamp uint64) uint64 {
	return timestamp - timestamp%secondsPerDay
}

func timestampToDate(timestamp uint64) string {
	return time.Unix(int64(timestamp), 0).Format("2006-01-02 15:04:05")
}

func parseReceipt(mt *mongodb.MgoTransaction, receipt *types.Receipt) (savedb bool) {
	if receipt == nil || receipt.Status == 0 {
		return false
	}
	var save bool
	for idx, rlog := range receipt.Logs {
		if len(rlog.Topics) == 0 {
			continue
		}

		if rlog.Removed {
			continue
		}

		switch rlog.Topics[0] {
		case topicAddLiquidity:
			save = addExchangeReceipt(mt, rlog, idx, "AddLiquidity")
		case topicRemoveLiquidity:
			save = addExchangeReceipt(mt, rlog, idx, "RemoveLiquidity")
		case topicTokenPurchase:
			save = addExchangeReceipt(mt, rlog, idx, "TokenPurchase")
		case topicEthPurchase:
			save = addExchangeReceipt(mt, rlog, idx, "EthPurchase")
		case topicTransfer:
			save = addErc20Receipt(mt, rlog, idx, "Transfer")
		case topicApproval:
			save = addErc20Receipt(mt, rlog, idx, "Approval")
		}
		if save {
			savedb = true
		}
	}
	return savedb
}

func addExchangeReceipt(mt *mongodb.MgoTransaction, rlog *types.Log, logIdx int, logType string) bool {
	exchange := strings.ToLower(rlog.Address.String())
	if !params.IsScanAllExchange() && !params.IsConfigedExchange(exchange) {
		return false
	}
	topics := rlog.Topics
	address := common.BytesToAddress(topics[1].Bytes())
	fromAmount := new(big.Int).SetBytes(topics[2].Bytes())
	toAmount := new(big.Int).SetBytes(topics[3].Bytes())

	exReceipt := &mongodb.ExchangeReceipt{
		LogType:         logType,
		LogIndex:        logIdx,
		Exchange:        exchange,
		Pairs:           params.GetExchangePairs(exchange),
		Address:         strings.ToLower(address.String()),
		TokenFromAmount: fromAmount.String(),
		TokenToAmount:   toAmount.String(),
	}

	switch rlog.Topics[0] {
	case topicAddLiquidity:
		log.Info("[parse] add liquidity", "exchange", exReceipt.Exchange, "pairs", exReceipt.Pairs, "address", exReceipt.Address, "fromAmount", exReceipt.TokenFromAmount, "toAmount", exReceipt.TokenToAmount)
	case topicRemoveLiquidity:
		log.Info("[parse] remove liquidity", "exchange", exReceipt.Exchange, "pairs", exReceipt.Pairs, "address", exReceipt.Address, "fromAmount", exReceipt.TokenFromAmount, "toAmount", exReceipt.TokenToAmount)
	case topicTokenPurchase:
		recordTokenAccounts(params.GetExchangeToken(exchange), exReceipt.Address)
	}

	mt.ExchangeReceipts = append(mt.ExchangeReceipts, exReceipt)
	log.Debug("addExchangeReceipt", "receipt", exReceipt)

	recordAccounts(exchange, exReceipt.Pairs, address.String())
	recordAccountVoumes(mt, exReceipt, topics[0])

	updateVolumes(mt, exReceipt, topics[0])
	return true
}

func addErc20Receipt(mt *mongodb.MgoTransaction, rlog *types.Log, logIdx int, logType string) bool {
	erc20Address := strings.ToLower(rlog.Address.String())
	if !(params.IsConfigedToken(erc20Address) || params.IsConfigedExchange(erc20Address)) {
		return false
	}
	topics := rlog.Topics
	from := common.BytesToAddress(topics[1].Bytes())
	to := common.BytesToAddress(topics[2].Bytes())
	value := new(big.Int).SetBytes(rlog.Data)

	erc20Receipt := &mongodb.Erc20Receipt{
		LogType:  logType,
		LogIndex: logIdx,
		Erc20:    erc20Address,
		From:     strings.ToLower(from.String()),
		To:       strings.ToLower(to.String()),
		Value:    value.String(),
	}

	mt.Erc20Receipts = append(mt.Erc20Receipts, erc20Receipt)
	log.Debug("addErc20Receipt", "receipt", erc20Receipt)

	recordTokenAccounts(erc20Address, erc20Receipt.To)
	return true
}

func recordAccounts(exchange, pairs, account string) {
	ma := &mongodb.MgoAccount{
		Key:      mongodb.GetKeyOfExchangeAndAccount(exchange, account),
		Exchange: strings.ToLower(exchange),
		Pairs:    pairs,
		Account:  strings.ToLower(account),
	}
	_ = mongodb.TryDoTimes("AddAccount "+ma.Key, func() error {
		return mongodb.AddAccount(ma)
	})
}

func recordTokenAccounts(token, account string) {
	ma := &mongodb.MgoTokenAccount{
		Key:     mongodb.GetKeyOfTokenAndAccount(token, account),
		Token:   strings.ToLower(token),
		Account: strings.ToLower(account),
	}
	_ = mongodb.TryDoTimes("AddTokenAccount "+ma.Key, func() error {
		return mongodb.AddTokenAccount(ma)
	})
}

func recordAccountVoumes(mt *mongodb.MgoTransaction, exReceipt *mongodb.ExchangeReceipt, logTopic common.Hash) {
	if !(logTopic == topicTokenPurchase || logTopic == topicEthPurchase) {
		return
	}

	if !params.IsConfigedExchange(exReceipt.Exchange) {
		return
	}

	// ensure addresses are all small case
	account := exReceipt.Address
	if exReceipt.Address == exReceipt.Exchange { // in case of token to token swap
		account = mt.From
	}

	var coinAmount, tokenAmount string
	if logTopic == topicTokenPurchase {
		coinAmount = exReceipt.TokenFromAmount
		tokenAmount = exReceipt.TokenToAmount
	} else if logTopic == topicEthPurchase {
		coinAmount = exReceipt.TokenToAmount
		tokenAmount = exReceipt.TokenFromAmount
	}

	mv := &mongodb.MgoVolumeHistory{
		Key:         mongodb.GetKeyOfVolumeHistory(mt.Hash, exReceipt.LogIndex),
		Exchange:    exReceipt.Exchange,
		Pairs:       exReceipt.Pairs,
		Account:     account,
		CoinAmount:  coinAmount,
		TokenAmount: tokenAmount,
		BlockNumber: mt.BlockNumber,
		Timestamp:   mt.Timestamp,
		TxHash:      mt.Hash,
		LogType:     exReceipt.LogType,
		LogIndex:    exReceipt.LogIndex,
	}
	_ = mongodb.TryDoTimes("AddVolumeHistory "+mv.Key, func() error {
		return mongodb.AddVolumeHistory(mv, overwrite)
	})
}

func updateVolumes(mt *mongodb.MgoTransaction, exReceipt *mongodb.ExchangeReceipt, logTopic common.Hash) {
	if !params.GetConfig().Sync.UpdateVolume {
		return
	}

	if !(logTopic == topicTokenPurchase || logTopic == topicEthPurchase) {
		return
	}

	if !params.IsConfigedExchange(exReceipt.Exchange) {
		return
	}

	timestamp := getDayBegin(mt.Timestamp)
	log.Debug("[parse] update volume", "txHash", mt.Hash,
		"logIndex", exReceipt.LogIndex, "logType", exReceipt.LogType,
		"exchange", exReceipt.Exchange, "pairs", exReceipt.Pairs,
		"tokenFromAmount", exReceipt.TokenFromAmount,
		"tokenToAmount", exReceipt.TokenToAmount,
		"timestamp", timestampToDate(mt.Timestamp))

	_ = mongodb.TryDoTimes("UpdateVolume "+mt.Hash, func() error {
		return mongodb.UpdateVolumeWithReceipt(exReceipt, mt.BlockHash, mt.BlockNumber, timestamp)
	})
}
