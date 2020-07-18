package main

import (
	"fmt"
	"os"

	"github.com/anyswap/ANYToken-distribution/cmd/utils"
	"github.com/anyswap/ANYToken-distribution/worker"
	"github.com/urfave/cli/v2"
)

var (
	clientIdentifier = "ANYToken distribution"
	// Git SHA1 commit hash of the release (set via linker flags)
	gitCommit = ""
	// The app that holds all commands and flags.
	app = utils.NewApp(clientIdentifier, gitCommit, "the ANYToken distribution command line interface")
)

func initApp() {
	app.Action = distribute
	app.HideVersion = true // we have a command to print the version
	app.Copyright = "Copyright 2017-2020 The Anyswap Authors"
	app.Commands = []*cli.Command{
		byLiquidityCommand,
		byVolumeCommand,
		sendRewardsCommand,
		utils.LicenseCommand,
		utils.VersionCommand,
	}
	app.Flags = []cli.Flag{
		utils.ConfigFileFlag,
		utils.SyncFromFlag,
		utils.SyncToFlag,
		utils.OverwriteFlag,
		utils.VerbosityFlag,
		utils.LogFileFlag,
		utils.LogRotationFlag,
		utils.LogMaxAgeFlag,
		utils.JSONFormatFlag,
		utils.ColorFormatFlag,
	}
}

func main() {
	initApp()
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func distribute(ctx *cli.Context) error {
	if ctx.NArg() > 0 {
		return fmt.Errorf("invalid command: %q", ctx.Args().Get(0))
	}

	capi := utils.InitApp(ctx, true)
	defer capi.CloseClient()

	worker.StartWork(capi)
	return nil
}
