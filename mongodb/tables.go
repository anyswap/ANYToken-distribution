package mongodb

import (
	"fmt"
	"strings"

	"gopkg.in/mgo.v2/bson"
)

const (
	tbSyncInfo           string = "SyncInfo"
	tbBlocks             string = "Blocks"
	tbTransactions       string = "Transactions"
	tbLiquidity          string = "Liquidity"
	tbVolume             string = "Volume"
	tbVolumeHistory      string = "VolumeHistory"
	tbAccounts           string = "Accounts"
	tbLiquidityBalance   string = "LiquidityBalances"
	tbDistributeInfo     string = "DistributeInfo"
	tbVolumeRewardResult string = "VolumeRewardResult"
	tbLiquidRewardResult string = "LiquidRewardResult"

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
	Difficulty string `bson:"difficulty"`
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
	Key         string `bson:"_id"` // exchange + timestamp
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
	Key            string `bson:"_id"` // exchange + timestamp
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

// MgoLiquidityBalance liquidity balance
type MgoLiquidityBalance struct {
	Key         string `bson:"_id"` // exchange + account + blockNumber
	Exchange    string `bson:"exchange"`
	Pairs       string `bson:"pairs"`
	Account     string `bson:"account"`
	BlockNumber uint64 `bson:"blockNumber"`
	Liquidity   string `bson:"liquidity"`
}

// MgoVolumeHistory volmue tx history
type MgoVolumeHistory struct {
	Key         string `bson:"_id"` // txhash + logIndex
	Exchange    string `bson:"exchange"`
	Pairs       string `bson:"pairs"`
	Account     string `bson:"account"`
	CoinAmount  string `bson:"coinAmount"`
	TokenAmount string `bson:"tokenAmount"`
	BlockNumber uint64 `bson:"blockNumber"`
	Timestamp   uint64 `bson:"timestamp"`
	TxHash      string `bson:"txhash"`
	LogType     string `bson:"logType"`
	LogIndex    int    `bson:"logIndex"`
}

// MgoDistributeInfo distribute info
type MgoDistributeInfo struct {
	Key          bson.ObjectId `bson:"_id"`
	Exchange     string        `bson:"exchange"`
	Pairs        string        `bson:"pairs"`
	ByWhat       string        `bson:"bywhat"`
	Start        uint64        `bson:"start"`
	End          uint64        `bson:"end"`
	RewardToken  string        `bson:"rewardToken"`
	Rewards      string        `bson:"rewards"`
	SampleHeigts []uint64      `bson:"sampleHeights,omitempty"`
}

// MgoVolumeRewardResult volume reward
type MgoVolumeRewardResult struct {
	Key         string `bson:"_id"` // exchange + start + end
	Exchange    string `bson:"exchange"`
	Pairs       string `bson:"pairs"`
	Start       uint64 `bson:"start"`
	End         uint64 `bson:"end"`
	RewardToken string `bson:"rewardToken"`
	Account     string `bson:"account"`
	Reward      string `bson:"reward"`
	Volume      string `bson:"volume"`
	TxCount     string `bson:"txcount"`
	RewardTx    string `bson:"rewardTx"`
}

// MgoLiquidRewardResult liquidity reward
type MgoLiquidRewardResult struct {
	Key         string `bson:"_id"` // exchange + start + end
	Exchange    string `bson:"exchange"`
	Pairs       string `bson:"pairs"`
	Start       uint64 `bson:"start"`
	End         uint64 `bson:"end"`
	RewardToken string `bson:"rewardToken"`
	Account     string `bson:"account"`
	Reward      string `bson:"reward"`
	Liquidity   string `bson:"liquidity"`
	Height      string `bson:"height"`
	RewardTx    string `bson:"rewardTx"`
}

// GetKeyOfRewardResult get key
func GetKeyOfRewardResult(exchange string, start, end uint64) string {
	return strings.ToLower(fmt.Sprintf("%s:%d-%d", exchange, start, end))
}

// GetKeyOfExchangeAndAccount get key
func GetKeyOfExchangeAndAccount(exchange, account string) string {
	return strings.ToLower(fmt.Sprintf("%s:%s", exchange, account))
}

// GetKeyOfExchangeAndTimestamp get key
func GetKeyOfExchangeAndTimestamp(exchange string, timestamp uint64) string {
	return strings.ToLower(fmt.Sprintf("%s:%d", exchange, timestamp))
}

// GetKeyOfLiquidityBalance get key
func GetKeyOfLiquidityBalance(exchange, account string, blockNumber uint64) string {
	return strings.ToLower(fmt.Sprintf("%s:%s:%d", exchange, account, blockNumber))
}

// GetKeyOfVolumeHistory get key
func GetKeyOfVolumeHistory(txhash string, logIndex int) string {
	return fmt.Sprintf("%s:%d", txhash, logIndex)
}
