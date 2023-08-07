package smt

import (
	"math/big"

	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/TwiN/gocache/v2"
	"github.com/ledgerwatch/erigon/smt/pkg/db"
	"github.com/ledgerwatch/erigon/smt/pkg/utils"
	"github.com/ledgerwatch/log/v3"
	"sort"
	"sync"
	"time"
)

type DB interface {
	Get(key utils.NodeKey) (utils.NodeValue12, error)
	Insert(key utils.NodeKey, value utils.NodeValue12) error
	IsEmpty() bool
	Delete(string) error
}

type DebuggableDB interface {
	DB
	PrintDb()
	GetDb() map[string][]string
}

type SMT struct {
	Db                DB
	Cache             *gocache.Cache
	CacheHitFrequency map[string]int

	lastRoot     *big.Int
	clearUpMutex sync.Mutex
}

type SMTResponse struct {
	NewRoot *big.Int
	Mode    string
}

func NewSMT(database DB) *SMT {
	if database == nil {
		database = db.NewMemDb()
	}
	cache := gocache.NewCache().WithMaxSize(10000).WithEvictionPolicy(gocache.LeastRecentlyUsed)
	return &SMT{
		Db:                database,
		Cache:             cache,
		CacheHitFrequency: make(map[string]int),
		lastRoot:          big.NewInt(0),
	}
}

func (s *SMT) LastRoot() *big.Int {
	s.clearUpMutex.Lock()
	defer s.clearUpMutex.Unlock()
	copy := new(big.Int).Set(s.lastRoot)
	return copy
}

func (s *SMT) SetLastRoot(lr *big.Int) {
	s.clearUpMutex.Lock()
	defer s.clearUpMutex.Unlock()
	s.lastRoot = lr
}

func (s *SMT) StartPeriodicCheck(doneChan chan bool) {
	if _, ok := s.Db.(*db.EriDb); ok {
		log.Warn("mdbx tx cannot be used in goroutine - periodic check disabled")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-doneChan:
				cancel()
				return
			case <-ticker.C:
				// start timer
				start := time.Now()
				ct := s.CheckOrphanedNodes(ctx)
				elapsed := time.Since(start)
				fmt.Printf("CheckOrphanedNodes took %s removing %d orphaned nodes\n", elapsed, ct)
			}
		}
	}()
}

func (s *SMT) InsertBI(lastRoot *big.Int, key *big.Int, value *big.Int) (*SMTResponse, error) {
	k := utils.ScalarToNodeKey(key)
	v := utils.ScalarToNodeValue8(value)
	return s.Insert(k, v)
}

func (s *SMT) InsertKA(key utils.NodeKey, value *big.Int) (*SMTResponse, error) {
	x := utils.ScalarToArrayBig(value)
	v, err := utils.NodeValue8FromBigIntArray(x)
	if err != nil {
		return nil, err
	}

	return s.Insert(key, *v)
}

func (s *SMT) Insert(k utils.NodeKey, v utils.NodeValue8) (*SMTResponse, error) {

	s.clearUpMutex.Lock()
	defer s.clearUpMutex.Unlock()

	oldRoot := utils.ScalarToRoot(s.lastRoot)
	r := oldRoot
	newRoot := oldRoot

	smtResponse := &SMTResponse{
		NewRoot: big.NewInt(0),
		Mode:    "not run",
	}

	if k.IsZero() && v.IsZero() {
		return smtResponse, nil
	}

	// split the key
	keys := k.GetPath()

	var usedKey []int
	var siblings map[int]*utils.NodeValue12
	var level int
	var foundKey *utils.NodeKey
	var foundVal utils.NodeValue8
	var oldValue utils.NodeValue8
	var foundRKey utils.NodeKey
	var proofHashCounter int
	var foundOldValHash utils.NodeKey
	var insKey *utils.NodeKey
	var insValue utils.NodeValue8
	var isOld0 bool

	siblings = map[int]*utils.NodeValue12{}

	// JS WHILE
	for !r.IsZero() && foundKey == nil {
		sl, err := s.Db.Get(r)
		if err != nil {
			return nil, err
		}
		siblings[level] = &sl
		if siblings[level].IsFinalNode() {
			foundOldValHash = utils.NodeKeyFromBigIntArray(siblings[level][4:8])
			fva, err := s.Db.Get(foundOldValHash)
			if err != nil {
				return nil, err
			}
			foundValA := utils.Value8FromBigIntArray(fva[0:8])
			foundRKey = utils.NodeKeyFromBigIntArray(siblings[level][0:4])
			foundVal = foundValA

			foundKey = utils.JoinKey(usedKey, foundRKey)
			if err != nil {
				return nil, err
			}
		} else {
			r = utils.NodeKeyFromBigIntArray(siblings[level][keys[level]*4 : keys[level]*4+4])
			usedKey = append(usedKey, keys[level])
			level++
		}
	}

	level--
	if len(usedKey) != 0 {
		usedKey = usedKey[:len(usedKey)-1]
	}

	proofHashCounter = 0
	if !oldRoot.IsZero() {
		utils.RemoveOver(siblings, level+1)
		proofHashCounter += len(siblings)
		if foundVal.IsZero() {
			proofHashCounter += 2
		}
	}

	if !v.IsZero() { // we have a value - so we're updating or inserting
		if foundKey != nil {
			if foundKey.IsEqualTo(k) {
				// UPDATE MODE
				smtResponse.Mode = "update"
				oldValue = foundVal

				newValH, err := s.HashSave(v.ToUintArray(), utils.BranchCapacity)
				if err != nil {
					return nil, err
				}
				newLeafHash, err := s.HashSave(utils.ConcatArrays4(foundRKey, newValH), utils.LeafCapacity)
				if err != nil {
					return nil, err
				}
				if level >= 0 {
					for j := 0; j < 4; j++ {
						siblings[level][keys[level]*4+j] = new(big.Int).SetUint64(newLeafHash[j])
					}
				} else {
					newRoot = newLeafHash
				}
			} else {
				smtResponse.Mode = "insertFound"
				// INSERT WITH FOUND KEY
				level2 := level + 1
				foundKeys := foundKey.GetPath()

				for {
					if level2 >= len(keys) || level2 >= len(foundKeys) {
						break
					}

					if keys[level2] != foundKeys[level2] {
						break
					}

					level2++
				}

				oldKey := utils.RemoveKeyBits(*foundKey, level2+1)
				oldLeafHash, err := s.HashSave(utils.ConcatArrays4(oldKey, foundOldValHash), utils.LeafCapacity)
				if err != nil {
					return nil, err
				}

				insKey = foundKey
				insValue = foundVal
				isOld0 = false

				newKey := utils.RemoveKeyBits(k, level2+1)
				newValH, err := s.HashSave(v.ToUintArray(), utils.BranchCapacity)
				if err != nil {
					return nil, err
				}

				newLeafHash, err := s.HashSave(utils.ConcatArrays4(newKey, newValH), utils.LeafCapacity)

				var node [8]uint64
				for i := 0; i < 8; i++ {
					node[i] = 0
				}

				for j := 0; j < 4; j++ {
					node[keys[level2]*4+j] = newLeafHash[j]
					node[foundKeys[level2]*4+j] = oldLeafHash[j]
				}

				r2, err := s.HashSave(node, utils.BranchCapacity)
				if err != nil {
					return nil, err
				}
				proofHashCounter += 4
				level2 -= 1

				for level2 != level {
					for i := 0; i < 8; i++ {
						node[i] = 0
					}

					for j := 0; j < 4; j++ {
						node[keys[level2]*4+j] = r2[j]
					}

					r2, err = s.HashSave(node, utils.BranchCapacity)
					if err != nil {
						return nil, err
					}
					proofHashCounter += 1
					level2 -= 1
				}

				if level >= 0 {
					for j := 0; j < 4; j++ {
						siblings[level][keys[level]*4+j] = new(big.Int).SetUint64(r2[j])
					}
				} else {
					newRoot = r2
				}
			}

		} else {
			// INSERT NOT FOUND
			smtResponse.Mode = "insertNotFound"
			newKey := utils.RemoveKeyBits(k, level+1)

			newValH, err := s.HashSave(v.ToUintArray(), utils.BranchCapacity)
			if err != nil {
				return nil, err
			}

			nk := utils.ConcatArrays4(newKey, newValH)

			newLeafHash, err := s.HashSave(nk, utils.LeafCapacity)
			if err != nil {
				return nil, err
			}

			proofHashCounter += 2

			if level >= 0 {
				for j := 0; j < 4; j++ {
					nlh := big.Int{}
					nlh.SetUint64(newLeafHash[j])
					siblings[level][keys[level]*4+j] = &nlh
				}
			} else {
				newRoot = newLeafHash
			}
		}
	} else if foundKey != nil && foundKey.IsEqualTo(k) { // we don't have a value so we're deleting
		oldValue = foundVal
		if level >= 0 {
			for j := 0; j < 4; j++ {
				siblings[level][keys[level]*4+j] = nil
			}

			uKey, err := siblings[level].IsUniqueSibling()
			if err != nil {
				return nil, err
			}

			if uKey {
				// DELETE FOUND
				smtResponse.Mode = "deleteFound"
				dk := utils.NodeKeyFromBigIntArray(siblings[level][4:8])
				sl, err := s.Db.Get(dk)
				if err != nil {
					return nil, err
				}
				siblings[level+1] = &sl

				if siblings[level+1].IsFinalNode() {
					valH := siblings[level+1].Get4to8()
					preValA, err := s.Db.Get(*valH)
					if err != nil {
						return nil, err
					}
					valA := preValA.Get0to8()
					rKey := siblings[level+1].Get0to4()
					proofHashCounter += 2

					val := utils.ArrayToScalar(valA[:])
					insKey = utils.JoinKey(append(usedKey, 1), *rKey)
					insValue = utils.ScalarToNodeValue8(val)
					isOld0 = false

					for uKey && level >= 0 {
						level -= 1
						if level >= 0 {
							uKey, err = siblings[level].IsUniqueSibling()
							if err != nil {
								return nil, err
							}
						}
					}

					oldKey := utils.RemoveKeyBits(*insKey, level+1)
					oldLeafHash, err := s.HashSave(utils.ConcatArrays4(oldKey, *valH), utils.LeafCapacity)
					if err != nil {
						return nil, err
					}
					proofHashCounter += 1

					if level >= 0 {
						for j := 0; j < 4; j++ {
							siblings[level][keys[level]*4+j] = new(big.Int).SetUint64(oldLeafHash[j])
						}
					} else {
						newRoot = oldLeafHash
					}
				} else {
					// DELETE NOT FOUND
					smtResponse.Mode = "deleteNotFound"
				}
			} else {
				// DELETE NOT FOUND
				smtResponse.Mode = "deleteNotFound"
			}
		} else {
			// DELETE LAST
			smtResponse.Mode = "deleteLast"
			newRoot = utils.NodeKey{0, 0, 0, 0}
		}
	} else { // we're going zero to zero - do nothing
		smtResponse.Mode = "zeroToZero"
		if foundKey != nil {
			insKey = foundKey
			insValue = foundVal
			isOld0 = false
		}
	}

	utils.RemoveOver(siblings, level+1)

	for level >= 0 {
		hashValueIn, err := utils.NodeValue8FromBigIntArray(siblings[level][0:8])
		hashCapIn := utils.NodeKeyFromBigIntArray(siblings[level][8:12])
		newRoot, err = s.HashSave(hashValueIn.ToUintArray(), hashCapIn)
		if err != nil {
			return nil, err
		}
		proofHashCounter += 1
		level -= 1
		if level >= 0 {
			for j := 0; j < 4; j++ {
				nrj := big.Int{}
				nrj.SetUint64(newRoot[j])
				siblings[level][keys[level]*4+j] = &nrj
			}
		}
	}

	_ = r
	_ = oldValue
	_ = insValue
	_ = isOld0

	smtResponse.NewRoot = newRoot.ToBigInt()

	s.lastRoot = newRoot.ToBigInt()

	return smtResponse, nil
}

func (s *SMT) HashSave(in [8]uint64, capacity [4]uint64) ([4]uint64, error) {
	cacheKey := fmt.Sprintf("%v-%v", in, capacity)
	if cachedValue, exists := s.Cache.Get(cacheKey); exists {
		s.CacheHitFrequency[cacheKey]++
		return cachedValue.([4]uint64), nil
	}

	h, err := utils.Hash(in, capacity)
	if err != nil {
		return [4]uint64{}, err
	}

	var sl []uint64
	sl = append(sl, in[:]...)
	sl = append(sl, capacity[:]...)

	v := utils.NodeValue12{}
	for i, val := range sl {
		b := new(big.Int)
		v[i] = b.SetUint64(val)
	}

	err = s.Db.Insert(h, v)

	s.Cache.Set(cacheKey, h)

	return h, err
}

func (s *SMT) HashSave1(in [8]uint64, capacity [4]uint64) ([4]uint64, error) {
	h, err := utils.Hash(in, capacity)
	if err != nil {
		return [4]uint64{}, err
	}

	var sl []uint64

	sl = append(sl, in[:]...)
	sl = append(sl, capacity[:]...)

	v := utils.NodeValue12{}
	for i, val := range sl {
		b := new(big.Int)
		v[i] = b.SetUint64(val)
	}

	err = s.Db.Insert(h, v)
	return h, err
}

// Utility functions for debugging

func (s *SMT) PrintDb() {
	if debugDB, ok := s.Db.(DebuggableDB); ok {
		debugDB.PrintDb()
	}
}

func (s *SMT) PrintTree() {
	if debugDB, ok := s.Db.(DebuggableDB); ok {
		data := debugDB.GetDb()
		str, err := json.Marshal(data)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(string(str))
	}
}

func (s *SMT) PrintCacheHitsByFrequency() {
	type kv struct {
		Key   string
		Value int
	}

	fmt.Println("SMT Cache Hits:")

	var ss []kv
	for k, v := range s.CacheHitFrequency {
		if v > 1 {
			ss = append(ss, kv{k, v})
		}
	}

	sort.Slice(ss, func(i, j int) bool {
		return ss[i].Value > ss[j].Value
	})

	for i, kv := range ss {
		if i > 10 {
			break
		}
		fmt.Printf("%s: %d\n", kv.Key, kv.Value)
	}
}

type VisitedNodesMap map[string]bool

func (s *SMT) CheckOrphanedNodes(ctx context.Context) int {
	if _, ok := s.Db.(*db.EriDb); ok {
		log.Warn("mdbx tx cannot be used in goroutine - periodic check disabled")
		return 0
	}

	s.clearUpMutex.Lock()
	defer s.clearUpMutex.Unlock()

	visited := make(VisitedNodesMap)

	root := s.lastRoot

	err := s.traverseAndMark(ctx, root, visited)
	if err != nil {
		return 0
	}

	debugDB, ok := s.Db.(DebuggableDB)
	if !ok {
		log.Warn("db is not cleanable")
	}

	orphanedNodes := make([]string, 0)
	for dbKey := range debugDB.GetDb() {
		if _, ok := visited[dbKey]; !ok {
			orphanedNodes = append(orphanedNodes, dbKey)
		}
	}

	rootKey := utils.ConvertBigIntToHex(root)

	for _, node := range orphanedNodes {
		if node == rootKey {
			continue
		}
		err := s.Db.Delete(node)
		if err != nil {
			log.Warn("failed to delete orphaned node", "node", node, "err", err)
		}
	}

	return len(orphanedNodes)
}

func (s *SMT) traverseAndMark(ctx context.Context, node *big.Int, visited VisitedNodesMap) error {
	if node == nil || node.Cmp(big.NewInt(0)) == 0 {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	nodeKey := utils.ConvertBigIntToHex(node)

	if _, ok := visited[nodeKey]; ok {
		return nil
	}

	visited[nodeKey] = true

	ky := utils.ScalarToRoot(node)

	nodeValue, err := s.Db.Get(ky)
	if err != nil {
		return err
	}

	if nodeValue.IsFinalNode() {
		return nil
	}

	for i := 0; i < 2; i++ {
		if len(nodeValue) < i*4+4 {
			return errors.New("nodeValue has insufficient length")
		}
		child := utils.NodeKeyFromBigIntArray(nodeValue[i*4 : i*4+4])
		err := s.traverseAndMark(ctx, child.ToBigInt(), visited)
		if err != nil {
			fmt.Println(err)
		}
	}

	return nil
}