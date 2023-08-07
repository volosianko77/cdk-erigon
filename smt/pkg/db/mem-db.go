package db

import (
	"github.com/ledgerwatch/erigon/smt/pkg/utils"
	"sync"
)

type MemDb struct {
	Db   map[string][]string
	lock sync.RWMutex
}

func NewMemDb() *MemDb {
	return &MemDb{
		Db: make(map[string][]string),
	}
}

func (m *MemDb) Get(key utils.NodeKey) (utils.NodeValue12, error) {
	m.lock.RLock()         // Lock for reading
	defer m.lock.RUnlock() // Make sure to unlock when done

	keyConc := utils.ArrayToScalar(key[:])

	k := utils.ConvertBigIntToHex(keyConc)

	values := utils.NodeValue12{}
	for i, v := range m.Db[k] {
		values[i] = utils.ConvertHexToBigInt(v)
	}

	return values, nil
}

func (m *MemDb) Insert(key utils.NodeKey, value utils.NodeValue12) error {
	m.lock.Lock()         // Lock for writing
	defer m.lock.Unlock() // Make sure to unlock when done

	keyConc := utils.ArrayToScalar(key[:])
	k := utils.ConvertBigIntToHex(keyConc)

	values := make([]string, 12)
	for i, v := range value {
		values[i] = utils.ConvertBigIntToHex(v)
	}

	m.Db[k] = values
	return nil
}

func (m *MemDb) Delete(key string) error {
	m.lock.Lock()         // Lock for writing
	defer m.lock.Unlock() // Make sure to unlock when done

	delete(m.Db, key)
	return nil
}

func (m *MemDb) IsEmpty() bool {
	m.lock.RLock()         // Lock for reading
	defer m.lock.RUnlock() // Make sure to unlock when done

	return len(m.Db) == 0
}

func (m *MemDb) PrintDb() {
	m.lock.RLock()         // Lock for reading
	defer m.lock.RUnlock() // Make sure to unlock when done

	for k, v := range m.Db {
		println(k, v)
	}
}

func (m *MemDb) GetDb() map[string][]string {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return m.Db
}