package main

import (
	"github.com/anyswap/ANYToken-distribution/cmd/utils"
	"github.com/anyswap/ANYToken-distribution/distributer"
	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/syncer"
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
			utils.StableHeightFlag,
			utils.StepCountFlag,
			utils.StepRewardFlag,
			utils.ExchangeFlag,
			utils.VolumesFileFlag,
			utils.SenderFlag,
			utils.KeyStoreFileFlag,
			utils.PasswordFileFlag,
			utils.GasLimitFlag,
			utils.GasPriceFlag,
			utils.AccountNonceFlag,
			utils.OutputFileFlag,
			utils.SaveDBFlag,
			utils.DryRunFlag,
		},
	}
)

func byVolume(ctx *cli.Context) (err error) {
	capi := utils.InitApp(ctx, true)
	distributer.SetAPICaller(capi)

	opt, err := getOptionAndTxArgs(ctx)
	if err != nil {
		log.Fatalf("get option error: %v", err)
	}

	if opt.InputFile != "" {
		syncer.WaitSyncToLatest()
	}

	defer capi.CloseClient()
	return distributer.ByVolume(opt)
}
