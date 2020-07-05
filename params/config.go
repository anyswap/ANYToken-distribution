package params

import (
	"encoding/json"
	"fmt"

	"github.com/BurntSushi/toml"
	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
)

var config *Config

// Config config
type Config struct {
	MongoDB *MongoDBConfig
	Sync    *SyncConfig
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
