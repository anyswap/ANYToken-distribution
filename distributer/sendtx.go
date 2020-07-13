package distributer

import (
	"fmt"
	"io/ioutil"
	"math/big"
	"strings"

	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/fsn-dev/fsn-go-sdk/efsn/accounts/keystore"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
	"github.com/fsn-dev/fsn-go-sdk/efsn/core/types"
)

var (
	transferFuncHash = common.FromHex("0xa9059cbb")

	commonTxArgs *BuildTxArgs
)

// BuildTxArgs build tx args
type BuildTxArgs struct {
	KeystoreFile string
	PasswordFile string

	Nonce    *uint64
	GasLimit *uint64
	GasPrice *big.Int

	// calculated result
	keyWrapper  *keystore.Key
	fromAddr    common.Address
	chainID     *big.Int
	chainSigner types.Signer
}

// InitBuildTxArgs init buildtx args
func InitBuildTxArgs(args *BuildTxArgs) {
	commonTxArgs = args
}

// Check check common args
func (args *BuildTxArgs) Check() error {
	if args == nil {
		return fmt.Errorf("[check] BuildTxArgs is not init")
	}
	err := args.loadKeyStore()
	if err != nil {
		return err
	}
	return args.setDefaults()
}

func (args *BuildTxArgs) loadKeyStore() error {
	keyfile := args.KeystoreFile
	passfile := args.PasswordFile
	keyjson, err := ioutil.ReadFile(keyfile)
	if err != nil {
		log.Println("read keystore fail", err)
		return err
	}

	passdata, err := ioutil.ReadFile(passfile)
	if err != nil {
		log.Println("read password fail", err)
		return err
	}
	passwd := strings.TrimSpace(string(passdata))

	log.Println("decrypt keystore ......")
	args.keyWrapper, err = keystore.DecryptKey(keyjson, passwd)
	if err != nil {
		log.Println("key decrypt fail", err)
		return err
	}
	args.fromAddr = args.keyWrapper.Address
	log.Info("decrypt keystore succeed", "from", args.fromAddr.String())
	return nil
}

func (args *BuildTxArgs) setDefaults() (err error) {
	from := args.fromAddr
	for {
		if args.chainID == nil {
			args.chainID, err = capi.GetChainID()
			if err != nil {
				log.Warn("get chain ID error", "err", err)
				continue
			}
			args.chainSigner = types.NewEIP155Signer(args.chainID)
		}
		log.Info("get chain ID succeed", "chainID", args.chainID)
		if args.Nonce == nil {
			var nonce uint64
			nonce, err = capi.GetAccountNonce(from)
			if err != nil {
				log.Warn("get nonce error", "from", from.String(), "err", err)
				continue
			}
			args.Nonce = &nonce
		}
		log.Info("get nonce succeed", "from", from.String(), "nonce", *args.Nonce)
		if args.GasPrice == nil {
			args.GasPrice, err = capi.SuggestGasPrice()
			if err != nil {
				log.Warn("get gas price error", "err", err)
				continue
			}
		}
		log.Info("get gas price succeed", "gasPrice", args.GasPrice)
		if args.GasLimit == nil {
			defaultGasLimit := uint64(90000)
			args.GasLimit = &defaultGasLimit
		}
		log.Info("get gas limit succeed", "gasLimit", *args.GasLimit)
		break
	}
	return nil
}

func (args *BuildTxArgs) sendRewardsTransaction(account common.Address, reward *big.Int, rewardToken common.Address, dryRun bool) error {
	data := make([]byte, 68)
	copy(data[:4], transferFuncHash)
	copy(data[4:36], account.Hash().Bytes())
	copy(data[36:68], common.LeftPadBytes(reward.Bytes(), 32))

	if nonce, err := capi.GetAccountNonce(args.fromAddr); err == nil {
		if nonce > *args.Nonce {
			*args.Nonce = nonce
		}
	}

	rawTx := types.NewTransaction(*args.Nonce, rewardToken, big.NewInt(0), *args.GasLimit, args.GasPrice, data)

	signedTx, err := types.SignTx(rawTx, args.chainSigner, args.keyWrapper.PrivateKey)
	if err != nil {
		return fmt.Errorf("sign tx failed, err=%v", err)
	}

	txHash := signedTx.Hash().String()

	if !dryRun {
		err = capi.SendTransaction(signedTx)
		if err != nil {
			return fmt.Errorf("send transaction failed, err=%v", err)
		}
		*args.Nonce++
		log.Printf("send tx success, tx hash is %v\n", txHash)
	} else {
		log.Println("is dry run, does not send transaction")
	}
	return nil
}
