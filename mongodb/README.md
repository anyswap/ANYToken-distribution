# Mongodb Collections

## SyncInfo

| Name   | Type   | Key    |
| ------ | ------ | ------ |
|Number   |uint64|`bson:"number"`|
|Hash     |string|`bson:"hash"`|
|Timestamp|uint64|`bson:"timestamp"`|

## Blocks

| Name   | Type   | Key    |
| ------ | ------ | ------ |
|Number    |uint64|`bson:"number"`|
|Hash      |string|`bson:"hash"`|
|ParentHash|string|`bson:"parentHash"`|
|Nonce     |string|`bson:"nonce"`|
|Miner     |string|`bson:"miner"`|
|Difficulty|string|`bson:"difficulty"`|
|GasLimit  |uint64|`bson:"gasLimit"`|
|GasUsed   |uint64|`bson:"gasUsed"`|
|Timestamp |uint64|`bson:"timestamp"`|

## Transactions

| Name   | Type   | Key    |
| ------ | ------ | ------ |
|Hash            |string|`bson:"hash"`|
|BlockNumber     |uint64|`bson:"blockNumber"`|
|BlockHash       |string|`bson:"blockHash"`|
|TransactionIndex|int   |`bson:"transactionIndex"`|
|From            |string|`bson:"from"`|
|To              |string|`bson:"to"`|
|Value           |string|`bson:"value"`|
|Nonce           |uint64|`bson:"nonce"`|
|GasLimit        |uint64|`bson:"gasLimit"`|
|GasUsed         |uint64|`bson:"gasUsed"`|
|GasPrice        |string|`bson:"gasPrice"`|
|Status          |uint64|`bson:"status"`|
|Timestamp       |uint64|`bson:"timestamp"`|
||||
|Erc20Receipts   |[]*Erc20Receipt   |`bson:"erc20Receipts,omitempty"`|
|ExchangeReceipts|[]*ExchangeReceipt|`bson:"exchangeReceipts,omitempty"`|

Note that `Erc20Receipts` and `ExchangeReceipts` is an array
because a tx may have multiple receipt logs.
And these fileds may be omitted if not exist.

#### Erc20Receipt

`LogType` can be `Transfer` or `Approval`

`Erc20` is ERC20 contract address

| Name   | Type   | Key    |
| ------ | ------ | ------ |
|LogType |string|`bson:"logType"`|
|LogIndex|int   |`bson:"logIndex"`|
|Erc20   |string|`bson:"erc20"`|
|From    |string|`bson:"from"`|
|To      |string|`bson:"to"`|
|Value   |string|`bson:"value"`|

#### ExchangeReceipt

`LogType` can be `AddLiquidity`, `RemoveLiquidity`, `TokenPurchase` or `EthPurchase`

`Pairs` is read from config file

| Name   | Type   | Key    |
| ------ | ------ | ------ |
|LogType        |string|`bson:"txnsType"`|
|LogIndex       |int   |`bson:"logIndex"`|
|Exchange       |string|`bson:"exchange"`|
|Pairs          |string|`bson:"pairs"`|
|Address        |string|`bson:"address"`|
|TokenFromAmount|string|`bson:"tokenFromAmount"`|
|TokenToAmount  |string|`bson:"tokenToAmount"`|

## Liquidity

one record per day

| Name   | Type   | Key    |
| ------ | ------ | ------ |
|Exchange   |string|`bson:"exchange"`|
|Pairs      |string|`bson:"pairs"`|
|Coin       |string|`bson:"coin"`|
|Token      |string|`bson:"token"`|
|Liquidity  |string|`bson:"liquidity"`|
|BlockNumber|uint64|`bson:"blockNumber"`|
|BlockHash  |string|`bson:"blockHash"`|
|Timestamp  |uint64|`bson:"timestamp"`|

## Volume

one record per day

| Name   | Type   | Key    |
| ------ | ------ | ------ |
|Exchange      |string|`bson:"exchange"`|
|Pairs         |string|`bson:"pairs"`|
|CoinVolume24h |string|`bson:"cvolume24h"`|
|TokenVolume24h|string|`bson:"tvolume24h"`|
|BlockNumber   |uint64|`bson:"blockNumber"`|
|BlockHash     |string|`bson:"blockHash"`|
|Timestamp     |uint64|`bson:"timestamp"`|

## Accounts

| Name   | Type   | Key    |
| ------ | ------ | ------ |
|Exchange|string|`bson:"exchange"`|
|Pairs   |string|`bson:"pairs"`|
|Account |string|`bson:"account"`|

## LiquidityBalances

| Name   | Type   | Key    |
| ------ | ------ | ------ |
|Exchange   |string|`bson:"exchange"`|
|Pairs      |string|`bson:"pairs"`|
|Account    |string|`bson:"account"`|
|BlockNumber|uint64|`bson:"blockNumber"`|
|Liquidity  |string|`bson:"liquidity"`|

## VolumeHistory

| Name   | Type   | Key    |
| ------ | ------ | ------ |
|Exchange   |string|`bson:"exchange"`|
|Pairs      |string|`bson:"pairs"`|
|Account    |string|`bson:"account"`|
|CoinAmount |string|`bson:"coinAmount"`|
|TokenAmount|string|`bson:"tokenAmount"`|
|BlockNumber|uint64|`bson:"blockNumber"`|
|Timestamp  |uint64|`bson:"timestamp"`|
|TxHash     |string|`bson:"txhash"`|
|LogType    |string|`bson:"logType"`|
|LogIndex   |int   |`bson:"logIndex"`|

