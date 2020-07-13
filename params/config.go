package params

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
)

const defaultBlockTime uint64 = 13

var config *Config

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
	Enable bool
}

// IsConfigedExchange return true if exchange is configed
func IsConfigedExchange(exchange string) bool {
	return GetExchangePairs(exchange) != ""
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

// CheckConfig check config
func CheckConfig() (err error) {
	switch {
	case config == nil:
		return errors.New("empty config")
	case config.MongoDB == nil:
		return errors.New("must config MongoDB")
	case config.Gateway == nil:
		return errors.New("must config Gateway")
	case config.Sync == nil:
		return errors.New("must config Sync")
	case config.Distribute == nil:
		return errors.New("must config Distribute")
	case config.Exchanges == nil:
		return errors.New("must config Exchanges")
	}

	var total float64
	for i, ex := range config.Exchanges {
		if !common.IsHexAddress(ex.Exchange) {
			return fmt.Errorf("wrong exchange address %v (index %v)", ex.Exchange, i)
		}
		if ex.Pairs == "" {
			return fmt.Errorf("empty exchange pairs (index %v)", i)
		}
		if ex.Token == "" {
			return fmt.Errorf("empty exchange token (index %v)", i)
		}
		if ex.CreationHeight == 0 {
			return fmt.Errorf("empty exchange creation height (index %v)", i)
		}
		total += ex.Percentage
	}
	if math.Abs(total-100) > 1e-18 {
		return fmt.Errorf("total percentage %v is not 100%%", total)
	}
	return nil
}

// LoadConfig load config
func LoadConfig(configFile string) *Config {
	log.Println("Config file is", configFile)
	if !common.FileExist(configFile) {
		panic(fmt.Sprintf("LoadConfig error: config file %v not exist", configFile))
	}

	config := &Config{}
	if _, err := toml.DecodeFile(configFile, &config); err != nil {
		panic(fmt.Sprintf("LoadConfig error (toml DecodeFile): %v", err))
	}

	SetConfig(config)

	var bs []byte
	if log.JSONFormat {
		bs, _ = json.Marshal(config)
	} else {
		bs, _ = json.MarshalIndent(config, "", "  ")
	}
	log.Println("LoadConfig finished.", string(bs))

	if err := CheckConfig(); err != nil {
		panic(fmt.Sprintf("Check config failed. %v", err))
	}
	return config
}
