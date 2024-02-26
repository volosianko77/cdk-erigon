package effective_gas

import (
	"context"
	"github.com/ledgerwatch/erigon/eth/ethconfig"
	"github.com/ledgerwatch/erigon/ethclient"
	"github.com/ledgerwatch/erigon/zkevm/log"
	"sync/atomic"
	"time"
)

type GasPriceTracker struct {
	cfg           *ethconfig.Zk
	checkDuration time.Duration

	quit         chan struct{}
	currentPrice atomic.Uint64
	busy         atomic.Bool
}

func NewGasPriceTracker(cfg *ethconfig.Zk, checkDuration time.Duration) *GasPriceTracker {
	return &GasPriceTracker{
		cfg:           cfg,
		checkDuration: checkDuration,
		quit:          make(chan struct{}),
	}
}

func (gpt *GasPriceTracker) StopTracking() {
	gpt.quit <- struct{}{}
}

func (gpt *GasPriceTracker) StartTracking() {
	// always get an initial price
	gpt.getPriceAndStore()
	ticker := time.NewTicker(gpt.checkDuration)
	go func() {
		for {
			select {
			case <-gpt.quit:
				return
			case <-ticker.C:
				func() {
					// only do some work if we're not in a busy state
					if gpt.busy.CompareAndSwap(false, true) {
						gpt.getPriceAndStore()
						gpt.busy.Store(false)
					}
				}()
			}
		}
	}()
}

func (gpt *GasPriceTracker) getPriceAndStore() {
	ethClient, err := ethclient.Dial(gpt.cfg.L1RpcUrl)
	if err != nil {
		log.Error("[GasPriceTracker] problem", "err", err)
		return
	}
	price, err := ethClient.SuggestGasPrice(context.Background())
	if err != nil {
		log.Error("[GasPriceTracker] problem", "err", err)
		return
	}
	gpt.currentPrice.Store(price.Uint64())
}

func (gpt *GasPriceTracker) GetCurrentGasPrice() uint64 {
	return gpt.currentPrice.Load()
}
