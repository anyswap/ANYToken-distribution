package syncer

import (
	"fmt"
	"strings"
	"sync"

	"github.com/anyswap/ANYToken-distribution/mongodb"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
	"github.com/fsn-dev/fsn-go-sdk/efsn/core/types"
)

const (
	maxParseBlocks = 1000
)

// Parse parse block and receipts
func (w *worker) Parse(block *types.Block, receipts types.Receipts) {
	msg := &message{
		block:    block,
		receipts: receipts,
	}
	w.messageChan <- msg
}

func (w *worker) startParser(wg *sync.WaitGroup) {
	defer wg.Done()
	count := 0
	wg2 := new(sync.WaitGroup)
	defer wg2.Wait()
	for {
		msg := <-w.messageChan
		if msg == nil {
			return
		}
		count++
		if !onlySyncAccount {
			wg2.Add(1)
			// parse block
			go w.parseBlock(msg.block, wg2)
		}
		wg2.Add(1)
		// parse transactions
		go w.parseTransactions(msg.block, msg.receipts, wg2)
		if count == maxParseBlocks {
			count = 0
			wg2.Wait() // prevent memory exhausted (when blocks too large)
		}
	}
}

func (w *worker) parseBlock(block *types.Block, wg *sync.WaitGroup) {
	defer wg.Done()
	mb := new(mongodb.MgoBlock)

	hash := block.Hash().String()

	mb.Key = hash
	mb.Number = block.NumberU64()
	mb.Hash = hash
	mb.ParentHash = block.ParentHash().String()
	mb.Nonce = fmt.Sprintf("%d", block.Nonce())
	mb.Miner = strings.ToLower(block.Coinbase().String())
	mb.Difficulty = block.Difficulty().String()
	mb.GasLimit = block.GasLimit()
	mb.GasUsed = block.GasUsed()
	mb.Timestamp = block.Time().Uint64()

	_ = mongodb.TryDoTimes("AddBlock "+mb.Key, func() error {
		return mongodb.AddBlock(mb, overwrite)
	})

	if w.end == 0 && hasSyncToLatest {
		_ = mongodb.TryDoTimes("UpdateSyncInfo "+mb.Hash, func() error {
			return mongodb.UpdateSyncInfo(mb.Number, mb.Hash, mb.Timestamp)
		})
	}
}

func (w *worker) parseTransactions(block *types.Block, receipts types.Receipts, wg *sync.WaitGroup) {
	defer wg.Done()
	wg.Add(len(block.Transactions()))
	for i, tx := range block.Transactions() {
		go w.parseTx(i, tx, block, receipts, wg)
	}
}

func (w *worker) parseTx(i int, tx *types.Transaction, block *types.Block, receipts types.Receipts, wg *sync.WaitGroup) {
	defer wg.Done()
	mt := new(mongodb.MgoTransaction)

	receipt := receipts[i]

	if onlySyncAccount {
		parseReceipt(mt, receipt)
		return
	}

	hash := tx.Hash().String()

	mt.Key = hash
	mt.Hash = hash
	mt.Nonce = tx.Nonce()
	mt.BlockHash = block.Hash().String()
	mt.BlockNumber = block.NumberU64()
	mt.TransactionIndex = i
	mt.From = strings.ToLower(getTxSender(tx).String())
	mt.To = "nil"
	if tx.To() != nil {
		mt.To = strings.ToLower(tx.To().String())
	}
	txValue := tx.Value()
	mt.Value = txValue.String()
	mt.GasLimit = tx.Gas()
	mt.GasPrice = tx.GasPrice().String()
	if receipt != nil {
		mt.GasUsed = receipt.GasUsed
		mt.Status = receipt.Status
	}
	mt.Timestamp = block.Time().Uint64()

	var savedb bool
	if receipt != nil && len(receipt.Logs) != 0 {
		savedb = parseReceipt(mt, receipt)
	}

	if savedb {
		_ = mongodb.TryDoTimes("AddTransaction "+mt.Key, func() error {
			return mongodb.AddTransaction(mt, overwrite)
		})
	}
}

func getTxSender(tx *types.Transaction) common.Address {
	signer := types.NewEIP155Signer(tx.ChainId())
	sender, _ := types.Sender(signer, tx)
	return sender
}
