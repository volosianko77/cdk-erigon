package effective_gas

import (
	"context"
	"fmt"
	"math/big"
	"strconv"

	"github.com/ledgerwatch/erigon/eth/ethconfig"
	"github.com/ledgerwatch/erigon/ethclient"
	"github.com/ledgerwatch/log/v3"
)

const (
	base10 = 10
)

// FollowerGasPrice struct.
type FollowerGasPrice struct {
	cfg      ethconfig.GasPriceEstimatorConfig
	l1RpcUrl string
	storage  PriceStorage
}

// newFollowerGasPriceSuggester inits l2 follower gas price suggester which is based on the l1 gas price.
func newFollowerGasPriceSuggester(cfg ethconfig.GasPriceEstimatorConfig, l1RpcUrl string, storage PriceStorage) *FollowerGasPrice {
	gps := &FollowerGasPrice{
		cfg:      cfg,
		l1RpcUrl: l1RpcUrl,
		storage:  storage,
	}
	gps.UpdateGasPriceAvg()
	return gps
}

func (f *FollowerGasPrice) GetPrices() (uint64, uint64) {
	return f.storage.GetPrices()
}

// UpdateGasPriceAvg updates the gas price.
func (f *FollowerGasPrice) UpdateGasPriceAvg() {
	// Get L1 gasprice
	ethClient, err := ethclient.Dial(f.l1RpcUrl)
	if err != nil {
		log.Error("[Follower] problem", "err", err)
		return
	}
	l1GasPrice, err := ethClient.SuggestGasPrice(context.Background())
	if err != nil {
		log.Error("[Follower] problem", "err", err)
		return
	}
	if big.NewInt(0).Cmp(l1GasPrice) == 0 {
		log.Warn("[Follower] gas price 0 received. Skipping update...")
		return
	}

	// Apply factor to calculate l2 gasPrice
	factor := big.NewFloat(0).SetFloat64(f.cfg.Factor)
	res := new(big.Float).Mul(factor, big.NewFloat(0).SetInt(l1GasPrice))

	// Store l2 gasPrice calculated
	result := new(big.Int)
	res.Int(result)
	minGasPrice := big.NewInt(0).SetUint64(f.cfg.DefaultGasPriceWei)
	if minGasPrice.Cmp(result) == 1 { // minGasPrice > result
		log.Warn("[Follower] setting DefaultGasPriceWei for L2")
		result = minGasPrice
	}
	maxGasPrice := new(big.Int).SetUint64(f.cfg.MaxGasPriceWei)
	if f.cfg.MaxGasPriceWei > 0 && result.Cmp(maxGasPrice) == 1 { // result > maxGasPrice
		log.Warn("[Follower] setting MaxGasPriceWei for L2")
		result = maxGasPrice
	}
	var truncateValue *big.Int
	log.Debug("[Follower] Full L2 gas price value: ", result, ". Length: ", len(result.String()))
	numLength := len(result.String())
	if numLength > 3 { //nolint:gomnd
		aux := "%0" + strconv.Itoa(numLength-3) + "d" //nolint:gomnd
		var ok bool
		value := result.String()[:3] + fmt.Sprintf(aux, 0)
		truncateValue, ok = new(big.Int).SetString(value, base10)
		if !ok {
			log.Error("[Follower] error converting: ", truncateValue)
		}
	} else {
		truncateValue = result
	}
	log.Debug("[Follower] Storing truncated L2 gas price: ", truncateValue)
	if truncateValue != nil {
		f.storage.SetPrices(l1GasPrice.Uint64(), truncateValue.Uint64())
	} else {
		log.Error("[Follower] nil value detected. Skipping...")
	}
}
