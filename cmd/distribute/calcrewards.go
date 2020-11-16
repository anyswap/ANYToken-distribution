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
			utils.HeightsFlag,
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
	if startHeight >= endHeight {
		log.Fatal("calcRewards: start height should be lower than end height")
	}

	sampleHeights, err := getSampleHeights(ctx)
	if err != nil {
		log.Fatal("calcRewards: get sample heights failed", "err", err)
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

	return distributer.CalcRewards(startHeight, endHeight, sampleHeights, calcType)
}
