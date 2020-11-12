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
			utils.SaveDBFlag,
			utils.DryRunFlag,
			utils.BatchCountFlag,
			utils.BatchIntervalFlag,
			utils.UseTimeMeasurementFlag,
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

	missInputFile := false
	for _, ifile := range opt.InputFiles {
		if ifile == "" {
			missInputFile = true
			break
		}
	}
	if missInputFile {
		syncer.WaitSyncToLatest()
	}

	defer capi.CloseClient()
	return distributer.ByVolume(opt)
}
