package effective_gas

import (
	"sync"
)

type PriceStorage interface {
	GetPrices() (uint64, uint64)
	SetPrices(l1Price, l2Price uint64)
}

type MemoryPriceStorage struct {
	l1Price uint64
	l2Price uint64
	mtx     *sync.Mutex
}

func NewMemoryPriceStorage() *MemoryPriceStorage {
	return &MemoryPriceStorage{mtx: &sync.Mutex{}}
}

func (ps *MemoryPriceStorage) GetPrices() (uint64, uint64) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return ps.l1Price, ps.l2Price
}

func (ps *MemoryPriceStorage) SetPrices(l1Price, l2Price uint64) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	ps.l1Price = l1Price
	ps.l2Price = l2Price
}
