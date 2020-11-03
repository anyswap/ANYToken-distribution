package syncer

import (
	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/params"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
	"github.com/fsn-dev/fsn-go-sdk/efsn/core/types"
)

var (
	cachedExchages = make(map[common.Address]struct{})
	cachedTokens   = make(map[common.Address]struct{})
)

func addExchanges(rlog *types.Log) {
	topics := rlog.Topics
	if len(topics) != 2 {
		return
	}
	token := common.BytesToAddress(topics[1].Bytes())
	exchange := common.BytesToAddress(topics[2].Bytes())
	addTokenAndExchange(token, exchange)
}

func initAllExchanges() {
	if !params.IsScanAllExchange() {
		return
	}
	for _, factory := range params.GetFactories() {
		initExchangesInFactory(factory)
	}
}

func initExchangesInFactory(factory common.Address) {
	tokenCount := capi.LoopGetFactoryTokenCount(factory)
	for i := uint64(1); i <= tokenCount; i++ {
		token := capi.LoopGetFactoryTokenWithID(factory, i)
		exchange := capi.LoopGetFactoryExchange(factory, token)
		addTokenAndExchange(token, exchange)
	}
	log.Info("initExchangesInFactory success", "factory", factory.String(), "tokenCount", tokenCount, "added", len(cachedExchages))
}

func addTokenAndExchange(token, exchange common.Address) {
	if token == (common.Address{}) || exchange == (common.Address{}) {
		return
	}
	cachedTokens[token] = struct{}{}
	cachedExchages[exchange] = struct{}{}
}

func isCachedToken(token common.Address) bool {
	_, exist := cachedTokens[token]
	return exist
}

func isCachedExchange(exchange common.Address) bool {
	_, exist := cachedExchages[exchange]
	return exist
}

func isCachedTokenOrExchange(address common.Address) bool {
	return isCachedToken(address) || isCachedExchange(address)
}
