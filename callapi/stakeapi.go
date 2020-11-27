package callapi

import (
	"math/big"

	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
)

// GetStakeAmount get stake amount
func (c *APICaller) GetStakeAmount(stakeContract, account common.Address, blockNumber *big.Int) (*big.Int, error) {
	userInfoFuncHash := common.FromHex("0x1959a002")
	data := packBytes(userInfoFuncHash, account.Bytes())
	res, err := c.CallContract(stakeContract, data, blockNumber)
	if err != nil {
		log.Warn("[callapi] GetStakeAmount error", "stakeContract", stakeContract.String(), "account", account.String(), "blockNumber", blockNumber, "err", err)
		return nil, err
	}
	return common.GetBigInt(res, 0, 32), nil
}

// GetRegisteredNodeID get registered nodeID
func (c *APICaller) GetRegisteredNodeID(stakeContract, account common.Address, blockNumber *big.Int) (string, error) {
	userInfoFuncHash := common.FromHex("0x1959a002")
	data := packBytes(userInfoFuncHash, account.Bytes())
	res, err := c.CallContract(stakeContract, data, blockNumber)
	if err != nil {
		log.Warn("[callapi] GetStakeAmount error", "stakeContract", stakeContract.String(), "account", account.String(), "blockNumber", blockNumber, "err", err)
		return "", err
	}
	return UnpackABIEncodedStringInIndex(res, 2)
}
