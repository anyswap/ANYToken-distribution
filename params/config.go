package params

import (
	"math"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/tools"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
)

const defaultBlockTime uint64 = 13

var config *Config

// Config config
type Config struct {
	MongoDB    *MongoDBConfig
	Gateway    *GatewayConfig
	Sync       *SyncConfig
	Distribute []*DistributeConfig
	Exchanges  []*ExchangeConfig
}

// MongoDBConfig mongodb config
type MongoDBConfig struct {
	DBURL    string
	DBName   string
	UserName string `json:"-"`
	Password string `json:"-"`
}

// GatewayConfig struct
type GatewayConfig struct {
	APIAddress       string
	AverageBlockTime uint64
}

// GetAverageBlockTime average block time
func GetAverageBlockTime() uint64 {
	avg := config.Gateway.AverageBlockTime
	if avg == 0 {
		avg = defaultBlockTime
	}
	return avg
}

// SyncConfig sync config
type SyncConfig struct {
	JobCount        uint64
	WaitInterval    uint64
	Stable          uint64
	UpdateLiquidity bool
	UpdateVolume    bool
}

// ExchangeConfig exchange config
type ExchangeConfig struct {
	Pairs          string
	Exchange       string
	Token          string
	Percentage     float64
	CreationHeight uint64
}

// DistributeConfig distribute config
type DistributeConfig struct {
	Enable       bool
	DryRun       bool
	Exchange     string
	RewardToken  string
	StartHeight  uint64
	StableHeight uint64
	GasLimit     uint64
	GasPrice     string

	AddNodeRewards     string
	AddNoVolumeRewards string

	ByLiquidCycle        uint64
	ByLiquidRewards      string // unit Wei
	ByLiquidKeystoreFile string
	ByLiquidPasswordFile string

	ByVolumeCycle        uint64
	ByVolumeRewards      string // unit Wei
	ByVolumeKeystoreFile string
	ByVolumePasswordFile string
}

// IsConfigedExchange return true if exchange is configed
func IsConfigedExchange(exchange string) bool {
	return GetExchangePairs(exchange) != ""
}

// IsConfigedToken return true if token is configed
func IsConfigedToken(token string) bool {
	return GetConfigedExchange(token) != ""
}

// GetConfigedExchange get configed exchange
func GetConfigedExchange(token string) string {
	for _, ex := range config.Exchanges {
		if strings.EqualFold(ex.Token, token) {
			return ex.Exchange
		}
	}
	return ""
}

// GetExchangePairs get pairs from config
func GetExchangePairs(exchange string) string {
	for _, ex := range config.Exchanges {
		if strings.EqualFold(ex.Exchange, exchange) {
			return ex.Pairs
		}
	}
	return ""
}

// GetTokenAddress get token address from config
func GetTokenAddress(exchange string) string {
	for _, ex := range config.Exchanges {
		if strings.EqualFold(ex.Exchange, exchange) {
			return ex.Token
		}
	}
	return ""
}

// GetMinExchangeCreationHeight get minimum exchange creation height
func GetMinExchangeCreationHeight() uint64 {
	minHeight := uint64(math.MaxUint64)
	for _, ex := range config.Exchanges {
		if ex.CreationHeight < minHeight {
			minHeight = ex.CreationHeight
		}
	}
	return minHeight
}

// GetConfig get config items structure
func GetConfig() *Config {
	return config
}

// SetConfig set config items
func SetConfig(cfg *Config) {
	config = cfg
}

// LoadConfig load config
func LoadConfig(configFile string) *Config {
	log.Println("Config file is", configFile)
	if !common.FileExist(configFile) {
		log.Fatalf("LoadConfig error: config file %v not exist", configFile)
	}

	config := &Config{}
	if _, err := toml.DecodeFile(configFile, &config); err != nil {
		log.Fatalf("LoadConfig error (toml DecodeFile): %v", err)
	}

	SetConfig(config)

	log.Println("LoadConfig finished.", tools.ToJSONString(config, !log.JSONFormat))

	if err := CheckConfig(); err != nil {
		log.Fatalf("Check config failed. %v", err)
	}
	return config
}
