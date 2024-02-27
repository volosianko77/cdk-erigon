package effective_gas

import (
	"github.com/ledgerwatch/erigon/eth/ethconfig"
	"math/big"
)

// DefaultGasPricer gas price from config is set.
type DefaultGasPricer struct {
	cfg        ethconfig.GasPriceEstimatorConfig
	l1GasPrice uint64
	storage    PriceStorage
}

// newDefaultGasPriceSuggester init default gas price suggester.
func newDefaultGasPriceSuggester(cfg ethconfig.GasPriceEstimatorConfig, storage PriceStorage) *DefaultGasPricer {
	// Apply factor to calculate l1 gasPrice
	factorAsPercentage := int64(cfg.Factor * 100) // nolint:gomnd
	factor := big.NewInt(factorAsPercentage)
	defaultGasPriceDivByFactor := new(big.Int).Div(new(big.Int).SetUint64(cfg.DefaultGasPriceWei), factor)

	gpe := &DefaultGasPricer{
		cfg:        cfg,
		storage:    storage,
		l1GasPrice: new(big.Int).Mul(defaultGasPriceDivByFactor, big.NewInt(100)).Uint64(), // nolint:gomnd
	}
	gpe.UpdateGasPriceAvg()
	return gpe
}

// UpdateGasPriceAvg not needed for default strategy.
func (d *DefaultGasPricer) UpdateGasPriceAvg() {
	d.storage.SetPrices(d.l1GasPrice, d.cfg.DefaultGasPriceWei)
}

func (d *DefaultGasPricer) GetPrices() (uint64, uint64) {
	return d.storage.GetPrices()
}
