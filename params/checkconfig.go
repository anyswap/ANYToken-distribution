package params

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/anyswap/ANYToken-distribution/tools"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
)

var maxDistributeStableHeight uint64

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
	// for security reason, if has distribute job, then
	// must sync with at least the distribute job's stable height
	// to prevent blockchain short forks
	if maxDistributeStableHeight > config.Sync.Stable {
		config.Sync.Stable = maxDistributeStableHeight
	}
	return nil
}

func checkExchangeConfig() error {
	for _, ex := range config.Exchanges {
		if err := ex.check(); err != nil {
			return err
		}
	}
	return nil
}

func checkDistributeConfig() error {
	for _, dist := range config.Distribute {
		if !dist.Enable {
			continue
		}
		if err := dist.check(); err != nil {
			return err
		}
	}
	return nil
}

func (ex *ExchangeConfig) check() error {
	if !common.IsHexAddress(ex.Exchange) {
		return fmt.Errorf("[check exchange] wrong exchange address '%v'", ex.Exchange)
	}
	if ex.Pairs == "" {
		return fmt.Errorf("[check exchange] empty exchange pairs (exchange %v)", ex.Exchange)
	}
	if !common.IsHexAddress(ex.Token) {
		return fmt.Errorf("[check exchange] wrong exchange token '%v' (exchange %v)", ex.Token, ex.Exchange)
	}
	if ex.CreationHeight == 0 {
		return fmt.Errorf("[check exchange] wrong exchange creation height '%v' (exchange %v)", ex.CreationHeight, ex.Exchange)
	}
	return nil
}

func (dist *DistributeConfig) check() error {
	if err := dist.checkAddress(); err != nil {
		return err
	}
	if err := dist.checkStringValue(); err != nil {
		return err
	}
	if err := dist.checkCycle(); err != nil {
		return err
	}
	if dist.StableHeight > maxDistributeStableHeight {
		maxDistributeStableHeight = dist.StableHeight
	}
	return nil
}

func (dist *DistributeConfig) checkAddress() error {
	if !common.IsHexAddress(dist.Exchange) {
		return fmt.Errorf("[check distribute] wrong exchange address %v", dist.Exchange)
	}
	if !IsConfigedExchange(dist.Exchange) {
		return fmt.Errorf("[check distribute] exchange %v is not configed with pairs", dist.Exchange)
	}
	if !common.IsHexAddress(dist.RewardToken) {
		return fmt.Errorf("[check distribute] wrong reward token address %v (exchange %v)", dist.RewardToken, dist.Exchange)
	}
	return nil
}

func (dist *DistributeConfig) checkBigIntStringValue(name, value string) error {
	if value == "" {
		return nil
	}
	_, err := tools.GetBigIntFromString(value)
	if err != nil {
		return fmt.Errorf("[check distribute] wrong %v %v (exchange %v)", name, value, dist.Exchange)
	}
	return nil
}

func (dist *DistributeConfig) checkStringValue() error {
	if err := dist.checkBigIntStringValue("add node rewards", dist.AddNodeRewards); err != nil {
		return err
	}
	if err := dist.checkBigIntStringValue("add no volume rewards", dist.AddNoVolumeRewards); err != nil {
		return err
	}
	if err := dist.checkBigIntStringValue("by liquid rewards", dist.ByLiquidRewards); err != nil {
		return err
	}
	if err := dist.checkBigIntStringValue("by volume rewards", dist.ByVolumeRewards); err != nil {
		return err
	}
	if err := dist.checkBigIntStringValue("gas price", dist.GasPrice); err != nil {
		return err
	}
	return nil
}

func (dist *DistributeConfig) checkCycle() error {
	if dist.ByVolumeCycle == 0 {
		return fmt.Errorf("[check distribute] error: zero by volume cycle length")
	}
	if dist.ByLiquidCycle < dist.ByVolumeCycle {
		return fmt.Errorf("[check distribute] error: by liquidity cycle %v < by volume cycle %v", dist.ByLiquidCycle, dist.ByVolumeCycle)
	}
	if dist.ByLiquidCycle%dist.ByVolumeCycle != 0 {
		return fmt.Errorf("[check distribute] error: by liquidity cycle %v is not an integral multiple of by volume cycle %v", dist.ByLiquidCycle, dist.ByVolumeCycle)
	}
	return nil
}

// GetByVolumeCycleRewards get non nil big int from string
func (dist *DistributeConfig) GetByVolumeCycleRewards() *big.Int {
	byVolumeRewards, _ := tools.GetBigIntFromString(dist.ByVolumeRewards)
	if byVolumeRewards == nil {
		byVolumeRewards = big.NewInt(0)
	}
	return byVolumeRewards
}

// GetByLiquidCycleRewards get non nil big int from string
func (dist *DistributeConfig) GetByLiquidCycleRewards() *big.Int {
	byLiquidRewards, _ := tools.GetBigIntFromString(dist.ByLiquidRewards)
	if byLiquidRewards == nil {
		byLiquidRewards = big.NewInt(0)
	}
	return byLiquidRewards
}

// GetAddNodeRewards get non nil big int from string
func (dist *DistributeConfig) GetAddNodeRewards() *big.Int {
	addNodeRewards, _ := tools.GetBigIntFromString(dist.AddNodeRewards)
	if addNodeRewards == nil {
		addNodeRewards = big.NewInt(0)
	}
	return addNodeRewards
}

// GetAddNoVolumeRewards get non nil big int from string
func (dist *DistributeConfig) GetAddNoVolumeRewards() *big.Int {
	addNoVolumeRewards, _ := tools.GetBigIntFromString(dist.AddNoVolumeRewards)
	if addNoVolumeRewards == nil {
		addNoVolumeRewards = big.NewInt(0)
	}
	return addNoVolumeRewards
}
