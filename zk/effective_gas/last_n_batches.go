package effective_gas

import (
	"context"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon/core/rawdb"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/eth/ethconfig"
	"github.com/ledgerwatch/erigon/zkevm/log"
	"math/big"
	"sort"
	"sync"
)

const sampleNumber = 3 // Number of transactions sampled in a batch.

// LastNL2BlocksGasPrice struct for gas price estimator last n l2 blocks.
type LastNL2BlocksGasPrice struct {
	lastL2BlockNumber uint64
	lastPrice         *big.Int

	cfg ethconfig.GasPriceEstimatorConfig
	ctx context.Context

	cacheLock sync.RWMutex
	fetchLock sync.Mutex

	db      kv.RoDB
	storage PriceStorage
}

// newLastNL2BlocksGasPriceSuggester init gas price suggester for last n l2 blocks strategy.
func newLastNL2BlocksGasPriceSuggester(ctx context.Context, cfg ethconfig.GasPriceEstimatorConfig, db kv.RoDB, storage PriceStorage) *LastNL2BlocksGasPrice {
	return &LastNL2BlocksGasPrice{
		cfg:     cfg,
		ctx:     ctx,
		db:      db,
		storage: storage,
	}
}

func (g *LastNL2BlocksGasPrice) GetPrices() (uint64, uint64) {
	return g.storage.GetPrices()
}

// UpdateGasPriceAvg for last n bathes strategy is not needed to implement this function.
func (g *LastNL2BlocksGasPrice) UpdateGasPriceAvg() {
	tx, err := g.db.BeginRo(context.Background())
	if err != nil {
		log.Error("[LastNBatches] error getting read only tx", "err", err)
		return
	}
	defer tx.Rollback()

	l2BlockNumber := rawdb.ReadCurrentBlockNumber(tx)
	if l2BlockNumber == nil {
		log.Error("[LastNBatches] could not retrieve the latest block number")
		return
	}
	g.cacheLock.RLock()
	lastL2BlockNumber, lastPrice := g.lastL2BlockNumber, g.lastPrice
	g.cacheLock.RUnlock()

	if *l2BlockNumber == lastL2BlockNumber {
		log.Debug("[LastNBatches] Block is still the same, no need to update the gas price at the moment")
		return
	}

	g.fetchLock.Lock()
	defer g.fetchLock.Unlock()

	var (
		sent, exp int
		number    = lastL2BlockNumber
		result    = make(chan results, g.cfg.CheckBlocks)
		quit      = make(chan struct{})
		results   []*big.Int
	)

	for sent < g.cfg.CheckBlocks && number > 0 {
		go g.getL2BlockTxsTips(tx, number, sampleNumber, g.cfg.IgnorePrice, result, quit)
		sent++
		exp++
		number--
	}

	for exp > 0 {
		res := <-result
		if res.err != nil {
			close(quit)
			return
		}
		exp--

		if len(res.values) == 0 {
			res.values = []*big.Int{lastPrice}
		}
		results = append(results, res.values...)
	}

	price := lastPrice
	if len(results) > 0 {
		sort.Sort(bigIntArray(results))
		price = results[(len(results)-1)*g.cfg.Percentile/100]
	}
	if price.Cmp(g.cfg.MaxPrice) > 0 {
		price = g.cfg.MaxPrice
	}

	g.cacheLock.Lock()
	g.lastPrice = price
	g.lastL2BlockNumber = *l2BlockNumber
	g.cacheLock.Unlock()

	// Store gasPrices
	factorAsPercentage := int64(g.cfg.Factor * 100) // nolint:gomnd
	factor := big.NewInt(factorAsPercentage)
	l1GasPriceDivBy100 := new(big.Int).Div(g.lastPrice, factor)
	l1GasPrice := l1GasPriceDivBy100.Mul(l1GasPriceDivBy100, big.NewInt(100)) // nolint:gomnd
	g.storage.SetPrices(l1GasPrice.Uint64(), g.lastPrice.Uint64())
}

// getL2BlockTxsTips calculates l2 block transaction gas fees.
func (g *LastNL2BlocksGasPrice) getL2BlockTxsTips(tx kv.Tx, l2BlockNumber uint64, limit int, ignorePrice *big.Int, result chan results, quit chan struct{}) {
	block, err := rawdb.ReadBlockByNumber(tx, l2BlockNumber)
	if err != nil {
		log.Error("[LastNBatches] failed to retrieve block", "no", l2BlockNumber)
		select {
		case result <- results{nil, err}:
		case <-quit:
		}
		return
	}
	sorter := newSorter(block.Transactions())
	sort.Sort(sorter)

	var prices []*big.Int
	for _, tx := range sorter.txs {
		tip := tx.GetTip()
		if ignorePrice != nil && tip.ToBig().Cmp(ignorePrice) == -1 {
			continue
		}
		prices = append(prices, tip.ToBig())
		if len(prices) >= limit {
			break
		}
	}
	select {
	case result <- results{prices, nil}:
	case <-quit:
	}
}

type results struct {
	values []*big.Int
	err    error
}

type txSorter struct {
	txs types.Transactions
}

func newSorter(txs types.Transactions) *txSorter {
	return &txSorter{
		txs: txs,
	}
}

func (s *txSorter) Len() int { return len(s.txs) }
func (s *txSorter) Swap(i, j int) {
	s.txs[i], s.txs[j] = s.txs[j], s.txs[i]
}
func (s *txSorter) Less(i, j int) bool {
	tip1 := s.txs[i].GetTip()
	tip2 := s.txs[j].GetTip()
	return tip1.Cmp(tip2) < 0
}

type bigIntArray []*big.Int

func (s bigIntArray) Len() int           { return len(s) }
func (s bigIntArray) Less(i, j int) bool { return s[i].Cmp(s[j]) < 0 }
func (s bigIntArray) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
