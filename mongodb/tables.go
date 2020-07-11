package mongodb

import (
	"fmt"
	"strings"
)

const (
	tbSyncInfo     string = "SyncInfo"
	tbBlocks       string = "Blocks"
	tbTransactions string = "Transactions"
	tbLiquidity    string = "Liquidity"
	tbVolume       string = "Volume"
	tbAccounts     string = "Accounts"

	// KeyOfLatestSyncInfo key
	KeyOfLatestSyncInfo string = "latest"
)

// MgoSyncInfo sync info
type MgoSyncInfo struct {
	Key       string `bson:"_id"`
	Number    uint64 `bson:"number"`
	Hash      string `bson:"hash"`
	Timestamp uint64 `bson:"timestamp"`
}

// MgoBlock block
type MgoBlock struct {
	Key        string `bson:"_id"` // = hash
	Number     uint64 `bson:"number"`
	Hash       string `bson:"hash"`
	ParentHash string `bson:"parentHash"`
	Nonce      string `bson:"nonce"`
	Miner      string `bson:"miner"`
	Difficulty uint64 `bson:"difficulty"`
	GasLimit   uint64 `bson:"gasLimit"`
	GasUsed    uint64 `bson:"gasUsed"`
	Timestamp  uint64 `bson:"timestamp"`
}

// MgoTransaction tx
type MgoTransaction struct {
	Key              string `bson:"_id"` // = hash
	Hash             string `bson:"hash"`
	BlockNumber      uint64 `bson:"blockNumber"`
	BlockHash        string `bson:"blockHash"`
	TransactionIndex int    `bson:"transactionIndex"`
	From             string `bson:"from"`
	To               string `bson:"to"`
	Value            string `bson:"value"`
	Nonce            uint64 `bson:"nonce"`
	GasLimit         uint64 `bson:"gasLimit"`
	GasUsed          uint64 `bson:"gasUsed"`
	GasPrice         string `bson:"gasPrice"`
	Status           uint64 `bson:"status"`
	Timestamp        uint64 `bson:"timestamp"`

	Erc20Receipts    []*Erc20Receipt    `bson:"erc20Receipts,omitempty"`
	ExchangeReceipts []*ExchangeReceipt `bson:"exchangeReceipts,omitempty"`
}

// Erc20Receipt erc20 tx receipt
type Erc20Receipt struct {
	LogType  string `bson:"logType"`
	LogIndex int    `bson:"logIndex"`
	Erc20    string `bson:"erc20"`
	From     string `bson:"from"`
	To       string `bson:"to"`
	Value    string `bson:"value"`
}

// ExchangeReceipt exchange tx receipt
type ExchangeReceipt struct {
	LogType         string `bson:"txnsType"`
	LogIndex        int    `bson:"logIndex"`
	Exchange        string `bson:"exchange"`
	Pairs           string `bson:"pairs"`
	Address         string `bson:"address"`
	TokenFromAmount string `bson:"tokenFromAmount"`
	TokenToAmount   string `bson:"tokenToAmount"`
}

// MgoLiquidity liquidity
type MgoLiquidity struct {
	Key         string `bson:"_id"` // exchange + ':' + Timestamp's day begin
	Exchange    string `bson:"exchange"`
	Pairs       string `bson:"pairs"`
	Coin        string `bson:"coin"`
	Token       string `bson:"token"`
	Liquidity   string `bson:"liquidity"`
	BlockNumber uint64 `bson:"blockNumber"`
	BlockHash   string `bson:"blockHash"`
	Timestamp   uint64 `bson:"timestamp"`
}

// MgoVolume volumn
type MgoVolume struct {
	Key            string `bson:"_id"` // exchange + ':' + Timestamp's day begin
	Exchange       string `bson:"exchange"`
	Pairs          string `bson:"pairs"`
	CoinVolume24h  string `bson:"cvolume24h"`
	TokenVolume24h string `bson:"tvolume24h"`
	BlockNumber    uint64 `bson:"blockNumber"`
	BlockHash      string `bson:"blockHash"`
	Timestamp      uint64 `bson:"timestamp"`
}

// MgoAccount exchange account
type MgoAccount struct {
	Key      string `bson:"_id"` // exchange + account
	Exchange string `bson:"exchange"`
	Pairs    string `bson:"pairs"`
	Account  string `bson:"account"`
}

// GetKeyOfExchangeAndAccount get key
func GetKeyOfExchangeAndAccount(exchange, account string) string {
	return strings.ToLower(fmt.Sprintf("%s:%s", exchange, account))
}

// GetKeyOfExchangeAndTimestamp get key
func GetKeyOfExchangeAndTimestamp(exchange string, timestamp uint64) string {
	return strings.ToLower(fmt.Sprintf("%s:%d", exchange, timestamp))
}
