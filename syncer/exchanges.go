package syncer

import (
	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/params"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
	"github.com/fsn-dev/fsn-go-sdk/efsn/core/types"
)

func addExchanges(rlog *types.Log) {
	topics := rlog.Topics
	if len(topics) != 2 {
		return
	}
	token := common.BytesToAddress(topics[1].Bytes())
	exchange := common.BytesToAddress(topics[2].Bytes())
	params.AddTokenAndExchange(token, exchange)
}

func initAllExchanges() {
	for _, factory := range params.GetFactories() {
		initExchangesInFactory(factory)
	}
}

func initExchangesInFactory(factory common.Address) {
	tokenCount := capi.LoopGetFactoryTokenCount(factory)
	for i := uint64(1); i <= tokenCount; i++ {
		token := capi.LoopGetFactoryTokenWithID(factory, i)
		exchange := capi.LoopGetFactoryExchange(factory, token)
		params.AddTokenAndExchange(token, exchange)
	}
	log.Info("initExchangesInFactory success", "factory", factory.String(), "tokenCount", tokenCount, "added", len(params.AllExchanges))
}
