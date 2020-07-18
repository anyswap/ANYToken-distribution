package main

import (
	"github.com/anyswap/ANYToken-distribution/cmd/utils"
	"github.com/anyswap/ANYToken-distribution/distributer"
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
