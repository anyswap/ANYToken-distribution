package worker

import (
	"github.com/anyswap/ANYToken-distribution/callapi"
	"github.com/anyswap/ANYToken-distribution/distributer"
	"github.com/anyswap/ANYToken-distribution/syncer"
)

var capi *callapi.APICaller

// StartWork start all work
func StartWork(apiCaller *callapi.APICaller) {
	capi = apiCaller

	go syncer.Start()

	go updateLiquidityDaily()

	go distributer.Start(capi)

	exitCh := make(chan struct{})
	<-exitCh
}
