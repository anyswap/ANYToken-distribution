package callapi

import (
	"context"
	"math/big"
	"time"

	"github.com/anyswap/ANYToken-distribution/log"
	ethereum "github.com/fsn-dev/fsn-go-sdk/efsn"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
	"github.com/fsn-dev/fsn-go-sdk/efsn/core/types"
	"github.com/fsn-dev/fsn-go-sdk/efsn/ethclient"
)

// APICaller encapsulate ethclient
type APICaller struct {
	client           *ethclient.Client
	context          context.Context
	rpcRetryCount    int
	rpcRetryInterval time.Duration
}

// NewDefaultAPICaller new default API caller
func NewDefaultAPICaller() *APICaller {
	return &APICaller{
		context:          context.Background(),
		rpcRetryCount:    3,
		rpcRetryInterval: 1 * time.Second,
	}
}

// NewAPICaller new API caller
func NewAPICaller(ctx context.Context, retryCount int, retryInterval time.Duration) *APICaller {
	return &APICaller{
		context:          ctx,
		rpcRetryCount:    retryCount,
		rpcRetryInterval: retryInterval,
	}
}

// DialServer dial server and assign client
func (c *APICaller) DialServer(serverURL string) (err error) {
	c.client, err = ethclient.Dial(serverURL)
	if err != nil {
		log.Error("[callapi] client connection error", "server", serverURL, "err", err)
		return err
	}
	log.Info("[callapi] client connection succeed", "server", serverURL)
	c.LoopGetLatestBlockHeader()
	return nil
}

// CloseClient close client
func (c *APICaller) CloseClient() {
	if c.client != nil {
		c.client.Close()
	}
}

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

// GetCoinBalance get coin balance
func (c *APICaller) GetCoinBalance(account common.Address, blockNumber *big.Int) (balance *big.Int, err error) {
	for i := 0; i < c.rpcRetryCount; i++ {
		balance, err = c.client.BalanceAt(c.context, account, blockNumber)
		if err == nil {
			break
		}
	}
	if err != nil {
		log.Warn("[callapi] GetCoinBalance error", "account", account.String(), "blockNumber", blockNumber, "err", err)
		return nil, err
	}
	return balance, nil
}

// GetExchangeLiquidity get exchange liquidity
func (c *APICaller) GetExchangeLiquidity(exchange common.Address, blockNumber *big.Int) (*big.Int, error) {
	var (
		res []byte
		err error

		totalSupplyFuncHash = common.FromHex("0x18160ddd")
	)
	msg := ethereum.CallMsg{
		To:   &exchange,
		Data: totalSupplyFuncHash,
	}
	for i := 0; i < c.rpcRetryCount; i++ {
		res, err = c.client.CallContract(c.context, msg, blockNumber)
		if err == nil {
			break
		}
	}
	if err != nil {
		log.Warn("[callapi] GetExchangeLiquidity error", "exchange", exchange.String(), "blockNumber", blockNumber, "err", err)
		return nil, err
	}
	return common.GetBigInt(res, 0, 32), nil
}

// GetTokenBalance get token balance
func (c *APICaller) GetTokenBalance(token, account common.Address, blockNumber *big.Int) (*big.Int, error) {
	var (
		res []byte
		err error

		balanceOfFuncHash = common.FromHex("0x70a08231")
	)
	data := make([]byte, 36)
	copy(data[:4], balanceOfFuncHash)
	copy(data[4:], account.Hash().Bytes())
	msg := ethereum.CallMsg{
		To:   &token,
		Data: data,
	}
	for i := 0; i < c.rpcRetryCount; i++ {
		res, err = c.client.CallContract(c.context, msg, blockNumber)
		if err == nil {
			break
		}
	}
	if err != nil {
		log.Warn("[callapi] GetTokenBalance error", "token", token.String(), "account", account.String(), "blockNumber", blockNumber, "err", err)
		return nil, err
	}
	return common.GetBigInt(res, 0, 32), nil
}

// GetExchangeTokenBalance get exchange token balance
func (c *APICaller) GetExchangeTokenBalance(exchange, token common.Address, blockNumber *big.Int) (*big.Int, error) {
	return c.GetTokenBalance(token, exchange, blockNumber)
}

// GetLiquidityBalance get liquidiry balance
func (c *APICaller) GetLiquidityBalance(exchange, account common.Address, blockNumber *big.Int) (*big.Int, error) {
	return c.GetTokenBalance(exchange, account, blockNumber)
}

// GetExchangeTokenAddress get exchange's token address
func (c *APICaller) GetExchangeTokenAddress(exchange common.Address) common.Address {
	var (
		res []byte
		err error

		tokenAddressFuncHash = common.FromHex("0x9d76ea58")
	)
	msg := ethereum.CallMsg{
		To:   &exchange,
		Data: tokenAddressFuncHash,
	}
	for i := 0; i < c.rpcRetryCount; i++ {
		res, err = c.client.CallContract(c.context, msg, nil)
		if err == nil {
			break
		}
	}
	if err != nil {
		return common.Address{}
	}
	return common.BytesToAddress(common.GetData(res, 0, 32))
}

// GetAccountNonce get account nonce
func (c *APICaller) GetAccountNonce(account common.Address) (uint64, error) {
	return c.client.PendingNonceAt(c.context, account)
}

// SendTransaction send signed tx
func (c *APICaller) SendTransaction(tx *types.Transaction) error {
	return c.client.SendTransaction(c.context, tx)
}

// GetChainID get chain ID, also known as network ID
func (c *APICaller) GetChainID() (*big.Int, error) {
	return c.client.NetworkID(c.context)
}

// SuggestGasPrice suggest gas price
func (c *APICaller) SuggestGasPrice() (*big.Int, error) {
	return c.client.SuggestGasPrice(c.context)
}
