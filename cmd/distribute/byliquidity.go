package main

import (
	"github.com/anyswap/ANYToken-distribution/cmd/utils"
	"github.com/anyswap/ANYToken-distribution/distributer"
	"github.com/anyswap/ANYToken-distribution/log"
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
			utils.StableHeightFlag,
			utils.ExchangeSliceFlag,
			utils.WeightSliceFlag,
			utils.InputFileSliceFlag,
			utils.OutputFileSliceFlag,
			utils.SenderFlag,
			utils.KeyStoreFileFlag,
			utils.PasswordFileFlag,
			utils.GasLimitFlag,
			utils.GasPriceFlag,
			utils.AccountNonceFlag,
			utils.SampleFlag,
			utils.SaveDBFlag,
			utils.DryRunFlag,
			utils.BatchCountFlag,
			utils.BatchIntervalFlag,
			utils.UseTimeMeasurementFlag,
			utils.ArchiveModeFlag,
		},
	}
)

func byLiquidity(ctx *cli.Context) error {
	capi := utils.InitApp(ctx, true)
	distributer.SetAPICaller(capi)

	opt, err := getOptionAndTxArgs(ctx)
	if err != nil {
		log.Fatalf("get option error: %v", err)
	}

	defer capi.CloseClient()
	return distributer.ByLiquidity(opt)
}
