package utils

import (
	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/urfave/cli/v2"
)

var (
	// DataDirFlag --datadir
	DataDirFlag = &cli.StringFlag{
		Name:  "datadir",
		Usage: "Data directory (default in the execute directory)",
		Value: "",
	}
	// ConfigFileFlag -c|--config
	ConfigFileFlag = &cli.StringFlag{
		Name:    "config",
		Aliases: []string{"c"},
		Usage:   "config file, use toml format",
	}
	// LogFileFlag --log
	LogFileFlag = &cli.StringFlag{
		Name:  "log",
		Usage: "log file, support rotate",
	}
	// LogRotationFlag --rotate
	LogRotationFlag = &cli.Uint64Flag{
		Name:  "rotate",
		Usage: "log rotation time (unit hour)",
		Value: 24,
	}
	// LogMaxAgeFlag --maxage
	LogMaxAgeFlag = &cli.Uint64Flag{
		Name:  "maxage",
		Usage: "log max age (unit hour)",
		Value: 720,
	}
	// VerbosityFlag -v|--verbosity
	VerbosityFlag = &cli.Uint64Flag{
		Name:    "verbosity",
		Aliases: []string{"v"},
		Usage:   "log verbosity (0:panic, 1:fatal, 2:error, 3:warn, 4:info, 5:debug, 6:trace)",
		Value:   4,
	}
	// JSONFormatFlag --json
	JSONFormatFlag = &cli.BoolFlag{
		Name:  "json",
		Usage: "output log in json format",
	}
	// ColorFormatFlag --color
	ColorFormatFlag = &cli.BoolFlag{
		Name:  "color",
		Usage: "output log in color text format",
		Value: true,
	}

	// SyncFromFlag --syncfrom
	SyncFromFlag = &cli.Uint64Flag{
		Name:  "syncfrom",
		Usage: "sync start height, 0 means read from database",
		Value: 0,
	}
	// SyncToFlag --syncto
	SyncToFlag = &cli.Uint64Flag{
		Name:  "syncto",
		Usage: "sync end height (excluding end), 0 means endless",
		Value: 0,
	}
	// OverwriteFlag --overwrite
	OverwriteFlag = &cli.BoolFlag{
		Name:  "overwrite",
		Usage: "overwrite exist items in database",
	}

	// KeyStoreFileFlag --keystore
	KeyStoreFileFlag = &cli.StringFlag{
		Name:  "keystore",
		Usage: "keystore file path",
	}
	// PasswordFileFlag --password
	PasswordFileFlag = &cli.StringFlag{
		Name:  "password",
		Usage: "password file path",
	}
	// GasLimitFlag --gas
	GasLimitFlag = &cli.StringFlag{
		Name:  "gasLimit",
		Usage: "gas limit in transaction, use default if not specified",
	}
	// GasPriceFlag --gasPrice
	GasPriceFlag = &cli.StringFlag{
		Name:  "gasPrice",
		Usage: "gas price in transaction, use default if not specified",
	}
	// AccountNonceFlag --nonce
	AccountNonceFlag = &cli.StringFlag{
		Name:  "nonce",
		Usage: "nonce in transaction, use default if not specified",
	}

	// RewardTokenFlag --rewardToken
	RewardTokenFlag = &cli.StringFlag{
		Name:  "rewardToken",
		Usage: "reward token",
	}
	// TotalRewardsFlag --rewards
	TotalRewardsFlag = &cli.StringFlag{
		Name:  "rewards",
		Usage: "total rewards (uint wei)",
	}
	// StartHeightFlag --start
	StartHeightFlag = &cli.Uint64Flag{
		Name:  "start",
		Usage: "start height (start inclusive)",
	}
	// EndHeightFlag --end
	EndHeightFlag = &cli.Uint64Flag{
		Name:  "end",
		Usage: "end height (end exclusive)",
	}
	// StableHeightFlag --stable
	StableHeightFlag = &cli.Uint64Flag{
		Name:  "stable",
		Usage: "stable height",
		Value: 30,
	}
	// StepCountFlag --step
	StepCountFlag = &cli.Uint64Flag{
		Name:  "step",
		Usage: "step count",
		Value: 100,
	}
	// StepRewardFlag --stepReward
	StepRewardFlag = &cli.StringFlag{
		Name:  "stepReward",
		Usage: "step reward",
		Value: "250000000000000000000",
	}
	// ExchangeFlag --exchange
	ExchangeFlag = &cli.StringFlag{
		Name:  "exchange",
		Usage: "exchange address",
	}
	// AccountsFileFlag --accounts
	AccountsFileFlag = &cli.StringFlag{
		Name:  "accounts",
		Usage: "accounts file (line format: <address>), read from database if not specified",
	}
	// VolumesFileFlag --volumes
	VolumesFileFlag = &cli.StringFlag{
		Name:  "volumes",
		Usage: "volumes file (line format: <address> <volume>), read from database if not specified",
	}
	// InputFileFlag --input
	InputFileFlag = &cli.StringFlag{
		Name:  "input",
		Usage: "input file",
	}
	// OutputFileFlag --output
	OutputFileFlag = &cli.StringFlag{
		Name:  "output",
		Usage: "output file of result",
	}
	// DryRunFlag --dryrun
	DryRunFlag = &cli.BoolFlag{
		Name:  "dryrun",
		Usage: "dry run",
	}
	// GatewayFlag --gateway
	GatewayFlag = &cli.StringFlag{
		Name:  "gateway",
		Usage: "gateway URL address",
	}
	// SenderFlag --sender
	SenderFlag = &cli.StringFlag{
		Name:  "sender",
		Usage: "specify transaction's sender",
	}
	// SaveDBFlag --savedb
	SaveDBFlag = &cli.BoolFlag{
		Name:  "savedb",
		Usage: "save result to database",
	}
	// HeightsFlag --heights
	HeightsFlag = &cli.StringFlag{
		Name:  "heights",
		Usage: "comma separated block heights",
	}
)

// SyncArguments command line arguments
type SyncArguments struct {
	SyncStartHeight *uint64
	SyncEndHeight   *uint64
	SyncOverwrite   *bool
}

// SyncArgs sync arguments
var SyncArgs SyncArguments

// SetLogger set log level, json format, color, rotate ...
func SetLogger(ctx *cli.Context) {
	logLevel := ctx.Uint64(VerbosityFlag.Name)
	jsonFormat := ctx.Bool(JSONFormatFlag.Name)
	colorFormat := ctx.Bool(ColorFormatFlag.Name)
	log.SetLogger(uint32(logLevel), jsonFormat, colorFormat)

	logFile := ctx.String(LogFileFlag.Name)
	if logFile != "" {
		logRotation := ctx.Uint64(LogRotationFlag.Name)
		logMaxAge := ctx.Uint64(LogMaxAgeFlag.Name)
		log.SetLogFile(logFile, logRotation, logMaxAge)
	}
}

// GetConfigFilePath specified by `-c|--config`
func GetConfigFilePath(ctx *cli.Context) string {
	return ctx.String(ConfigFileFlag.Name)
}

// InitSyncArguments init sync arguments
func InitSyncArguments(ctx *cli.Context) {
	if ctx.IsSet(SyncFromFlag.Name) {
		start := ctx.Uint64(SyncFromFlag.Name)
		SyncArgs.SyncStartHeight = &start
	}
	if ctx.IsSet(SyncToFlag.Name) {
		end := ctx.Uint64(SyncToFlag.Name)
		SyncArgs.SyncEndHeight = &end
	}
	if ctx.IsSet(OverwriteFlag.Name) {
		overwrite := ctx.Bool(OverwriteFlag.Name)
		SyncArgs.SyncOverwrite = &overwrite
	}
}
