package main

import (
	"github.com/anyswap/ANYToken-distribution/cmd/utils"
	"github.com/anyswap/ANYToken-distribution/distributer"
	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/urfave/cli/v2"
)

var (
	calcRewardsCommand = &cli.Command{
		Action:    calcRewards,
		Name:      "calcRewards",
		Usage:     "calculate rewards by config file",
		ArgsUsage: " ",
		Description: `
calculate rewards by config file
`,
		Flags: []cli.Flag{
			utils.StartHeightFlag,
			utils.EndHeightFlag,
			utils.SampleFlag,
			utils.InputFileSliceFlag,
			calcTypeFlag,
		},
	}

	calcTypeFlag = &cli.StringFlag{
		Name:  "type",
		Usage: "type value can be liquid, volume, both",
		Value: "both",
	}
)

func calcRewards(ctx *cli.Context) error {
	configFile := utils.GetConfigFilePath(ctx)
	if configFile == "" {
		log.Fatal("calcRewards: must specify config file path")
	}

	startHeight := ctx.Uint64(utils.StartHeightFlag.Name)
	endHeight := ctx.Uint64(utils.EndHeightFlag.Name)
	sampleHeight := ctx.Uint64(utils.SampleFlag.Name)
	if startHeight >= endHeight {
		log.Fatal("calcRewards: start height should be lower than end height", "start", startHeight, "end", endHeight)
	}
	if sampleHeight != 0 && !(sampleHeight >= startHeight && sampleHeight < endHeight) {
		log.Fatal("calcRewards: sample not in the range of start to end", "start", startHeight, "end", endHeight, "sample", sampleHeight)
	}

	calcType := ctx.String(calcTypeFlag.Name)
	switch calcType {
	case distributer.CalcBothRewards,
		distributer.CalcVolumeRewards,
		distributer.CalcLiquidRewards:
	default:
		log.Fatal("calcRewards: wrong type %v", calcType)
	}

	capi := utils.InitApp(ctx, true)
	distributer.SetAPICaller(capi)
	defer capi.CloseClient()

	inputs := ctx.StringSlice(utils.InputFileSliceFlag.Name)

	return distributer.CalcRewards(startHeight, endHeight, sampleHeight, calcType, inputs)
}
