package cli

import (
	"fmt"

	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/cmd/utils"
	"github.com/ledgerwatch/erigon/eth/ethconfig"
	"github.com/ledgerwatch/erigon/zk/sequencer"
	"github.com/urfave/cli/v2"
	"math/big"
	"strings"
	"time"
)

func ApplyFlagsForZkConfig(ctx *cli.Context, cfg *ethconfig.Config) {
	checkFlag := func(flagName string, value interface{}) {
		switch v := value.(type) {
		case string:
			if v == "" {
				panic(fmt.Sprintf("Flag not set: %s", flagName))
			}
		case uint64:
			if v == 0 {
				panic(fmt.Sprintf("Flag not set: %s", flagName))
			}
		}
	}

	priceSuggesterUpdatePeriod, err := time.ParseDuration(ctx.String(utils.PriceSuggesterUpdatePeriod.Name))
	if err != nil {
		panic("could not parse price suggester update period")
	}

	var priceSuggesterMaxPrice *big.Int
	readMaxPrice := ctx.Uint64(utils.PriceSuggesterMaxPrice.Name)
	if readMaxPrice > 0 {
		priceSuggesterMaxPrice = big.NewInt(0).SetUint64(readMaxPrice)
	}

	var priceSuggesterIgnorePrice *big.Int
	readIgnorePrice := ctx.Uint64(utils.PriceSuggesterIgnorePrice.Name)
	if readIgnorePrice > 0 {
		priceSuggesterIgnorePrice = big.NewInt(0).SetUint64(readIgnorePrice)
	}

	cfg.Zk = &ethconfig.Zk{
		L2ChainId:                   ctx.Uint64(utils.L2ChainIdFlag.Name),
		L2RpcUrl:                    ctx.String(utils.L2RpcUrlFlag.Name),
		L2DataStreamerUrl:           ctx.String(utils.L2DataStreamerUrlFlag.Name),
		L1ChainId:                   ctx.Uint64(utils.L1ChainIdFlag.Name),
		L1RpcUrl:                    ctx.String(utils.L1RpcUrlFlag.Name),
		L1PolygonRollupManager:      libcommon.HexToAddress(ctx.String(utils.L1PolygonRollupManagerFlag.Name)),
		L1Rollup:                    libcommon.HexToAddress(ctx.String(utils.L1RollupFlag.Name)),
		L1TopicVerification:         libcommon.HexToHash(ctx.String(utils.L1TopicVerificationFlag.Name)),
		L1TopicSequence:             libcommon.HexToHash(ctx.String(utils.L1TopicSequenceFlag.Name)),
		L1BlockRange:                ctx.Uint64(utils.L1BlockRangeFlag.Name),
		L1QueryDelay:                ctx.Uint64(utils.L1QueryDelayFlag.Name),
		L1MaticContractAddress:      libcommon.HexToAddress(ctx.String(utils.L1MaticContractAddressFlag.Name)),
		L1GERManagerContractAddress: libcommon.HexToAddress(ctx.String(utils.L1GERManagerContractAddressFlag.Name)),
		L1FirstBlock:                ctx.Uint64(utils.L1FirstBlockFlag.Name),
		RpcRateLimits:               ctx.Int(utils.RpcRateLimitsFlag.Name),
		DatastreamVersion:           ctx.Int(utils.DatastreamVersionFlag.Name),
		RebuildTreeAfter:            ctx.Uint64(utils.RebuildTreeAfterFlag.Name),
		SequencerAddress:            libcommon.HexToAddress(ctx.String(utils.SequencerAddressFlag.Name)),
		ExecutorUrls:                strings.Split(ctx.String(utils.ExecutorUrls.Name), ","),
		ExecutorStrictMode:          ctx.Bool(utils.ExecutorStrictMode.Name),
		EffectiveGas: ethconfig.EffectiveGasPriceCfg{
			Enabled:                     ctx.Bool(utils.EffectiveGasEnabled.Name),
			L1GasPriceFactor:            ctx.Float64(utils.EffectiveGasL1GasPriceFactor.Name),
			ByteGasCost:                 ctx.Uint64(utils.EffectiveGasByteGasCost.Name),
			ZeroByteGasCost:             ctx.Uint64(utils.EffectiveGasZeroByteGasCost.Name),
			NetProfit:                   ctx.Float64(utils.EffectiveGasNetProfit.Name),
			BreakEvenFactor:             ctx.Float64(utils.EffectiveGasBreakEvenFactor.Name),
			FinalDeviationPct:           ctx.Uint64(utils.EffectiveGasFinalDeviationPct.Name),
			EthTransferGasPrice:         ctx.Uint64(utils.EffectiveGasEthTransferGasPrice.Name),
			EthTransferL1GasPriceFactor: ctx.Float64(utils.EffectiveGasEthTransferL1GasPriceFactor.Name),
			L2GasPriceSuggesterFactor:   ctx.Float64(utils.EffectiveGasL2GasPriceSuggesterFactor.Name),
		},
		GasPriceEstimator: ethconfig.GasPriceEstimatorConfig{
			Type:               ethconfig.EstimatorType(ctx.String(utils.PriceSuggesterType.Name)),
			DefaultGasPriceWei: ctx.Uint64(utils.PriceSuggesterDefaultGasPriceWei.Name),
			MaxGasPriceWei:     ctx.Uint64(utils.PriceSuggesterMaxGasPriceWei.Name),
			MaxPrice:           priceSuggesterMaxPrice,
			IgnorePrice:        priceSuggesterIgnorePrice,
			CheckBlocks:        ctx.Int(utils.PriceSuggesterCheckBlocks.Name),
			Percentile:         ctx.Int(utils.PriceSuggesterPercentile.Name),
			UpdatePeriod:       priceSuggesterUpdatePeriod,
			Factor:             ctx.Float64(utils.PriceSuggesterFactor.Name),
		},
	}

	checkFlag(utils.L2ChainIdFlag.Name, cfg.Zk.L2ChainId)
	if !sequencer.IsSequencer() {
		checkFlag(utils.L2RpcUrlFlag.Name, cfg.Zk.L2RpcUrl)
		checkFlag(utils.L2DataStreamerUrlFlag.Name, cfg.Zk.L2DataStreamerUrl)
	} else {
		checkFlag(utils.SequencerAddressFlag.Name, cfg.Zk.SequencerAddress)
		checkFlag(utils.ExecutorUrls.Name, cfg.Zk.ExecutorUrls)
		checkFlag(utils.ExecutorStrictMode.Name, cfg.Zk.ExecutorStrictMode)

		// if we are running in strict mode, the default, and we have no executor URLs then we panic
		if cfg.Zk.ExecutorStrictMode && (len(cfg.Zk.ExecutorUrls) == 0 || cfg.Zk.ExecutorUrls[0] == "") {
			panic("You must set executor urls when running in executor strict mode (zkevm.executor-strict)")
		}

		checkFlag(utils.EffectiveGasEnabled.Name, cfg.Zk.EffectiveGas.Enabled)
		checkFlag(utils.EffectiveGasL1GasPriceFactor.Name, cfg.Zk.EffectiveGas.L1GasPriceFactor)
		checkFlag(utils.EffectiveGasByteGasCost.Name, cfg.Zk.EffectiveGas.ByteGasCost)
		checkFlag(utils.EffectiveGasZeroByteGasCost.Name, cfg.Zk.EffectiveGas.ZeroByteGasCost)
		checkFlag(utils.EffectiveGasNetProfit.Name, cfg.Zk.EffectiveGas.NetProfit)
		checkFlag(utils.EffectiveGasBreakEvenFactor.Name, cfg.Zk.EffectiveGas.BreakEvenFactor)
		checkFlag(utils.EffectiveGasFinalDeviationPct.Name, cfg.Zk.EffectiveGas.FinalDeviationPct)
		checkFlag(utils.EffectiveGasL2GasPriceSuggesterFactor.Name, cfg.Zk.EffectiveGas.L2GasPriceSuggesterFactor)

		switch cfg.Zk.GasPriceEstimator.Type {
		case ethconfig.DefaultType:
			checkFlag(utils.PriceSuggesterFactor.Name, cfg.Zk.GasPriceEstimator.Factor)
			checkFlag(utils.PriceSuggesterDefaultGasPriceWei.Name, cfg.Zk.GasPriceEstimator.DefaultGasPriceWei)
		case ethconfig.FollowerType:
			checkFlag(utils.PriceSuggesterFactor.Name, cfg.Zk.GasPriceEstimator.Factor)
			checkFlag(utils.PriceSuggesterDefaultGasPriceWei.Name, cfg.Zk.GasPriceEstimator.DefaultGasPriceWei)
			checkFlag(utils.PriceSuggesterMaxGasPriceWei.Name, cfg.Zk.GasPriceEstimator.MaxGasPriceWei)
		case ethconfig.LastNBatchesType:
			checkFlag(utils.PriceSuggesterFactor.Name, cfg.Zk.GasPriceEstimator.Factor)
			checkFlag(utils.PriceSuggesterCheckBlocks.Name, cfg.Zk.GasPriceEstimator.CheckBlocks)
			checkFlag(utils.PriceSuggesterIgnorePrice.Name, cfg.Zk.GasPriceEstimator.IgnorePrice)
			checkFlag(utils.PriceSuggesterPercentile.Name, cfg.Zk.GasPriceEstimator.Percentile)
			checkFlag(utils.PriceSuggesterMaxPrice.Name, cfg.Zk.GasPriceEstimator.MaxPrice)
		}
	}
	checkFlag(utils.L1ChainIdFlag.Name, cfg.Zk.L1ChainId)
	checkFlag(utils.L1RpcUrlFlag.Name, cfg.Zk.L1RpcUrl)
	checkFlag(utils.L1PolygonRollupManagerFlag.Name, cfg.Zk.L1PolygonRollupManager.Hex())
	checkFlag(utils.L1RollupFlag.Name, cfg.Zk.L1Rollup.Hex())
	checkFlag(utils.L1TopicVerificationFlag.Name, cfg.Zk.L1TopicVerification.Hex())
	checkFlag(utils.L1TopicSequenceFlag.Name, cfg.Zk.L1TopicSequence.Hex())
	checkFlag(utils.L1MaticContractAddressFlag.Name, cfg.Zk.L1MaticContractAddress.Hex())
	checkFlag(utils.L1GERManagerContractAddressFlag.Name, cfg.Zk.L1GERManagerContractAddress.Hex())
	checkFlag(utils.L1FirstBlockFlag.Name, cfg.Zk.L1FirstBlock)
	checkFlag(utils.RpcRateLimitsFlag.Name, cfg.Zk.RpcRateLimits)
	checkFlag(utils.RebuildTreeAfterFlag.Name, cfg.Zk.RebuildTreeAfter)
	checkFlag(utils.L1BlockRangeFlag.Name, cfg.Zk.L1BlockRange)
	checkFlag(utils.L1QueryDelayFlag.Name, cfg.Zk.L1QueryDelay)
}
