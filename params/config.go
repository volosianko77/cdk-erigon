// Copyright 2016 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package params

import (
	"embed"
	"encoding/json"
	"fmt"
	"math/big"
	"path"

	erigonchain "github.com/ledgerwatch/erigon-lib/chain"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/chain"
	"github.com/ledgerwatch/erigon/common/paths"
	"github.com/ledgerwatch/erigon/params/networkname"
)

//go:embed chainspecs
var chainspecs embed.FS

func readChainSpec(filename string) *chain.Config {
	f, err := chainspecs.Open(filename)
	if err != nil {
		panic(fmt.Sprintf("Could not open chainspec for %s: %v", filename, err))
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	spec := &chain.Config{}
	err = decoder.Decode(&spec)
	if err != nil {
		panic(fmt.Sprintf("Could not parse chainspec for %s: %v", filename, err))
	}
	return spec
}

// Genesis hashes to enforce below configs on.
var (
	MainnetGenesisHash                 = libcommon.HexToHash("0xd4e56740f876aef8c010b86a40d5f56745a118d0906a34e69aec8c0db1cb8fa3")
	SepoliaGenesisHash                 = libcommon.HexToHash("0x25a5cc106eea7138acab33231d7160d69cb777ee0c2c553fcddf5138993e6dd9")
	RinkebyGenesisHash                 = libcommon.HexToHash("0x6341fd3daf94b748c72ced5a5b26028f2474f5f00d824504e4fa37a75767e177")
	GoerliGenesisHash                  = libcommon.HexToHash("0xbf7e331f7f7c1dd2e05159666b3bf8bc7a8a3a9eb1d518969eab529dd9b88c1a")
	MumbaiGenesisHash                  = libcommon.HexToHash("0x7b66506a9ebdbf30d32b43c5f15a3b1216269a1ec3a75aa3182b86176a2b1ca7")
	BorMainnetGenesisHash              = libcommon.HexToHash("0xa9c28ce2141b56c474f1dc504bee9b01eb1bd7d1a507580d5519d4437a97de1b")
	BorDevnetGenesisHash               = libcommon.HexToHash("0x5a06b25b0c6530708ea0b98a3409290e39dce6be7f558493aeb6e4b99a172a87")
	GnosisGenesisHash                  = libcommon.HexToHash("0x4f1dd23188aab3a76b463e4af801b52b1248ef073c648cbdc4c9333d3da79756")
	ChiadoGenesisHash                  = libcommon.HexToHash("0xada44fd8d2ecab8b08f256af07ad3e777f17fb434f8f8e678b312f576212ba9a")
	HermezMainnetGenesisHash           = libcommon.HexToHash("0x81005434635456a16f74ff7023fbe0bf423abbc8a8deb093ffff455c0ad3b741")
	HermezMainnetShadowforkGenesisHash = libcommon.HexToHash("0xe54709058a084845156393707161a7b3347859b1796167ca014354841f68373c")
	HermezLocalDevnetGenesisHash       = libcommon.HexToHash("0x433043a1b0948d109cd92a6b7e0e5a3f011c761d70eebe3135ec8f7a39815a65")
	HermezCardonaGenesisHash           = libcommon.HexToHash("0x676c1a76a6c5855a32bdf7c61977a0d1510088a4eeac1330466453b3d08b60b9")
	HermezCardonaInternalGenesisHash   = libcommon.HexToHash("0x7311011ce6ab98ef0a15e44fe29f7680909588322534d1736361daa678543038")
	X1TestnetGenesisHash               = libcommon.HexToHash("0x0ffb92e130f1acaabd8b12aa1bb409b46561ef7568cb8aa7eb8d102a6ab76566")
	HermezEtrogGenesisHash             = libcommon.HexToHash("0xccfed260e3ef666b058dcd577551de8e00c743c47774a39ca7dbcd9214ba370a")
)

var (
	GnosisGenesisStateRoot = libcommon.HexToHash("0x40cf4430ecaa733787d1a65154a3b9efb560c95d9e324a23b97f0609b539133b")
	ChiadoGenesisStateRoot = libcommon.HexToHash("0x9ec3eaf4e6188dfbdd6ade76eaa88289b57c63c9a2cde8d35291d5a29e143d31")
)

var (
	// MainnetChainConfig is the chain parameters to run a node on the main network.
	MainnetChainConfig = readChainSpec("chainspecs/mainnet.json")

	// SepoliaChainConfig contains the chain parameters to run a node on the Sepolia test network.
	SepoliaChainConfig = readChainSpec("chainspecs/sepolia.json")

	// RinkebyChainConfig contains the chain parameters to run a node on the Rinkeby test network.
	RinkebyChainConfig = readChainSpec("chainspecs/rinkeby.json")

	// GoerliChainConfig contains the chain parameters to run a node on the Görli test network.
	GoerliChainConfig = readChainSpec("chainspecs/goerli.json")

	// AllProtocolChanges contains every protocol change (EIPs) introduced
	// and accepted by the Ethereum core developers into the main net protocol.
	AllProtocolChanges = &chain.Config{
		ChainID:                       big.NewInt(1337),
		Consensus:                     erigonchain.EtHashConsensus,
		HomesteadBlock:                big.NewInt(0),
		TangerineWhistleBlock:         big.NewInt(0),
		SpuriousDragonBlock:           big.NewInt(0),
		ByzantiumBlock:                big.NewInt(0),
		ConstantinopleBlock:           big.NewInt(0),
		PetersburgBlock:               big.NewInt(0),
		IstanbulBlock:                 big.NewInt(0),
		MuirGlacierBlock:              big.NewInt(0),
		BerlinBlock:                   big.NewInt(0),
		LondonBlock:                   big.NewInt(0),
		ArrowGlacierBlock:             big.NewInt(0),
		GrayGlacierBlock:              big.NewInt(0),
		TerminalTotalDifficulty:       big.NewInt(0),
		TerminalTotalDifficultyPassed: true,
		ShanghaiTime:                  big.NewInt(0),
		Ethash:                        new(erigonchain.EthashConfig),
	}

	// AllCliqueProtocolChanges contains every protocol change (EIPs) introduced
	// and accepted by the Ethereum core developers into the Clique consensus.
	AllCliqueProtocolChanges = &chain.Config{
		ChainID:               big.NewInt(1337),
		Consensus:             erigonchain.CliqueConsensus,
		HomesteadBlock:        big.NewInt(0),
		TangerineWhistleBlock: big.NewInt(0),
		SpuriousDragonBlock:   big.NewInt(0),
		ByzantiumBlock:        big.NewInt(0),
		ConstantinopleBlock:   big.NewInt(0),
		PetersburgBlock:       big.NewInt(0),
		IstanbulBlock:         big.NewInt(0),
		MuirGlacierBlock:      big.NewInt(0),
		BerlinBlock:           big.NewInt(0),
		LondonBlock:           big.NewInt(0),
		Clique:                &erigonchain.CliqueConfig{Period: 0, Epoch: 30000},
	}

	MumbaiChainConfig = readChainSpec("chainspecs/mumbai.json")

	BorMainnetChainConfig = readChainSpec("chainspecs/bor-mainnet.json")

	BorDevnetChainConfig = readChainSpec("chainspecs/bor-devnet.json")

	GnosisChainConfig = readChainSpec("chainspecs/gnosis.json")

	ChiadoChainConfig = readChainSpec("chainspecs/chiado.json")

	HermezMainnetChainConfig = readChainSpec("chainspecs/hermez.json")

	HermezMainnetShadowforkChainConfig = readChainSpec("chainspecs/hermez-shadowfork.json")

	HermezLocalDevnetChainConfig = readChainSpec("chainspecs/hermez-dev.json")

	HermezEtrogChainConfig = readChainSpec("chainspecs/hermez-etrog.json")

	HermezCardonaChainConfig = readChainSpec("chainspecs/hermez-cardona.json")

	HermezCardonaInternalChainConfig = readChainSpec("chainspecs/hermez-cardona-internal.json")

	X1TestnetChainConfig = readChainSpec("chainspecs/x1-testnet.json")

	CliqueSnapshot = NewSnapshotConfig(10, 1024, 16384, true, "")

	TestChainConfig = &chain.Config{
		ChainID:               big.NewInt(1337),
		Consensus:             erigonchain.EtHashConsensus,
		HomesteadBlock:        big.NewInt(0),
		TangerineWhistleBlock: big.NewInt(0),
		SpuriousDragonBlock:   big.NewInt(0),
		ByzantiumBlock:        big.NewInt(0),
		ConstantinopleBlock:   big.NewInt(0),
		PetersburgBlock:       big.NewInt(0),
		IstanbulBlock:         big.NewInt(0),
		MuirGlacierBlock:      big.NewInt(0),
		BerlinBlock:           big.NewInt(0),
		Ethash:                new(erigonchain.EthashConfig),
	}

	TestChainAuraConfig = &chain.Config{
		ChainID:               big.NewInt(1),
		Consensus:             erigonchain.AuRaConsensus,
		HomesteadBlock:        big.NewInt(0),
		TangerineWhistleBlock: big.NewInt(0),
		SpuriousDragonBlock:   big.NewInt(0),
		ByzantiumBlock:        big.NewInt(0),
		ConstantinopleBlock:   big.NewInt(0),
		PetersburgBlock:       big.NewInt(0),
		IstanbulBlock:         big.NewInt(0),
		MuirGlacierBlock:      big.NewInt(0),
		BerlinBlock:           big.NewInt(0),
		LondonBlock:           big.NewInt(0),
		Aura:                  &erigonchain.AuRaConfig{},
	}

	TestRules = TestChainConfig.Rules(0, 0)
)

type ConsensusSnapshotConfig struct {
	CheckpointInterval uint64 // Number of blocks after which to save the vote snapshot to the database
	InmemorySnapshots  int    // Number of recent vote snapshots to keep in memory
	InmemorySignatures int    // Number of recent block signatures to keep in memory
	DBPath             string
	InMemory           bool
}

const cliquePath = "clique"

func NewSnapshotConfig(checkpointInterval uint64, inmemorySnapshots int, inmemorySignatures int, inmemory bool, dbPath string) *ConsensusSnapshotConfig {
	if len(dbPath) == 0 {
		dbPath = paths.DefaultDataDir()
	}

	return &ConsensusSnapshotConfig{
		checkpointInterval,
		inmemorySnapshots,
		inmemorySignatures,
		path.Join(dbPath, cliquePath),
		inmemory,
	}
}

func ChainConfigByChainName(chain string) *chain.Config {
	switch chain {
	case networkname.MainnetChainName:
		return MainnetChainConfig
	case networkname.SepoliaChainName:
		return SepoliaChainConfig
	case networkname.RinkebyChainName:
		return RinkebyChainConfig
	case networkname.GoerliChainName:
		return GoerliChainConfig
	case networkname.MumbaiChainName:
		return MumbaiChainConfig
	case networkname.BorMainnetChainName:
		return BorMainnetChainConfig
	case networkname.BorDevnetChainName:
		return BorDevnetChainConfig
	case networkname.GnosisChainName:
		return GnosisChainConfig
	case networkname.ChiadoChainName:
		return ChiadoChainConfig
	case networkname.HermezMainnetChainName:
		return HermezMainnetChainConfig
	case networkname.HermezMainnetShadowforkChainName:
		return HermezMainnetShadowforkChainConfig
	case networkname.HermezLocalDevnetChainName:
		return HermezLocalDevnetChainConfig
	case networkname.HermezEtrogChainName:
		return HermezEtrogChainConfig
	case networkname.HermezCardonaChainName:
		return HermezCardonaChainConfig
	case networkname.HermezCardonaInternalChainName:
		return HermezCardonaInternalChainConfig
	case networkname.X1TestnetChainName:
		return X1TestnetChainConfig
	default:
		return nil
	}
}

func GenesisHashByChainName(chain string) *libcommon.Hash {
	switch chain {
	case networkname.MainnetChainName:
		return &MainnetGenesisHash
	case networkname.SepoliaChainName:
		return &SepoliaGenesisHash
	case networkname.RinkebyChainName:
		return &RinkebyGenesisHash
	case networkname.GoerliChainName:
		return &GoerliGenesisHash
	case networkname.MumbaiChainName:
		return &MumbaiGenesisHash
	case networkname.BorMainnetChainName:
		return &BorMainnetGenesisHash
	case networkname.BorDevnetChainName:
		return &BorDevnetGenesisHash
	case networkname.GnosisChainName:
		return &GnosisGenesisHash
	case networkname.ChiadoChainName:
		return &ChiadoGenesisHash
	case networkname.HermezMainnetChainName:
		return &HermezMainnetGenesisHash
	case networkname.HermezMainnetShadowforkChainName:
		return &HermezMainnetShadowforkGenesisHash
	case networkname.HermezLocalDevnetChainName:
		return &HermezLocalDevnetGenesisHash
	case networkname.HermezEtrogChainName:
		return &HermezEtrogGenesisHash
	case networkname.HermezCardonaChainName:
		return &HermezCardonaGenesisHash
	case networkname.HermezCardonaInternalChainName:
		return &HermezCardonaInternalGenesisHash
	case networkname.X1TestnetChainName:
		return &X1TestnetGenesisHash
	default:
		return nil
	}
}

func ChainConfigByGenesisHash(genesisHash libcommon.Hash) *chain.Config {
	switch {
	case genesisHash == MainnetGenesisHash:
		return MainnetChainConfig
	case genesisHash == SepoliaGenesisHash:
		return SepoliaChainConfig
	case genesisHash == RinkebyGenesisHash:
		return RinkebyChainConfig
	case genesisHash == GoerliGenesisHash:
		return GoerliChainConfig
	case genesisHash == MumbaiGenesisHash:
		return MumbaiChainConfig
	case genesisHash == BorMainnetGenesisHash:
		return BorMainnetChainConfig
	case genesisHash == BorDevnetGenesisHash:
		return BorDevnetChainConfig
	case genesisHash == GnosisGenesisHash:
		return GnosisChainConfig
	case genesisHash == ChiadoGenesisHash:
		return ChiadoChainConfig
	case genesisHash == HermezMainnetGenesisHash:
		return HermezMainnetChainConfig
	case genesisHash == HermezMainnetShadowforkGenesisHash:
		return HermezMainnetShadowforkChainConfig
	case genesisHash == HermezLocalDevnetGenesisHash:
		return HermezLocalDevnetChainConfig
	case genesisHash == HermezCardonaGenesisHash:
		return HermezCardonaChainConfig
	case genesisHash == HermezCardonaInternalGenesisHash:
		return HermezCardonaInternalChainConfig
	default:
		panic(fmt.Sprintf("Unknown genesis hash: %s", genesisHash.Hex()))
		return nil
	}
}

func NetworkIDByChainName(chain string) uint64 {
	switch chain {
	case networkname.DevChainName:
		return 1337
	default:
		config := ChainConfigByChainName(chain)
		if config == nil {
			return 0
		}
		return config.ChainID.Uint64()
	}
}
