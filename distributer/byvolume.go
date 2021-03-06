package distributer

import (
	"math/big"

	"github.com/anyswap/ANYToken-distribution/log"
	"github.com/anyswap/ANYToken-distribution/mongodb"
	"github.com/anyswap/ANYToken-distribution/params"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
)

// ByVolume distribute rewards by vloume
func ByVolume(opt *Option) error {
	opt.byWhat = byVolumeMethodID
	log.Info("[byvolume] start", "option", opt.String())
	if opt.TotalValue == nil || opt.TotalValue.Sign() <= 0 {
		log.Warn("no volume rewards", "option", opt.String())
		return errTotalRewardsIsZero
	}
	err := opt.checkAndInit()
	defer opt.deinit()
	if err != nil {
		log.Error("[byvolume] check option error", "option", opt.String(), "err", err)
		return errCheckOptionFailed
	}
	accountStats, err := opt.GetAccountsAndRewards()
	if err != nil {
		log.Error("[byvolume] GetAccountsAndRewards error", "err", err)
		return errGetAccountsRewardsFailed
	}
	if len(accountStats) != len(opt.Exchanges) {
		log.Warn("[byvolume] account list is not complete. " + opt.String())
		return errAccountsNotComplete
	}
	totalReward := opt.TotalValue
	if opt.noVolumes > 0 && opt.StepReward.Sign() > 0 {
		subReward := new(big.Int).Mul(opt.StepReward, new(big.Int).SetUint64(opt.noVolumes))
		log.Info("[byvolume] has novolums", "novolumes", opt.noVolumes, "subReward", subReward)
		totalReward = new(big.Int).Sub(totalReward, subReward)
	}
	if totalReward.Sign() <= 0 {
		return nil
	}
	var rewards []*big.Int
	if opt.WeightIsPercentage {
		rewards = opt.divideVolumeRewardsByPercentage(totalReward)
	} else {
		rewards = opt.divideVolumeRewardsByExchange(accountStats, totalReward)
	}
	if len(rewards) != len(accountStats) {
		log.Warn("[byvolume] divided rewards by exchange liquidity failed")
		return nil
	}
	mongodb.CalcRewardsInBatch(accountStats, rewards)
	return opt.dispatchRewards(accountStats)
}

func (opt *Option) divideVolumeRewardsByPercentage(totalReward *big.Int) []*big.Int {
	if len(opt.Weights) == 1 {
		return []*big.Int{totalReward}
	}
	shares := make([]*big.Int, len(opt.Weights))
	for i, percent := range opt.Weights {
		shares[i] = new(big.Int).SetUint64(percent)
	}
	return mongodb.DivideRewards(totalReward, shares)
}

func calcPrevCycleSttEnd(height uint64, useTimeMeasurement bool) (preCycleStart, preCycleEnd uint64) {
	distCfg := params.GetConfig().Distribute

	if useTimeMeasurement {
		startHeight := distCfg.StartTimestamp
		cycleLen := distCfg.ByLiquidCycleDuration

		preCycleEnd = height - (height-startHeight)%cycleLen
		preCycleStart = preCycleEnd - cycleLen
		return preCycleStart, preCycleEnd
	}

	startHeight := distCfg.StartHeight
	cycleLen := distCfg.ByLiquidCycle

	preCycleEnd = height - (height-startHeight)%cycleLen
	preCycleStart = preCycleEnd - cycleLen
	return preCycleStart, preCycleEnd
}

func (opt *Option) divideVolumeRewardsByExchange(accountStats []mongodb.AccountStatSlice, totalReward *big.Int) []*big.Int {
	if len(opt.Exchanges) == 1 {
		return []*big.Int{totalReward}
	}
	if opt.Weights != nil && len(opt.Weights) != len(opt.Exchanges) {
		log.Error("divideVolumeRewards: number of weights and exchanges are not equal")
		return nil
	}

	preCycleStart, preCycleEnd := calcPrevCycleSttEnd(opt.StartHeight, opt.UseTimeMeasurement)

	var sampleHeight uint64
	var sampleBlockNumber *big.Int
	if !opt.ArchiveMode {
		latestBlock := capi.LoopGetLatestBlockHeader()
		sampleHeight = latestBlock.Number.Uint64()
		sampleBlockNumber = nil // use latest block in non archive mode
	} else {
		sampleHeight = CalcRandomSampleHeight(preCycleStart, preCycleEnd, opt.UseTimeMeasurement)
		sampleBlockNumber = new(big.Int).SetUint64(sampleHeight)
	}
	log.Info("divideVolumeRewards", "archiveMode", opt.ArchiveMode,
		"start", opt.StartHeight, "end", opt.EndHeight,
		"preCycleStart", preCycleStart, "preCycleEnd", preCycleEnd, "sampleHeight", sampleHeight)

	exchangeShares := make([]*big.Int, len(opt.Exchanges))
	for i, exchange := range opt.Exchanges {
		sumShare := accountStats[i].CalcTotalShare()
		if sumShare.Sign() == 0 {
			exchangeShares[i] = big.NewInt(0)
			continue
		}

		// use exchange's liquidity (represent by coin) as upper limit
		exCoinBalance := capi.LoopGetCoinBalance(common.HexToAddress(exchange), sampleBlockNumber)
		if opt.EndHeight-opt.StartHeight != preCycleEnd-preCycleStart {
			exCoinBalance.Mul(exCoinBalance, new(big.Int).SetUint64(opt.EndHeight-opt.StartHeight))
			exCoinBalance.Div(exCoinBalance, new(big.Int).SetUint64(preCycleEnd-preCycleStart))
		}
		var truncated bool
		totalShare := new(big.Int).Set(sumShare)
		if totalShare.Cmp(exCoinBalance) > 0 {
			totalShare.Set(exCoinBalance)
			truncated = true
		}

		weight := uint64(1)
		if len(opt.Weights) > i && opt.Weights[i] > 1 {
			weight = opt.Weights[i]
			totalShare.Mul(totalShare, new(big.Int).SetUint64(weight))
		}

		exchangeShares[i] = totalShare
		log.Info("divide volume rewards by exchange", "archiveMode", opt.ArchiveMode,
			"start", opt.StartHeight, "end", opt.EndHeight, "sampleHeight", sampleHeight,
			"exchange", exchange, "weight", weight, "totalShare", totalShare, "sumShare", sumShare,
			"upperLimit", exCoinBalance, "truncated", truncated)
	}
	return mongodb.DivideRewards(totalReward, exchangeShares)
}
