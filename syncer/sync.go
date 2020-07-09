package syncer

import (
	"context"
	"math/big"
	"sync"
	"time"

	"github.com/anyswap/ANYToken-distribution/cmd/utils"
	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/mongodb"
	"github.com/anyswap/ANYToken-distribution/params"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
	"github.com/fsn-dev/fsn-go-sdk/efsn/core/types"
	"github.com/fsn-dev/fsn-go-sdk/efsn/ethclient"
)

var (
	// configurable syncer items
	serverURL    string
	overwrite           = false
	jobCount     uint64 = 4
	waitInterval uint64 = 6 // seconds
	stableHeight uint64
	startHeight  uint64
	endHeight    uint64

	maxJobs         uint64 = 100
	minWorkBlocks   uint64 = 100
	blockInterval   uint64 = 100
	messageChanSize        = 100

	retryDuration = time.Duration(1) * time.Second
	waitDuration  = time.Duration(waitInterval) * time.Second

	client     *ethclient.Client
	cliContext = context.Background()
	workers    []*worker
)

type message struct {
	block    *types.Block
	receipts types.Receipts
}

type worker struct {
	id     int // identify worker
	stable uint64
	start  uint64
	end    uint64

	messageChan chan *message
}

type syncer struct {
	stable uint64
	start  uint64
	end    uint64
	last   uint64
}

func initConfig() {
	config := params.GetConfig()
	syncCfg := config.Sync

	cJobCount := syncCfg.JobCount
	if cJobCount != 0 {
		if cJobCount > maxJobs {
			cJobCount = maxJobs
		}
		jobCount = cJobCount
	}

	cWaitInterval := syncCfg.WaitInterval
	if cWaitInterval != 0 {
		waitInterval = cWaitInterval
		waitDuration = time.Duration(waitInterval) * time.Second
	}

	serverURL = config.Gateway.APIAddress
	stableHeight = syncCfg.Stable
	startHeight = syncCfg.Start
	endHeight = syncCfg.End
	overwrite = syncCfg.Overwrite

	applyArguments()

	log.Info("init sync parameters finished",
		"serverURL", serverURL,
		"overwrite", overwrite,
		"jobCount", jobCount,
		"waitInterval", waitInterval,
		"stableHeight", stableHeight,
		"startHeight", startHeight,
		"endHeight", endHeight,
	)
}

func applyArguments() {
	args := utils.SyncArgs
	if args.SyncStartHeight != nil {
		startHeight = *args.SyncStartHeight
	}
	if args.SyncEndHeight != nil {
		endHeight = *args.SyncEndHeight
	}
	if args.SyncOverwrite != nil {
		overwrite = *args.SyncOverwrite
	}
}

// Start start syncer
func Start() {
	initConfig()
	newSyncer := &syncer{
		stable: stableHeight,
		start:  startHeight,
		end:    endHeight,
	}
	newSyncer.sync()
}

func dialServer() (err error) {
	client, err = ethclient.Dial(serverURL)
	if err != nil {
		log.Error("[syncer] client connection error", "server", serverURL, "err", err)
		return err
	}
	log.Info("[syncer] client connection succeed", "server", serverURL)
	return nil
}

func closeClient() {
	if client != nil {
		client.Close()
	}
}

func (s *syncer) sync() {
	for {
		err := dialServer()
		if err == nil {
			break
		}
		time.Sleep(3 * time.Second)
	}
	defer closeClient()
	s.dipatchWork()
	s.doWork()
}

func (s *syncer) getStartAndLast() (start, last uint64) {
	start = s.start
	last = s.end
	if s.start == 0 && s.end == 0 {
		syncInfo, err := mongodb.FindLatestSyncInfo()
		if err == nil {
			start = syncInfo.Number
			if start != 0 {
				start++
			}
		}
	}
	for s.end == 0 {
		latestHeader, err := client.HeaderByNumber(cliContext, nil)
		if err == nil {
			last = latestHeader.Number.Uint64()
			if last > s.stable {
				last -= s.stable
			}
			break
		}
		log.Warn("get latest block header failed", "err", err)
		time.Sleep(retryDuration)
	}
	return start, last
}

func (s *syncer) dipatchWork() {
	start, last := s.getStartAndLast()
	if last <= start && s.end != 0 {
		log.Info("no need to sync block", "begin", start, "end", last)
		return
	}

	s.start = start
	s.last = last

	blockCount := uint64(1)
	if last > start {
		blockCount = last - start
	}
	if blockCount < minWorkBlocks && s.end == 0 {
		s.last = start
		return
	}
	workerCount := blockCount / minWorkBlocks
	if workerCount > jobCount {
		workerCount = jobCount
	} else if workerCount == 0 {
		workerCount = 1
	}
	stepCount := blockCount / workerCount

	for i := uint64(0); i < workerCount; i++ {
		wstart := start + i*stepCount
		wend := start + (i+1)*stepCount
		if i == workerCount-1 {
			wend = last
		}
		w := &worker{
			id:          int(i + 1),
			stable:      s.stable,
			start:       wstart,
			end:         wend,
			messageChan: make(chan *message, messageChanSize),
		}
		workers = append(workers, w)
	}

	log.Info("dispatch work", "count", workerCount, "step", stepCount, "start", start, "end", last)
}

func (s *syncer) doWork() {
	if len(workers) != 0 {
		s.doSyncWork()
	}
	if s.end == 0 {
		s.doLoopWork()
	}
}

func (s *syncer) checkSync(start, end uint64) {
	log.Info("checkSync", "from", start, "to", end)
	checkWorker := &worker{
		id:          -1,
		stable:      s.stable,
		start:       start,
		end:         end,
		messageChan: make(chan *message, 10),
	}
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go checkWorker.doSync(wg)
	wg.Wait()
}

func (s *syncer) doSyncWork() {
	log.Info("doSyncWork start", "from", s.start, "to", s.last)
	wg := new(sync.WaitGroup)
	wg.Add(len(workers))
	for _, worker := range workers {
		go worker.doSync(wg)
	}
	wg.Wait()
	log.Info("doSyncWork finished", "from", s.start, "to", s.last)

	log.Info("checkSync start", "from", s.start, "to", s.last)
	s.checkSync(s.start, s.last)
	log.Info("checkSync finished", "from", s.start, "to", s.last)
}

func (s *syncer) doLoopWork() {
	log.Info("doLoopWork start")
	loopWorker := &worker{
		id:          0,
		stable:      s.stable,
		start:       s.last,
		end:         0,
		messageChan: make(chan *message, messageChanSize),
	}
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go loopWorker.doSync(wg)
	wg.Wait()
	log.Info("doLoopWork finished")
}

func (w *worker) doSync(wg *sync.WaitGroup) {
	defer func(bstart time.Time) {
		log.Info("End sync process", "id", w.id, "start", w.start, "end", w.end, "duration", common.PrettyDuration(time.Since(bstart)))
		close(w.messageChan)
		wg.Done()
	}(time.Now())

	wg.Add(1)
	go w.startParser(wg)

	log.Info("Start sync process", "id", w.id, "start", w.start, "end", w.end)

	latest := w.end
	height := w.start
	for {
		if w.end > 0 && height >= w.end {
			break
		}
		if height+w.stable > latest {
			latestHeader, err := client.HeaderByNumber(cliContext, nil)
			if err != nil {
				log.Warn("get latest block header failed", "id", w.id, "err", err)
				time.Sleep(retryDuration)
				continue
			}
			latest = latestHeader.Number.Uint64()
			if height+w.stable > latest {
				time.Sleep(waitDuration)
				continue
			}
		}
		last := latest - w.stable
		if w.end > 0 && last >= w.end {
			last = w.end - 1
		}
		w.syncRange(height, last)
		height = last + 1
	}
	w.messageChan <- nil
}

func getSynced(mbs []*mongodb.MgoBlock, num uint64) *mongodb.MgoBlock {
	for _, mb := range mbs {
		if mb.Number == num {
			return mb
		}
	}
	return nil
}

func (w *worker) calcSyncPercentage(height uint64) float64 {
	if w.end <= w.start {
		return 100
	}
	return 100 * float64(height-w.start) / float64(w.end-w.start)
}

func (w *worker) syncRange(start, end uint64) {
	step := uint64(10000)
	height := start
	for height <= end {
		from := height
		to := from + step - 1
		if to > end {
			to = end
		}
		mblocks, err := mongodb.FindBlocksInRange(from, to)
		if err != nil {
			log.Error("syncRange error", "from", from, "to", to, "err", err)
			time.Sleep(retryDuration)
			continue
		}
		if !overwrite && len(mblocks) == int(to-from+1) {
			log.Info("syncRange already synced", "id", w.id, "from", from, "to", to)
			height = to + 1
			continue
		}
		if w.end != 0 {
			log.Info("syncRange", "id", w.id, "from", from, "to", to, "exist", len(mblocks))
		}
		for height <= to {
			mb := getSynced(mblocks, height)
			if overwrite || mb == nil {
				block, err := client.BlockByNumber(cliContext, new(big.Int).SetUint64(height))
				if err != nil {
					log.Warn("get block failed", "id", w.id, "number", height, "err", err)
					time.Sleep(retryDuration)
					continue
				}
				txs := block.Transactions()
				receipts := make(types.Receipts, len(txs))
				wg := new(sync.WaitGroup)
				wg.Add(len(txs))
				for i, tx := range txs {
					go func(index int, txhash common.Hash) {
						defer wg.Done()
						receipt, _ := client.TransactionReceipt(cliContext, txhash)
						receipts[index] = receipt
					}(i, tx.Hash())
				}
				wg.Wait()
				w.Parse(block, receipts)
				if w.end == 0 {
					log.Info("sync block completed", "id", w.id, "number", height)
				} else if height%blockInterval == 0 {
					log.Info("syncRange in process", "id", w.id, "number", height, "percentage", w.calcSyncPercentage(height))
				}
			}
			height++
		}
		if w.end != 0 {
			log.Info("syncRange completed", "id", w.id, "from", from, "to", to)
		}
	}
}
