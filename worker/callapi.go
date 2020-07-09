package worker

import (
	"context"
	"math/big"
	"time"

	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/params"
	ethereum "github.com/fsn-dev/fsn-go-sdk/efsn"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
	"github.com/fsn-dev/fsn-go-sdk/efsn/core/types"
	"github.com/fsn-dev/fsn-go-sdk/efsn/ethclient"
)

var (
	client     *ethclient.Client
	cliContext = context.Background()

	rpcRetryCount    = 3
	rpcRetryInterval = 3 * time.Second
)

func dialServer() (err error) {
	config := params.GetConfig()
	serverURL := config.Gateway.APIAddress
	client, err = ethclient.Dial(serverURL)
	if err != nil {
		log.Error("[worker] client connection error", "server", serverURL, "err", err)
		return err
	}
	log.Info("[worker] client connection succeed", "server", serverURL)
	loopGetLatestBlockHeader()
	return nil
}

func closeClient() {
	if client != nil {
		client.Close()
	}
}

func loopGetBlockHeader(blockNumber *big.Int) *types.Header {
	for {
		header, err := client.HeaderByNumber(cliContext, blockNumber)
		if err == nil {
			return header
		}
		log.Error("[worker] get block header failed.", "blockNumber", blockNumber, "err", err)
		time.Sleep(rpcRetryInterval)
	}
}

func loopGetLatestBlockHeader() *types.Header {
	for {
		header, err := client.HeaderByNumber(cliContext, nil)
		if err == nil {
			log.Info("[worker] get latest block header succeed.",
				"number", header.Number,
				"hash", header.Hash().String(),
				"timestamp", header.Time,
			)
			return header
		}
		log.Error("[worker] get latest block header failed.", "err", err)
		time.Sleep(rpcRetryInterval)
	}
}

func getCoinBalance(account common.Address, blockNumber *big.Int) (balance *big.Int, err error) {
	for i := 0; i < rpcRetryCount; i++ {
		balance, err = client.BalanceAt(cliContext, account, blockNumber)
		if err == nil {
			break
		}
	}
	if err != nil {
		log.Warn("[worker] getCoinBalance error", "blockNumber", blockNumber, "err", err)
		return nil, err
	}
	return balance, nil
}

func getExchangeLiquidity(exchange common.Address, blockNumber *big.Int) (*big.Int, error) {
	var (
		res []byte
		err error

		totalSupplyFuncHash = common.FromHex("0x18160ddd")
	)
	msg := ethereum.CallMsg{
		To:   &exchange,
		Data: totalSupplyFuncHash,
	}
	for i := 0; i < rpcRetryCount; i++ {
		res, err = client.CallContract(cliContext, msg, blockNumber)
		if err == nil {
			break
		}
	}
	if err != nil {
		log.Warn("[worker] getExchangeLiquidity error", "blockNumber", blockNumber, "err", err)
		return nil, err
	}
	return common.GetBigInt(res, 0, 32), nil
}

func getExchangeTokenBalance(exchange, contract common.Address, blockNumber *big.Int) (*big.Int, error) {
	var (
		res []byte
		err error

		balanceOfFuncHash = common.FromHex("0x70a08231")
	)
	data := make([]byte, 36)
	copy(data[:4], balanceOfFuncHash)
	copy(data[4:], exchange.Hash().Bytes())
	msg := ethereum.CallMsg{
		To:   &contract,
		Data: data,
	}
	for i := 0; i < rpcRetryCount; i++ {
		res, err = client.CallContract(cliContext, msg, blockNumber)
		if err == nil {
			break
		}
	}
	if err != nil {
		log.Warn("[worker] getExchangeLiquidity error", "blockNumber", blockNumber, "err", err)
		return nil, err
	}
	return common.GetBigInt(res, 0, 32), nil
}
