package distributer

import (
	"math/big"
	"regexp"
	"time"

	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/params"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
	"github.com/fsn-dev/fsn-go-sdk/efsn/core/types"
)

var blankOrCommaSepRegexp = regexp.MustCompile(`[\s,]+`) // blank or comma separated

// CalcTotalValue calc the summary
func CalcTotalValue(shares []*big.Int) *big.Int {
	sum := big.NewInt(0)
	for _, share := range shares {
		if share == nil || share.Sign() <= 0 {
			continue
		}
		sum.Add(sum, share)
	}
	return sum
}

// IsAccountExist judge if given account exist in given slice
func IsAccountExist(account common.Address, accounts []common.Address) bool {
	for _, item := range accounts {
		if item == account {
			return true
		}
	}
	return false
}

// FindBlockByTimestamp find block by timestamp
func FindBlockByTimestamp(timestamp uint64) *types.Header {
	var blockNumber, hi, lo *big.Int

	for blockNumber == nil {
		header := capi.LoopGetBlockHeader(blockNumber)
		headerTime := header.Time.Uint64()
		if headerTime < timestamp {
			log.Info("FindBlockByTimestamp waiting", "bytime", timestamp, "blockNumber", header.Number, "headerTime", headerTime)
			time.Sleep(60 * time.Second)
			continue
		}
		blockNumber = header.Number
		hi = blockNumber
	}

	avgBlockTime := params.GetAverageBlockTime()

	for {
		header := capi.LoopGetBlockHeader(blockNumber)
		headerTime := header.Time.Uint64()
		if headerTime == timestamp {
			return header
		}
		if headerTime > timestamp {
			hi = blockNumber
			if hi.Sign() == 0 {
				return header
			}
			countOfBlocks := (headerTime-timestamp)/avgBlockTime + 1
			blockNumber.Sub(blockNumber, new(big.Int).SetUint64(countOfBlocks))
			if blockNumber.Sign() <= 0 {
				blockNumber = big.NewInt(0)
			}
		} else {
			lo = blockNumber
			break
		}
	}

	return binarySearch(timestamp, hi, lo)
}

func binarySearch(timestamp uint64, hi, lo *big.Int) *types.Header {
	for lo.Cmp(hi) < 0 {
		mid := new(big.Int).Add(lo, hi)
		mid.Div(mid, big.NewInt(2))
		header := capi.LoopGetBlockHeader(mid)
		headerTime := header.Time.Uint64()
		if headerTime == timestamp {
			return header
		}
		if headerTime > timestamp {
			hi = mid
		} else {
			lo = new(big.Int).Add(mid, big.NewInt(1))
		}
	}
	return capi.LoopGetBlockHeader(lo)
}
