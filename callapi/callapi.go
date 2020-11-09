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
	return c.GetTokenTotalSupply(exchange, blockNumber)
}

// GetTokenTotalSupply get token total spply
func (c *APICaller) GetTokenTotalSupply(token common.Address, blockNumber *big.Int) (*big.Int, error) {
	var (
		res []byte
		err error

		totalSupplyFuncHash = common.FromHex("0x18160ddd")
	)
	msg := ethereum.CallMsg{
		To:   &token,
		Data: totalSupplyFuncHash,
	}
	for i := 0; i < c.rpcRetryCount; i++ {
		res, err = c.client.CallContract(c.context, msg, blockNumber)
		if err == nil {
			break
		}
	}
	if err != nil {
		log.Warn("[callapi] GetTokenTotalSupply error", "token", token.String(), "blockNumber", blockNumber, "err", err)
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

// IsNodeSyncing return if full node is in syncing state
func (c *APICaller) IsNodeSyncing() bool {
	for {
		progress, err := c.client.SyncProgress(c.context)
		if err == nil {
			log.Info("call eth_syncing success", "progress", progress)
			return progress != nil
		}
		log.Warn("call eth_syncing failed", "err", err)
		time.Sleep(c.rpcRetryInterval)
	}
}

// CallContract common call contract
func (c *APICaller) CallContract(contract common.Address, data []byte) (res []byte, err error) {
	msg := ethereum.CallMsg{
		To:   &contract,
		Data: data,
	}
	for i := 0; i < c.rpcRetryCount; i++ {
		res, err = c.client.CallContract(c.context, msg, nil)
		if err == nil {
			break
		}
		log.Error("[callapi] CallContract error", "contract", contract.String(), "err", err)
		time.Sleep(c.rpcRetryInterval)
	}
	return res, err
}

func getStringFromABIEncodedData(res []byte, pos uint64) string {
	datalen := uint64(len(res))
	offset, overflow := common.GetUint64(res, pos, 32)
	if overflow || datalen < offset+32 {
		return ""
	}
	length, overflow := common.GetUint64(res, offset, 32)
	if overflow || datalen < offset+32+length {
		return ""
	}
	return string(res[offset+32 : offset+32+length])
}

// GetErc20Name erc20
func (c *APICaller) GetErc20Name(erc20 common.Address) (string, error) {
	res, err := c.CallContract(erc20, common.FromHex("0x06fdde03"))
	if err != nil {
		return "", err
	}
	return getStringFromABIEncodedData(res, 0), nil
}

// GetErc20Symbol erc20
func (c *APICaller) GetErc20Symbol(erc20 common.Address) (string, error) {
	res, err := c.CallContract(erc20, common.FromHex("0x95d89b41"))
	if err != nil {
		return "", err
	}
	return getStringFromABIEncodedData(res, 0), nil
}

// GetErc20Decimals erc20
func (c *APICaller) GetErc20Decimals(erc20 common.Address) (uint8, error) {
	res, err := c.CallContract(erc20, common.FromHex("0x313ce567"))
	if err != nil {
		return 0, err
	}
	return uint8(common.GetBigInt(res, 0, 32).Uint64()), nil
}

// GetErc20TotalSupply erc20
func (c *APICaller) GetErc20TotalSupply(erc20 common.Address) (*big.Int, error) {
	res, err := c.CallContract(erc20, common.FromHex("0x18160ddd"))
	if err != nil {
		return nil, err
	}
	return common.GetBigInt(res, 0, 32), nil
}
