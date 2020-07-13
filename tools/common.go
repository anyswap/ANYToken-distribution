package tools

import (
	"fmt"
	"math/big"
)

// GetBigIntFromString get big int from string
func GetBigIntFromString(str string) (*big.Int, error) {
	bi, ok := new(big.Int).SetString(str, 0)
	if !ok {
		return nil, fmt.Errorf("wrong number '%v'", str)
	}
	return bi, nil
}
