package params

import (
	"errors"
	"fmt"
	"math"

	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
)

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
	err = checkExchangeConfig()
	if err != nil {
		return err
	}
	err = checkDistributeConfig()
	if err != nil {
		return err
	}
	return nil
}

func checkExchangeConfig() (err error) {
	var total float64
	for i, ex := range config.Exchanges {
		if !common.IsHexAddress(ex.Exchange) {
			return fmt.Errorf("[check exchange] wrong exchange address %v (index %v)", ex.Exchange, i)
		}
		if ex.Pairs == "" {
			return fmt.Errorf("[check exchange] empty exchange pairs (index %v)", i)
		}
		if ex.Token == "" {
			return fmt.Errorf("[check exchange] empty exchange token (index %v)", i)
		}
		if ex.CreationHeight == 0 {
			return fmt.Errorf("[check exchange] empty exchange creation height (index %v)", i)
		}
		total += ex.Percentage
	}
	if math.Abs(total-100) > 1e-18 {
		return fmt.Errorf("[check exchange] total percentage %v is not 100%%", total)
	}
	return nil
}

func checkDistributeConfig() (err error) {
	for i, dist := range config.Distribute {
		if !dist.Enable {
			continue
		}
		if !common.IsHexAddress(dist.Exchange) {
			return fmt.Errorf("[check distribute] wrong exchange address %v (index %v)", dist.Exchange, i)
		}
		if GetExchangePairs(dist.Exchange) == "" {
			return fmt.Errorf("[check distribute] exchange %v (index %v) is not configed with pairs", dist.Exchange, i)
		}
		if !common.IsHexAddress(dist.RewardToken) {
			return fmt.Errorf("[check distribute] wrong reward token address %v (index %v)", dist.RewardToken, i)
		}
	}
	return nil
}
