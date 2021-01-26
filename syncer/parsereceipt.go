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
	topicCreateExchange  = common.HexToHash("0x9d42cb017eb05bd8944ab536a8b35bc68085931dd5f4356489801453923953f9")
	// exchange v2 topics
	topicMint = common.HexToHash("0x4c209b5fc8ad50758f13e2e1088ba56a560dff690a1c6fef26394f4c03821c4f")
	topicBurn = common.HexToHash("0xdccd412f0b1252819cb1fd330b93224ca42612892bb3f4f789976e6d81936496")
	topicSwap = common.HexToHash("0xd78ad95fa46c994b6551d0da85fc275fe613ce37657fb8d5e3d130840159d822")
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
		case topicCreateExchange:
			addExchanges(rlog)
		case topicMint:
			save = addExchangeV2Receipt(mt, rlog, idx, "Mint")
		case topicBurn:
			save = addExchangeV2Receipt(mt, rlog, idx, "Burn")
		case topicSwap:
			save = addExchangeV2Receipt(mt, rlog, idx, "Swap")
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
	if len(topics) < 4 {
		return false
	}
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

	switch topics[0] {
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
		if !(params.IsScanAllExchange() && params.IsInAllTokenAndExchanges(common.HexToAddress(erc20Address))) {
			return false
		}
	}
	topics := rlog.Topics
	if len(topics) < 3 {
		return false
	}
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

	if topics[0] == topicTransfer {
		recordTokenAccounts(erc20Address, erc20Receipt.To)
	}
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
	if params.IsConfigedExchange(token) ||
		(params.IsScanAllExchange() && params.IsInAllExchanges(common.HexToAddress(token))) {
		exchange := token
		pairs := params.GetExchangePairs(exchange)
		recordAccounts(exchange, pairs, account)
	}
	if !params.IsRecordTokenAccount() {
		return
	}
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
	if onlySyncAccount {
		return
	}
	if !(logTopic == topicTokenPurchase || logTopic == topicEthPurchase) {
		return
	}

	// in case of token to token swap, exclude buyer of exchange
	if params.IsInAllExchanges(common.HexToAddress(exReceipt.Address)) {
		return
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
		Account:     exReceipt.Address,
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
	if onlySyncAccount {
		return
	}
	if !params.GetConfig().Sync.UpdateVolume {
		return
	}

	if !(logTopic == topicTokenPurchase || logTopic == topicEthPurchase) {
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

func addExchangeV2Receipt(mt *mongodb.MgoTransaction, rlog *types.Log, logIdx int, logType string) bool {
	exchange := strings.ToLower(rlog.Address.String())
	topics := rlog.Topics
	data := rlog.Data

	if len(topics) < 2 || len(data) < 64 {
		return false
	}

	exReceipt := &mongodb.ExchangeV2Receipt{
		LogType:  logType,
		LogIndex: logIdx,
		Exchange: exchange,
	}

	exReceipt.Sender = strings.ToLower(common.BytesToAddress(topics[1].Bytes()).String())
	if !params.IsConfigedRouter(exReceipt.Sender) {
		return false
	}
	if len(topics) == 3 {
		exReceipt.To = strings.ToLower(common.BytesToAddress(topics[2].Bytes()).String())
	}

	switch topics[0] {
	case topicMint:
		if len(topics) != 2 || len(data) != 64 {
			return false
		}
		exReceipt.Amount0In = new(big.Int).SetBytes(data[0:32]).String()
		exReceipt.Amount1In = new(big.Int).SetBytes(data[32:64]).String()
		log.Info("[parse] mint", "exchange", exReceipt.Exchange, "amount0In", exReceipt.Amount0In, "amount1In", exReceipt.Amount1In)
	case topicBurn:
		if len(topics) != 3 || len(data) != 64 {
			return false
		}
		exReceipt.Amount0Out = new(big.Int).SetBytes(data[0:32]).String()
		exReceipt.Amount1Out = new(big.Int).SetBytes(data[32:64]).String()
		log.Info("[parse] burn", "exchange", exReceipt.Exchange, "amount0Out", exReceipt.Amount0Out, "amount1Out", exReceipt.Amount1Out)
	case topicSwap:
		if len(topics) != 3 || len(data) != 128 {
			return false
		}
		exReceipt.Amount0In = new(big.Int).SetBytes(data[0:32]).String()
		exReceipt.Amount1In = new(big.Int).SetBytes(data[32:64]).String()
		exReceipt.Amount0Out = new(big.Int).SetBytes(data[64:96]).String()
		exReceipt.Amount1Out = new(big.Int).SetBytes(data[96:128]).String()
		log.Info("[parse] swap", "exchange", exReceipt.Exchange, "amount0In", exReceipt.Amount0In, "amount1In", exReceipt.Amount1In, "amount0Out", exReceipt.Amount0Out, "amount1Out", exReceipt.Amount1Out)
	default:
		return false
	}

	mt.ExchangeV2Receipts = append(mt.ExchangeV2Receipts, exReceipt)
	log.Debug("addExchangeV2Receipt", "receipt", exReceipt)
	return true
}
