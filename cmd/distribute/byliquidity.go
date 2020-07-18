package main

import (
	"math/big"

	"github.com/anyswap/ANYToken-distribution/cmd/utils"
	"github.com/anyswap/ANYToken-distribution/distributer"
	"github.com/anyswap/ANYToken-distribution/tools"
	"github.com/urfave/cli/v2"
)

var (
	byLiquidityCommand = &cli.Command{
		Action:    byLiquidity,
		Name:      "byliquid",
		Usage:     "distribute rewards by liquidity",
		ArgsUsage: " ",
		Description: `
distribute rewards by liquidity
`,
		Flags: []cli.Flag{
			utils.RewardTokenFlag,
			utils.TotalRewardsFlag,
			utils.StartHeightFlag,
			utils.EndHeightFlag,
			utils.ExchangeFlag,
			utils.AccountsFileFlag,
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

func byLiquidity(ctx *cli.Context) error {
	capi := utils.InitApp(ctx, true)
	defer capi.CloseClient()
	distributer.SetAPICaller(capi)

	opt, err := getOptionAndTxArgs(ctx)
	if err != nil {
		return err
	}

	return distributer.ByLiquidity(opt)
}

func getOptionAndTxArgs(ctx *cli.Context) (*distributer.Option, error) {
	var (
		rewards     *big.Int
		gasPrice    *big.Int
		gasLimitPtr *uint64
		noncePtr    *uint64
		err         error
	)

	if ctx.IsSet(utils.TotalRewardsFlag.Name) {
		rewards, err = tools.GetBigIntFromString(ctx.String(utils.TotalRewardsFlag.Name))
		if err != nil {
			return nil, err
		}
	}

	if ctx.IsSet(utils.GasPriceFlag.Name) {
		gasPrice, err = tools.GetBigIntFromString(ctx.String(utils.GasPriceFlag.Name))
		if err != nil {
			return nil, err
		}
	}

	if ctx.IsSet(utils.GasLimitFlag.Name) {
		gasLimitBig, errf := tools.GetBigIntFromString(ctx.String(utils.GasLimitFlag.Name))
		if errf != nil {
			return nil, errf
		}
		gasLimit := gasLimitBig.Uint64()
		gasLimitPtr = &gasLimit
	}

	if ctx.IsSet(utils.AccountNonceFlag.Name) {
		nonceBig, errf := tools.GetBigIntFromString(ctx.String(utils.AccountNonceFlag.Name))
		if errf != nil {
			return nil, errf
		}
		nonce := nonceBig.Uint64()
		noncePtr = &nonce
	}

	var inputFile string
	switch {
	case ctx.IsSet(utils.AccountsFileFlag.Name):
		inputFile = ctx.String(utils.AccountsFileFlag.Name)
	case ctx.IsSet(utils.VolumesFileFlag.Name):
		inputFile = ctx.String(utils.VolumesFileFlag.Name)
	case ctx.IsSet(utils.InputFileFlag.Name):
		inputFile = ctx.String(utils.InputFileFlag.Name)
	}

	args := &distributer.BuildTxArgs{
		KeystoreFile: ctx.String(utils.KeyStoreFileFlag.Name),
		PasswordFile: ctx.String(utils.PasswordFileFlag.Name),
		Nonce:        noncePtr,
		GasLimit:     gasLimitPtr,
		GasPrice:     gasPrice,
	}

	err = args.Check()
	if err != nil {
		return nil, err
	}

	opt := &distributer.Option{
		BuildTxArgs: args,
		RewardToken: ctx.String(utils.RewardTokenFlag.Name),
		TotalValue:  rewards,
		StartHeight: ctx.Uint64(utils.StartHeightFlag.Name),
		EndHeight:   ctx.Uint64(utils.EndHeightFlag.Name),
		Exchange:    ctx.String(utils.ExchangeFlag.Name),
		InputFile:   inputFile,
		OutputFile:  ctx.String(utils.OutputFileFlag.Name),
		DryRun:      ctx.Bool(utils.DryRunFlag.Name),
	}

	return opt, nil
}
