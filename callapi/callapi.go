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
	totalSupplyFuncHash := common.FromHex("0x18160ddd")
	res, err := c.CallContract(token, totalSupplyFuncHash, blockNumber)
	if err != nil {
		log.Warn("[callapi] GetTokenTotalSupply error", "token", token.String(), "blockNumber", blockNumber, "err", err)
		return nil, err
	}
	return common.GetBigInt(res, 0, 32), nil
}

func packBytes(bsSlice ...[]byte) []byte {
	if len(bsSlice) == 0 {
		return nil
	}
	result := make([]byte, 0, len(bsSlice)*32-28)
	result = append(result, bsSlice[0]...)
	for i := 1; i < len(bsSlice); i++ {
		result = append(result, common.LeftPadBytes(bsSlice[i], 32)...)
	}
	return result
}

// GetTokenBalance get token balance
func (c *APICaller) GetTokenBalance(token, account common.Address, blockNumber *big.Int) (*big.Int, error) {
	balanceOfFuncHash := common.FromHex("0x70a08231")
	data := packBytes(balanceOfFuncHash, account.Bytes())
	res, err := c.CallContract(token, data, blockNumber)
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
	tokenAddressFuncHash := common.FromHex("0x9d76ea58")
	res, err := c.CallContract(exchange, tokenAddressFuncHash, nil)
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
func (c *APICaller) CallContract(contract common.Address, data []byte, blockNumber *big.Int) (res []byte, err error) {
	msg := ethereum.CallMsg{
		To:   &contract,
		Data: data,
	}
	for i := 0; i < c.rpcRetryCount; i++ {
		res, err = c.client.CallContract(c.context, msg, blockNumber)
		if err == nil {
			break
		}
		log.Error("[callapi] CallContract error", "contract", contract.String(), "blockNumber", blockNumber, "err", err)
		time.Sleep(c.rpcRetryInterval)
	}
	return res, err
}

// GetErc20Name erc20
func (c *APICaller) GetErc20Name(erc20 common.Address) (string, error) {
	res, err := c.CallContract(erc20, common.FromHex("0x06fdde03"), nil)
	if err != nil {
		return "", err
	}
	return UnpackABIEncodedStringInIndex(res, 0)
}

// GetErc20Symbol erc20
func (c *APICaller) GetErc20Symbol(erc20 common.Address) (string, error) {
	res, err := c.CallContract(erc20, common.FromHex("0x95d89b41"), nil)
	if err != nil {
		return "", err
	}
	return UnpackABIEncodedStringInIndex(res, 0)
}

// GetErc20Decimals erc20
func (c *APICaller) GetErc20Decimals(erc20 common.Address) (uint8, error) {
	res, err := c.CallContract(erc20, common.FromHex("0x313ce567"), nil)
	if err != nil {
		return 0, err
	}
	return uint8(common.GetBigInt(res, 0, 32).Uint64()), nil
}

// GetErc20TotalSupply erc20
func (c *APICaller) GetErc20TotalSupply(erc20 common.Address, blockNumber *big.Int) (*big.Int, error) {
	res, err := c.CallContract(erc20, common.FromHex("0x18160ddd"), blockNumber)
	if err != nil {
		return nil, err
	}
	return common.GetBigInt(res, 0, 32), nil
}
