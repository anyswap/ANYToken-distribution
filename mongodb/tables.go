package mongodb

const (
	tbSyncInfo     string = "SyncInfo"
	tbBlocks       string = "Blocks"
	tbTransactions string = "PairsTxns"
	tbTrades       string = "PairsTables"

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

	Pairs           string `bson:"pairs,omitempty"`  // symbol pairs
	Tokens          string `bson:"tokens,omitempty"` // address pairs
	Type            string `bson:"txnsType,omitempty"`
	TotalValue      string `bson:"totalValue,omitempty"`
	TokenFromAmount string `bson:"tokenFromAmount,omitempty"`
	TokenToAmount   string `bson:"tokenToAmount,omitempty"`
}

// MgoTrade trade
type MgoTrade struct {
	Key       string `bson:"_id"` // pairs
	Pairs     string `bson:"pairs"`
	Liquidity string `bson:"liquidity"`
	Volume24h string `bson:"volume24h"`
	Volume7d  string `bson:"volume7d,omitempty"`
	Fees24h   string `bson:"fees24h,omitempty"`
}
