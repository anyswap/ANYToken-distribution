package callapi

import (
	"math/big"
	"time"

	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
	"github.com/fsn-dev/fsn-go-sdk/efsn/core/types"
)

// LoopGetBlockHeader loop get block header
func (c *APICaller) LoopGetBlockHeader(blockNumber *big.Int) *types.Header {
	for {
		header, err := c.client.HeaderByNumber(c.context, blockNumber)
		if err == nil {
			return header
		}
		log.Error("[callapi] get block header failed.", "blockNumber", blockNumber, "err", err)
		time.Sleep(c.rpcRetryInterval)
	}
}

// LoopGetLatestBlockHeader loop get latest block header
func (c *APICaller) LoopGetLatestBlockHeader() *types.Header {
	for {
		header, err := c.client.HeaderByNumber(c.context, nil)
		if err == nil {
			log.Info("[callapi] get latest block header succeed.",
				"number", header.Number,
				"hash", header.Hash().String(),
				"timestamp", header.Time,
			)
			return header
		}
		log.Error("[callapi] get latest block header failed.", "err", err)
		time.Sleep(c.rpcRetryInterval)
	}
}

// LoopGetExchangeLiquidity get exchange liquidity
func (c *APICaller) LoopGetExchangeLiquidity(exchangeAddr common.Address, blockNumber *big.Int) *big.Int {
	return c.LoopGetTokenTotalSupply(exchangeAddr, blockNumber)
}

// LoopGetTokenTotalSupply get token total supply
func (c *APICaller) LoopGetTokenTotalSupply(address common.Address, blockNumber *big.Int) *big.Int {
	var totalSupply *big.Int
	var err error
	for {
		totalSupply, err = c.GetTokenTotalSupply(address, blockNumber)
		if err == nil {
			break
		}
		log.Error("[callapi] GetTokenTotalSupply error", "address", address.String(), "err", err)
		time.Sleep(c.rpcRetryInterval)
	}
	return totalSupply
}

// LoopGetCoinBalance get coin balance
func (c *APICaller) LoopGetCoinBalance(address common.Address, blockNumber *big.Int) *big.Int {
	var fsnBalance *big.Int
	var err error
	for {
		fsnBalance, err = c.GetCoinBalance(address, blockNumber)
		if err == nil {
			break
		}
		log.Error("[callapi] GetCoinBalance error", "address", address.String(), "err", err)
		time.Sleep(c.rpcRetryInterval)
	}
	return fsnBalance
}

// LoopGetExchangeTokenBalance get account token balance
func (c *APICaller) LoopGetExchangeTokenBalance(exchange, token common.Address, blockNumber *big.Int) *big.Int {
	return c.LoopGetTokenBalance(token, exchange, blockNumber)
}

// LoopGetLiquidityBalance get account token balance
func (c *APICaller) LoopGetLiquidityBalance(exchange, account common.Address, blockNumber *big.Int) *big.Int {
	return c.LoopGetTokenBalance(exchange, account, blockNumber)
}

// LoopGetTokenBalance get account token balance
func (c *APICaller) LoopGetTokenBalance(tokenAddr, account common.Address, blockNumber *big.Int) *big.Int {
	var tokenBalance *big.Int
	var err error
	for {
		tokenBalance, err = c.GetTokenBalance(tokenAddr, account, blockNumber)
		if err == nil {
			break
		}
		log.Error("[callapi] GetTokenBalance error", "token", tokenAddr.String(), "account", account.String(), "err", err)
		time.Sleep(c.rpcRetryInterval)
	}
	return tokenBalance
}
