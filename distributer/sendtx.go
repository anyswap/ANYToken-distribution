package distributer

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"strings"
	"time"

	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/mongodb"
	"github.com/anyswap/ANYToken-distribution/params"
	"github.com/fsn-dev/fsn-go-sdk/efsn/accounts/keystore"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
	"github.com/fsn-dev/fsn-go-sdk/efsn/core/types"
)

var (
	transferFuncHash = common.FromHex("0xa9059cbb")

	errDustReward = errors.New("dust reward")
)

// BuildTxArgs build tx args
type BuildTxArgs struct {
	Sender       string
	KeystoreFile string `json:"-"`
	PasswordFile string `json:"-"`

	Nonce    *uint64
	GasLimit *uint64
	GasPrice *big.Int

	// calculated result
	keyWrapper  *keystore.Key
	fromAddr    common.Address
	chainID     *big.Int
	chainSigner types.Signer
}

// GetSender get sender from keystore
func (args *BuildTxArgs) GetSender() common.Address {
	return args.fromAddr
}

// GetChainID get chainID
func (args *BuildTxArgs) GetChainID() *big.Int {
	return args.chainID
}

// Check check common args
func (args *BuildTxArgs) Check(dryRun bool) error {
	if args.Sender != "" && !common.IsHexAddress(args.Sender) {
		return fmt.Errorf("wrong sender address '%v'", args.Sender)
	}
	if !dryRun {
		err := args.loadKeyStore()
		if err != nil {
			return err
		}
		if !strings.EqualFold(args.Sender, args.fromAddr.String()) {
			return fmt.Errorf("sender mismatch. sender from args = '%v', sender from keystore = '%v'", args.Sender, args.fromAddr.String())
		}
	} else {
		if args.Sender != "" {
			args.fromAddr = common.HexToAddress(args.Sender)
		} else {
			args.Sender = args.fromAddr.String()
		}
	}
	log.Info("get build transaction's sender", "sender", args.Sender)
	args.setDefaults()
	return nil
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
	if args.Sender == "" {
		args.Sender = args.fromAddr.String()
	}
	return nil
}

func (args *BuildTxArgs) setDefaults() {
	from := args.fromAddr
	var err error
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
}

func (args *BuildTxArgs) sendRewardsTransaction(account common.Address, reward *big.Int, rewardToken common.Address, dryRun bool) (txHash *common.Hash, err error) {
	dustRewardThreshold := params.GetDustRewardThreshold()
	if reward.Cmp(dustRewardThreshold) < 0 {
		log.Info("sendRewards ignore dust reward", "account", account.String(), "reward", reward, "dustRewardThreshold", dustRewardThreshold)
		return nil, errDustReward
	}
	if dryRun {
		log.Info("sendRewards dry run", "account", account.String(), "reward", reward)
		return nil, nil
	}

	nonce, err := capi.GetAccountNonce(args.fromAddr)
	if err == nil && nonce > *args.Nonce {
		*args.Nonce = nonce
	}

	var rawTx *types.Transaction

	if rewardToken != (common.Address{}) {
		data := make([]byte, 68)
		copy(data[:4], transferFuncHash)
		copy(data[4:36], account.Hash().Bytes())
		copy(data[36:68], common.LeftPadBytes(reward.Bytes(), 32))

		rawTx = types.NewTransaction(*args.Nonce, rewardToken, big.NewInt(0), *args.GasLimit, args.GasPrice, data)
	} else {
		rawTx = types.NewTransaction(*args.Nonce, account, reward, *args.GasLimit, args.GasPrice, nil)
	}

	signedTx, err := types.SignTx(rawTx, args.chainSigner, args.keyWrapper.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("sign tx failed, %v", err)
	}

	err = capi.SendTransaction(signedTx)
	if err != nil {
		return nil, fmt.Errorf("send tx failed, %v", err)
	}
	*args.Nonce++

	signedTxHash := signedTx.Hash()
	txHash = &signedTxHash
	log.Info("sendRewards success", "account", account.String(), "reward", reward, "txHash", txHash.String())
	return txHash, nil
}

func (opt *Option) checkSendRewardsFromFile(ifile string) (mongodb.AccountStatSlice, error) {
	accountStats, _, err := GetAccountsAndRewardsFromFile(ifile)
	if err != nil {
		log.Error("[sendRewards] get accounts and rewards from input file failed", "inputfile", ifile, "err", err)
		return nil, err
	}
	if len(accountStats) == 0 {
		log.Warn("empty account list, no need to send reward")
		return nil, nil
	}

	// assign total value before check balance
	opt.TotalValue = accountStats.CalcTotalReward()
	if opt.RewardToken != "" {
		err = opt.CheckSenderRewardTokenBalance()
	} else {
		err = opt.CheckSenderCoinBalance()
	}
	if err != nil {
		return nil, err
	}

	return accountStats, nil
}

// SendRewardsFromFile send rewards from file
func (opt *Option) SendRewardsFromFile() (err error) {
	if len(opt.Exchanges) != 0 {
		if len(opt.InputFiles) != len(opt.Exchanges) {
			return fmt.Errorf("count of exchanges and input files is not equal")
		}
		if len(opt.OutputFiles) != len(opt.Exchanges) {
			return fmt.Errorf("count of exchanges and output files is not equal")
		}
	} else if len(opt.InputFiles) != len(opt.OutputFiles) {
		return fmt.Errorf("count of input and output files is not equal")
	}

	totalRewardsSended := big.NewInt(0)

	var rewardsSended *big.Int
	var exchange string
	for i, inputFile := range opt.InputFiles {
		if len(opt.Exchanges) != 0 {
			exchange = opt.Exchanges[i]
		}
		outputFile := opt.OutputFiles[i]
		rewardsSended, err = opt.sendRewardsFromFile(exchange, inputFile, outputFile)
		if rewardsSended != nil {
			totalRewardsSended.Add(totalRewardsSended, rewardsSended)
		}
		if err != nil {
			log.Error("send reward from file failed", "exchange", exchange, "index", i, "input", inputFile, "output", outputFile, "err", err)
			break
		}
	}
	log.Infof("total sended reward is %v, input file count is %v\n", totalRewardsSended, len(opt.InputFiles))
	return err
}

func (opt *Option) sendRewardsFromFile(exchange, ifile, ofile string) (rewardsSended *big.Int, err error) {
	accountStats, err := opt.checkSendRewardsFromFile(ifile)
	if err != nil {
		return nil, err
	}
	outputFile, err := openOutputFile(ofile)
	if err != nil {
		return nil, err
	}

	log.Info("call send rewards from file", "input", ifile, "output", ofile)
	defer opt.deinit()

	rewardsSended = big.NewInt(0)
	totalDustReward := big.NewInt(0)
	totalDustRewardCount := 0
	i := uint64(0)
	for _, stat := range accountStats {
		account := stat.Account
		reward := stat.Reward
		if reward == nil || reward.Sign() <= 0 {
			log.Info("ignore zero reward line", "account", account)
			continue
		}
		txHash, err := opt.SendRewardsTransaction(account, reward)
		switch err {
		case nil:
		case errDustReward:
			totalDustReward.Add(totalDustReward, reward)
			totalDustRewardCount++
		default:
			log.Error("[sendRewardsFromFile] send tx failed", "account", account.String(), "reward", reward, "dryrun", opt.DryRun, "err", err)
			return rewardsSended, errSendTransactionFailed
		}
		rewardsSended.Add(rewardsSended, reward)
		if opt.DryRun || txHash != nil {
			// write body
			_ = opt.WriteSendRewardResult(outputFile, exchange, stat, txHash)
			i++
		}
		if !opt.DryRun && opt.BatchCount > 0 && i%opt.BatchCount == 0 {
			time.Sleep(time.Duration(opt.BatchInterval) * time.Millisecond)
		}
	}

	log.Info("[sendRewardsFromFile] rewards sended",
		"exchange", exchange,
		"totalRewards", opt.TotalValue,
		"rewardsSended", rewardsSended,
		"allRewardsSended", opt.TotalValue == nil || rewardsSended.Cmp(opt.TotalValue) == 0,
		"totalDustReward", totalDustReward,
		"totalDustRewardCount", totalDustRewardCount,
	)
	return rewardsSended, nil
}
