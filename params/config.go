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

var (
	config = &Config{}

	factoryAddresses []common.Address
	routerAddresses  []common.Address
)

// all exchanges and tokens of configed factories
var (
	AllExchanges = make(map[common.Address]struct{})
	AllTokens    = make(map[common.Address]struct{})
)

// Config config
type Config struct {
	MongoDB    *MongoDBConfig
	Gateway    *GatewayConfig
	Sync       *SyncConfig
	Distribute *DistributeConfig
	Exchanges  []*ExchangeConfig
	Factories  []string
	Stake      *StakeConfig
	Routers    []string // for exchange v2
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

// StakeConfig struct
type StakeConfig struct {
	Contract string
	Points   []uint64 // whole unit
	Percents []uint64
	Stakers  []string

	stakersMap map[common.Address]struct{}
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
	Enable      bool
	ArchiveMode bool

	RewardToken  string
	StartHeight  uint64
	StableHeight uint64
	GasLimit     uint64
	GasPrice     string

	ByLiquidCycle   uint64
	ByLiquidRewards string // unit Wei

	ByVolumeCycle   uint64
	ByVolumeRewards string // unit Wei

	QuickSettleVolumeRewards bool
	DustRewardThreshold      string

	// use time measurement instead of block height
	UseTimeMeasurement    bool
	StartTimestamp        uint64 // unix timestamp
	StableDuration        uint64 // unit of seconds
	ByLiquidCycleDuration uint64 // unit of seconds
	ByVolumeCycleDuration uint64 // unit of seconds

	TradeWeightIsPercentage bool
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

// GetFactories get facotries
func GetFactories() []common.Address {
	if factoryAddresses == nil {
		factories := make([]common.Address, len(config.Factories))
		for i, factory := range config.Factories {
			factories[i] = common.HexToAddress(factory)
		}
		factoryAddresses = factories
	}
	return factoryAddresses
}

// IsConfigedFactory is configed factory
func IsConfigedFactory(factory common.Address) bool {
	for _, fact := range GetFactories() {
		if factory == fact {
			return true
		}
	}
	return false
}

// GetRouters get routers
func GetRouters() []common.Address {
	if routerAddresses == nil {
		routers := make([]common.Address, len(config.Routers))
		for i, router := range config.Routers {
			routers[i] = common.HexToAddress(router)
		}
		routerAddresses = routers
	}
	return routerAddresses
}

// IsConfigedRouter return true if router is configed
func IsConfigedRouter(router string) bool {
	for _, item := range GetRouters() {
		if strings.EqualFold(item.String(), router) {
			return true
		}
	}
	return false
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

	if tmpConfig.Stake != nil {
		tmpConfig.Stake.initStakersMap()
	}
	return tmpConfig
}

// AddTokenAndExchange add token and exchange
func AddTokenAndExchange(token, exchange common.Address) {
	if token == (common.Address{}) || exchange == (common.Address{}) {
		return
	}
	AllTokens[token] = struct{}{}
	AllExchanges[exchange] = struct{}{}
}

// IsInAllTokens is exchange token
func IsInAllTokens(token common.Address) bool {
	_, exist := AllTokens[token]
	return exist
}

// IsInAllExchanges is in all exchanges
func IsInAllExchanges(exchange common.Address) bool {
	_, exist := AllExchanges[exchange]
	return exist
}

// IsInAllTokenAndExchanges is in all exchanges or tokens
func IsInAllTokenAndExchanges(address common.Address) bool {
	return IsInAllTokens(address) || IsInAllExchanges(address)
}

// IsExcludedRewardAccount is excluded
func IsExcludedRewardAccount(account common.Address) bool {
	if account == (common.Address{}) {
		return true
	}
	if IsConfigedExchange(account.String()) {
		return true
	}
	return IsInAllExchanges(account)
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

// IsInStakerList in in staker list
func IsInStakerList(account common.Address) bool {
	if config.Stake == nil {
		return false
	}
	_, exist := config.Stake.stakersMap[account]
	return exist
}

func (s *StakeConfig) initStakersMap() {
	s.stakersMap = make(map[common.Address]struct{}, len(s.Stakers))
	for _, staker := range s.Stakers {
		s.stakersMap[common.HexToAddress(staker)] = struct{}{}
	}
}
