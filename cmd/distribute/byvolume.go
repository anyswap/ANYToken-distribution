package main

import (
	"github.com/anyswap/ANYToken-distribution/cmd/utils"
	"github.com/anyswap/ANYToken-distribution/distributer"
	"github.com/urfave/cli/v2"
)

var (
	byVolumeCommand = &cli.Command{
		Action:    byVolume,
		Name:      "byvolume",
		Usage:     "distribute rewards by volume",
		ArgsUsage: " ",
		Description: `
distribute rewards by volume
`,
		Flags: []cli.Flag{
			utils.RewardTokenFlag,
			utils.TotalRewardsFlag,
			utils.StartHeightFlag,
			utils.EndHeightFlag,
			utils.ExchangeFlag,
			utils.VolumesFileFlag,
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

func byVolume(ctx *cli.Context) (err error) {
	capi := utils.InitApp(ctx, true)
	defer capi.CloseClient()
	distributer.SetAPICaller(capi)

	opt, args, err := getOptionAndTxArgs(ctx)
	if err != nil {
		return err
	}

	return distributer.ByVolume(opt, args)
}
