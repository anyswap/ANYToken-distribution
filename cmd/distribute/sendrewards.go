package main

import (
	"fmt"
	"math/big"

	"github.com/anyswap/ANYToken-distribution/cmd/utils"
	"github.com/anyswap/ANYToken-distribution/distributer"
	"github.com/anyswap/ANYToken-distribution/log"
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
			utils.RewardTyepFlag,
			utils.DustRewardFlag,
			utils.ExchangeSliceFlag,
			utils.RewardTokenFlag,
			utils.StartHeightFlag,
			utils.EndHeightFlag,
			utils.InputFileSliceFlag,
			utils.OutputFileSliceFlag,
			utils.SenderFlag,
			utils.KeyStoreFileFlag,
			utils.PasswordFileFlag,
			utils.GasLimitFlag,
			utils.GasPriceFlag,
			utils.AccountNonceFlag,
			utils.SaveDBFlag,
			utils.DryRunFlag,
			utils.BatchCountFlag,
			utils.BatchIntervalFlag,
			utils.ScalingValueFlag,
		},
	}
)

func sendRewards(ctx *cli.Context) error {
	serverURL := ctx.String(utils.GatewayFlag.Name)
	if serverURL == "" {
		return fmt.Errorf("must specify gateway URL")
	}
	rewardType := ctx.String(utils.RewardTyepFlag.Name)
	if rewardType == "" {
		return fmt.Errorf("must specify rewardType")
	}

	withConfigFile := !distributer.IsCustomMethod(rewardType)
	capi := utils.InitAppWithURL(ctx, serverURL, withConfigFile)
	distributer.SetAPICaller(capi)

	opt, err := getOptionAndTxArgs(ctx)
	if err != nil {
		log.Fatalf("get option error: %v", err)
	}

	opt.ScalingNumerator, opt.ScalingDenominator = getScalingValue(ctx.String(utils.ScalingValueFlag.Name))

	defer capi.CloseClient()
	return opt.SendRewardsFromFile()
}

func getScalingValue(scalingStr string) (numerator, denominator *big.Int) {
	if scalingStr == "" {
		return
	}
	parts := blankOrCommaSepRegexp.Split(scalingStr, -1)
	if len(parts) > 2 {
		log.Fatalf("wrong scaling value '%v'", scalingStr)
	}
	var ok bool
	numerator, ok = new(big.Int).SetString(parts[0], 0)
	if !ok {
		log.Fatalf("wrong scaling numerator '%v'", parts[0])
	}
	if len(parts) > 1 {
		denominator, ok = new(big.Int).SetString(parts[1], 0)
		if !ok || denominator.Sign() == 0 {
			log.Fatalf("wrong scaling denominator '%v'", parts[1])
		}
	} else if numerator.Cmp(big.NewInt(1)) == 0 {
		numerator = nil
	}
	if numerator != nil {
		log.Info("get scaling value",
			"scalingStr", scalingStr,
			"numerator", numerator,
			"denominator", denominator,
			"decrease", denominator != nil && numerator.Cmp(denominator) < 0)
	}
	return numerator, denominator
}
