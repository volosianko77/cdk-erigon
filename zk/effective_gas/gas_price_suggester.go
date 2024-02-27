package effective_gas

import (
	"context"
	"fmt"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon/eth/ethconfig"
	"github.com/ledgerwatch/log/v3"
	"time"
)

// L2GasPricer interface for gas price suggester.
type L2GasPricer interface {
	UpdateGasPriceAvg()
}

// NewL2GasPriceSuggester init.
func StartPriceSuggestor(ctx context.Context, zkcfg ethconfig.Zk, db kv.RoDB, storage PriceStorage) {
	cfg := zkcfg.GasPriceEstimator
	var gpricer L2GasPricer
	switch cfg.Type {
	case ethconfig.LastNBatchesType:
		log.Info("[GasPriceSuggester] Lastnbatches type selected")
		gpricer = newLastNL2BlocksGasPriceSuggester(ctx, cfg, db, storage)
	case ethconfig.FollowerType:
		log.Info("[GasPriceSuggester] follower type selected")
		gpricer = newFollowerGasPriceSuggester(cfg, zkcfg.L1RpcUrl, storage)
	case ethconfig.DefaultType:
		log.Info("[GasPriceSuggester] default type selected")
		gpricer = newDefaultGasPriceSuggester(cfg, storage)
	default:
		panic(fmt.Sprintf("unknown l2 gas price suggester type %s. Please specify a valid one: 'lastnbatches', 'follower' or 'default'", cfg.Type))
	}

	updateTimer := time.NewTimer(cfg.UpdatePeriod)

	for {
		select {
		case <-ctx.Done():
			log.Info("Finishing l2 gas price suggester...")
			return
		case <-updateTimer.C:
			gpricer.UpdateGasPriceAvg()
			updateTimer.Reset(cfg.UpdatePeriod)
		}
	}
}
