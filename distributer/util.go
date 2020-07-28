package distributer

import (
	"math/big"
	"regexp"

	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
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
