package params

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
)

var config *Config

// Config config
type Config struct {
	MongoDB    *MongoDBConfig
	Sync       *SyncConfig
	Distribute *DistributeConfig
}

// MongoDBConfig mongodb config
type MongoDBConfig struct {
	DBURL    string
	DBName   string
	UserName string `json:"-"`
	Password string `json:"-"`
}

// SyncConfig sync config
type SyncConfig struct {
	ServerURL    string
	Overwrite    bool
	JobCount     uint64
	WaitInterval uint64
	Stable       uint64
	Start        uint64
	End          uint64
}

// ExchangeConfig exchange config
type ExchangeConfig struct {
	Exchange   string
	Token      string
	Symbol     string
	Percentage float64
}

// DistributeConfig distribute config
type DistributeConfig struct {
	Exchanges []*ExchangeConfig
}

// GetTokenSymbol get token symbol from config
func GetTokenSymbol(exchange string) string {
	for _, ex := range config.Distribute.Exchanges {
		if strings.EqualFold(ex.Exchange, exchange) {
			return ex.Symbol
		}
	}
	return ""
}

// GetTokenAddress get token symbol from config
func GetTokenAddress(exchange string) string {
	for _, ex := range config.Distribute.Exchanges {
		if strings.EqualFold(ex.Exchange, exchange) {
			return ex.Token
		}
	}
	return ""
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
	var total float64
	for i, ex := range config.Distribute.Exchanges {
		if ex.Exchange == "" {
			return fmt.Errorf("empty exchange address (index %v)", i)
		}
		if ex.Token == "" {
			return fmt.Errorf("empty exchange token (index %v)", i)
		}
		if ex.Symbol == "" {
			return fmt.Errorf("empty exchange symbol (index %v)", i)
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
