package tools

import (
	"encoding/json"
	"fmt"
	"math/big"
)

// ToJSONString to json string
func ToJSONString(content interface{}, pretty bool) string {
	var data []byte
	if pretty {
		data, _ = json.MarshalIndent(content, "", "  ")
	} else {
		data, _ = json.Marshal(content)
	}
	return string(data)
}

// GetBigIntFromString get big int from string
func GetBigIntFromString(str string) (*big.Int, error) {
	bi, ok := new(big.Int).SetString(str, 0)
	if !ok {
		return nil, fmt.Errorf("wrong number '%v'", str)
	}
	return bi, nil
}
