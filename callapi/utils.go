package callapi

import (
	"errors"
	"math/big"

	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
)

var errAccessDataOverflow = errors.New("access data overflow")

// PackStringToABIEncoded pack string to abi encoded bytes
func PackStringToABIEncoded(content string) []byte {
	strLen := uint64(len(content))
	packingLen := (strLen + 31) / 32 * 32
	result := make([]byte, 32+packingLen)
	lengthBytes := common.LeftPadBytes(new(big.Int).SetUint64(strLen).Bytes(), 32)
	copy(result, lengthBytes)
	copy(result[32:], content)
	return result
}

// UnpackABIEncodedString parse abi encoded string
func UnpackABIEncodedString(data []byte, offset uint64) (string, error) {
	dataLen := uint64(len(data))
	if dataLen < offset+32 {
		return "", errAccessDataOverflow
	}
	length, overflow := common.GetUint64(data, offset, 32)
	if overflow || dataLen < offset+32+length {
		return "", errAccessDataOverflow
	}
	bs := data[offset+32 : offset+32+length]
	return string(bs), nil
}

// UnpackABIEncodedStringInIndex parse abi encoded string (index start from 0)
func UnpackABIEncodedStringInIndex(data []byte, index uint64) (string, error) {
	dataLen := uint64(len(data))
	pos := index * 32
	if dataLen < pos+32 {
		return "", errAccessDataOverflow
	}
	offset, overflow := common.GetUint64(data, pos, 32)
	if overflow || dataLen < offset+32 {
		return "", errAccessDataOverflow
	}
	return UnpackABIEncodedString(data, offset)
}
