package main

import (
	"fmt"
	"math/big"

	"github.com/anyswap/ANYToken-distribution/cmd/utils"
	"github.com/anyswap/ANYToken-distribution/distributer"
	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
	"github.com/urfave/cli/v2"
)

var (
	sendRewardsCommand = &cli.Command{
		Action:    sendRewards,
		Name:      "sendrewards",
		Usage:     "send rewards batchly",
		ArgsUsage: " ",
		Description: `
send rewards batchly according to verified input file with line format: <address> <rewards>
`,
		Flags: []cli.Flag{
			utils.GatewayFlag,
			utils.RewardTokenFlag,
			utils.InputFileFlag,
			utils.KeyStoreFileFlag,
			utils.PasswordFileFlag,
			utils.GasLimitFlag,
			utils.GasPriceFlag,
			utils.AccountNonceFlag,
			utils.OutputFileFlag,
			utils.DryRunFlag,
		},
	}
)

func sendRewards(ctx *cli.Context) (err error) {
	utils.SetLogger(ctx)
	serverURL := ctx.String(utils.GatewayFlag.Name)
	if serverURL == "" {
		return fmt.Errorf("must specify gateway URL")
	}

	capi := utils.DialServer(serverURL)
	defer capi.CloseClient()
	distributer.SetAPICaller(capi)

	opt, err := getOptionAndTxArgs(ctx)
	if err != nil {
		log.Error("[sendRewards] get option and args failed", "err", err)
		return err
	}

	if !common.IsHexAddress(opt.RewardToken) {
		return fmt.Errorf("wrong reward token: '%v'", opt.RewardToken)
	}
	if opt.InputFile == "" {
		return fmt.Errorf("must specify input file")
	}

	accounts, rewards, err := opt.GetAccountsAndRewardsFromFile()
	if err != nil {
		log.Error("[sendRewards] get accounts and rewards from input file failed", "inputfile", opt.InputFile, "err", err)
		return err
	}

	if len(accounts) != len(rewards) {
		return fmt.Errorf("accounts length %v is not equal to rewards length %v", len(accounts), len(rewards))
	}

	totalRewards := distributer.CalcTotalValue(rewards)

	opt.TotalValue = totalRewards
	err = opt.CheckSenderRewardTokenBalance()
	if err != nil {
		log.Errorf("[sendRewards] sender %v has not enough token balance (< %v), token: %v", opt.GetSender().String(), totalRewards, opt.RewardToken)
		return err
	}

	rewardsSended := big.NewInt(0)
	for i, account := range accounts {
		reward := rewards[i]
		if reward == nil || reward.Sign() <= 0 {
			log.Info("ignore zero reward line", "account", account)
			continue
		}
		txHash, err := opt.SendRewardsTransaction(account, reward)
		if err != nil {
			log.Info("[sendRewards] rewards sended", "totalRewards", totalRewards, "rewardsSended", rewardsSended, "allRewardsSended", rewardsSended.Cmp(totalRewards) == 0)
			log.Error("[sendRewards] send tx failed", "account", account.String(), "reward", reward, "dryrun", opt.DryRun, "err", err)
			return fmt.Errorf("[sendRewards] send tx failed")
		}
		rewardsSended.Add(rewardsSended, reward)
		_ = opt.WriteSendRewardResult(account, reward, txHash)
	}

	log.Info("[sendRewards] rewards sended", "totalRewards", totalRewards, "rewardsSended", rewardsSended, "allRewardsSended", rewardsSended.Cmp(totalRewards) == 0)
	return nil
}
