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

	opt, args, err := getOptionAndTxArgs(ctx)
	if err != nil {
		return err
	}

	return distributer.ByLiquidity(opt, args)
}

func getOptionAndTxArgs(ctx *cli.Context) (*distributer.Option, *distributer.BuildTxArgs, error) {
	var (
		rewards     *big.Int
		gasPrice    *big.Int
		gasLimitPtr *uint64
		noncePtr    *uint64
		err         error
	)

	rewards, err = tools.GetBigIntFromString(ctx.String(utils.TotalRewardsFlag.Name))
	if err != nil {
		return nil, nil, err
	}

	if ctx.IsSet(utils.GasPriceFlag.Name) {
		gasPrice, err = tools.GetBigIntFromString(ctx.String(utils.GasPriceFlag.Name))
		if err != nil {
			return nil, nil, err
		}
	}

	if ctx.IsSet(utils.GasLimitFlag.Name) {
		gasLimitBig, errf := tools.GetBigIntFromString(ctx.String(utils.GasLimitFlag.Name))
		if errf != nil {
			return nil, nil, errf
		}
		gasLimit := gasLimitBig.Uint64()
		gasLimitPtr = &gasLimit
	}

	if ctx.IsSet(utils.AccountNonceFlag.Name) {
		nonceBig, errf := tools.GetBigIntFromString(ctx.String(utils.AccountNonceFlag.Name))
		if errf != nil {
			return nil, nil, errf
		}
		nonce := nonceBig.Uint64()
		noncePtr = &nonce
	}

	var inputFile string
	if ctx.IsSet(utils.AccountsFileFlag.Name) {
		inputFile = ctx.String(utils.AccountsFileFlag.Name)
	} else if ctx.IsSet(utils.VolumesFileFlag.Name) {
		inputFile = ctx.String(utils.VolumesFileFlag.Name)
	}

	opt := &distributer.Option{
		RewardToken: ctx.String(utils.RewardTokenFlag.Name),
		TotalValue:  rewards,
		StartHeight: ctx.Uint64(utils.StartHeightFlag.Name),
		EndHeight:   ctx.Uint64(utils.EndHeightFlag.Name),
		Exchange:    ctx.String(utils.ExchangeFlag.Name),
		InputFile:   inputFile,
		OutputFile:  ctx.String(utils.OutputFileFlag.Name),
		DryRun:      ctx.Bool(utils.DryRunFlag.Name),
	}

	args := &distributer.BuildTxArgs{
		KeystoreFile: ctx.String(utils.KeyStoreFileFlag.Name),
		PasswordFile: ctx.String(utils.PasswordFileFlag.Name),
		Nonce:        noncePtr,
		GasLimit:     gasLimitPtr,
		GasPrice:     gasPrice,
	}

	return opt, args, nil
}
