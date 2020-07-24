package main

import (
	"fmt"
	"math/big"
	"regexp"

	"github.com/anyswap/ANYToken-distribution/cmd/utils"
	"github.com/anyswap/ANYToken-distribution/distributer"
	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/tools"
	cmath "github.com/fsn-dev/fsn-go-sdk/efsn/common/math"
	"github.com/urfave/cli/v2"
)

func getOptionAndTxArgs(ctx *cli.Context) (*distributer.Option, error) {
	var rewards *big.Int
	if ctx.IsSet(utils.TotalRewardsFlag.Name) {
		rewardsiBig, err := tools.GetBigIntFromString(ctx.String(utils.TotalRewardsFlag.Name))
		if err != nil {
			return nil, err
		}
		rewards = rewardsiBig
	}

	args, err := getBuildTxArgs(ctx)
	if err != nil {
		return nil, err
	}

	sampleHeights, err := getSampleHeights(ctx)
	if err != nil {
		return nil, err
	}

	opt := &distributer.Option{
		BuildTxArgs:  args,
		RewardToken:  ctx.String(utils.RewardTokenFlag.Name),
		TotalValue:   rewards,
		StartHeight:  ctx.Uint64(utils.StartHeightFlag.Name),
		EndHeight:    ctx.Uint64(utils.EndHeightFlag.Name),
		StableHeight: ctx.Uint64(utils.StableHeightFlag.Name),
		StepCount:    ctx.Uint64(utils.StepCountFlag.Name),
		Exchange:     ctx.String(utils.ExchangeFlag.Name),
		InputFile:    getInputFile(ctx),
		OutputFile:   ctx.String(utils.OutputFileFlag.Name),
		Heights:      sampleHeights,
		SaveDB:       ctx.Bool(utils.SaveDBFlag.Name),
		DryRun:       ctx.Bool(utils.DryRunFlag.Name),
	}

	log.Println("get option success.", tools.ToJSONString(opt, !log.JSONFormat))
	return opt, nil
}

func getSampleHeights(ctx *cli.Context) ([]uint64, error) {
	heightsStr := ctx.String(utils.HeightsFlag.Name)
	re := regexp.MustCompile(`[\s,]+`) // blank or comma separated
	parts := re.Split(heightsStr, -1)
	heights := make([]uint64, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		height, ok := cmath.ParseUint64(part)
		if !ok {
			return nil, fmt.Errorf("invalid height '%v' in heights '%v'", part, heightsStr)
		}
		heights = append(heights, height)
	}
	return heights, nil
}

func getBuildTxArgs(ctx *cli.Context) (*distributer.BuildTxArgs, error) {
	var (
		gasPrice    *big.Int
		gasLimitPtr *uint64
		noncePtr    *uint64
	)

	if ctx.IsSet(utils.GasPriceFlag.Name) {
		gasPriceBig, errf := tools.GetBigIntFromString(ctx.String(utils.GasPriceFlag.Name))
		if errf != nil {
			return nil, errf
		}
		gasPrice = gasPriceBig
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

	args := &distributer.BuildTxArgs{
		Sender:       ctx.String(utils.SenderFlag.Name),
		KeystoreFile: ctx.String(utils.KeyStoreFileFlag.Name),
		PasswordFile: ctx.String(utils.PasswordFileFlag.Name),
		Nonce:        noncePtr,
		GasLimit:     gasLimitPtr,
		GasPrice:     gasPrice,
	}

	dryRun := ctx.Bool(utils.DryRunFlag.Name)
	if err := args.Check(dryRun); err != nil {
		return nil, err
	}

	return args, nil
}

func getInputFile(ctx *cli.Context) string {
	var inputFile string
	switch {
	case ctx.IsSet(utils.AccountsFileFlag.Name):
		inputFile = ctx.String(utils.AccountsFileFlag.Name)
	case ctx.IsSet(utils.VolumesFileFlag.Name):
		inputFile = ctx.String(utils.VolumesFileFlag.Name)
	case ctx.IsSet(utils.InputFileFlag.Name):
		inputFile = ctx.String(utils.InputFileFlag.Name)
	}
	return inputFile
}
