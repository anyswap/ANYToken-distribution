package params

import (
	"math"
	"math/big"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/tools"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
)

const defaultBlockTime uint64 = 13

var config = &Config{}

// Config config
type Config struct {
	MongoDB    *MongoDBConfig
	Gateway    *GatewayConfig
	Sync       *SyncConfig
	Distribute *DistributeConfig
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
	JobCount           uint64
	WaitInterval       uint64
	Stable             uint64
	UpdateLiquidity    bool
	UpdateVolume       bool
	ScanAllExchange    bool
	RecordTokenAccount bool
}

// ExchangeConfig exchange config
type ExchangeConfig struct {
	Pairs          string
	Exchange       string
	Token          string
	CreationHeight uint64
	LiquidWeight   uint64
	TradeWeight    uint64
}

// DistributeConfig distribute config
type DistributeConfig struct {
	Enable       bool
	DryRun       bool
	SaveDB       bool
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

	QuickSettleVolumeRewards bool
	DustRewardThreshold      string
}

// IsScanAllExchange is scan all exchange
func IsScanAllExchange() bool {
	return config.Sync.ScanAllExchange
}

// IsRecordTokenAccount is record token account
func IsRecordTokenAccount() bool {
	return config.Sync.RecordTokenAccount
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

// GetExchangeToken get exchane token from config
func GetExchangeToken(exchange string) string {
	for _, ex := range config.Exchanges {
		if strings.EqualFold(ex.Exchange, exchange) {
			return ex.Token
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
	log.Printf("Config file is '%v'\n", configFile)
	if !common.FileExist(configFile) {
		log.Fatalf("LoadConfig error: config file '%v' not exist", configFile)
	}

	tmpConfig := &Config{}
	if _, err := toml.DecodeFile(configFile, &tmpConfig); err != nil {
		log.Fatalf("LoadConfig error (toml DecodeFile): %v", err)
	}

	SetConfig(tmpConfig)

	log.Println("LoadConfig finished.", tools.ToJSONString(tmpConfig, !log.JSONFormat))

	if err := CheckConfig(); err != nil {
		log.Fatalf("Check config failed. %v", err)
	}
	return tmpConfig
}

// IsExcludedRewardAccount is excluded
func IsExcludedRewardAccount(account common.Address) bool {
	accountStr := strings.ToLower(account.String())
	if IsConfigedExchange(accountStr) {
		return true
	}
	return account == (common.Address{})
}

// GetDustRewardThreshold get dust reward threshold
func GetDustRewardThreshold() *big.Int {
	if config.Distribute != nil {
		return config.Distribute.GetDustRewardThreshold()
	}
	return big.NewInt(0)
}

// SetDustRewardThreshold set dust reward threshold
func SetDustRewardThreshold(dustThreshold string) {
	if config.Distribute == nil {
		config.Distribute = &DistributeConfig{}
	}
	config.Distribute.DustRewardThreshold = dustThreshold
}
