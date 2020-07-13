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

func byLiquidity(ctx *cli.Context) (err error) {
	capi := utils.InitApp(ctx, true)
	defer capi.CloseClient()
	distributer.SetAPICaller(capi)

	var (
		rewards     *big.Int
		gasPrice    *big.Int
		gasLimitPtr *uint64
		noncePtr    *uint64
	)

	rewards, err = tools.GetBigIntFromString(ctx.String(utils.TotalRewardsFlag.Name))
	if err != nil {
		return err
	}

	if ctx.IsSet(utils.GasPriceFlag.Name) {
		gasPrice, err = tools.GetBigIntFromString(ctx.String(utils.GasPriceFlag.Name))
		if err != nil {
			return err
		}
	}

	if ctx.IsSet(utils.GasLimitFlag.Name) {
		gasLimitBig, errf := tools.GetBigIntFromString(ctx.String(utils.GasLimitFlag.Name))
		if errf != nil {
			return errf
		}
		gasLimit := gasLimitBig.Uint64()
		gasLimitPtr = &gasLimit
	}

	if ctx.IsSet(utils.AccountNonceFlag.Name) {
		nonceBig, errf := tools.GetBigIntFromString(ctx.String(utils.AccountNonceFlag.Name))
		if errf != nil {
			return errf
		}
		nonce := nonceBig.Uint64()
		noncePtr = &nonce
	}

	opt := &distributer.Option{
		TotalValue:   rewards,
		StartHeight:  ctx.Uint64(utils.StartHeightFlag.Name),
		EndHeight:    ctx.Uint64(utils.EndHeightFlag.Name),
		Exchange:     ctx.String(utils.ExchangeFlag.Name),
		AccountsFile: ctx.String(utils.AccountsFileFlag.Name),
		OutputFile:   ctx.String(utils.OutputFileFlag.Name),
		DryRun:       ctx.Bool(utils.DryRunFlag.Name),
	}

	args := &distributer.BuildTxArgs{
		KeystoreFile: ctx.String(utils.KeyStoreFileFlag.Name),
		PasswordFile: ctx.String(utils.PasswordFileFlag.Name),
		Nonce:        noncePtr,
		GasLimit:     gasLimitPtr,
		GasPrice:     gasPrice,
	}
	distributer.InitBuildTxArgs(args)

	distributer.ByLiquidity(opt)
	return nil
}
