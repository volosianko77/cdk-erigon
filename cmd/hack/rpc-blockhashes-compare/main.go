package main

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"reflect"

	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/log/v3"

	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/ethclient"
	"gopkg.in/yaml.v2"
)

// compare block hashes and binary search the first block where they mismatch
// then print the block number and the field differences
func main() {
	rpcConfig, err := getConf()
	if err != nil {
		panic(fmt.Sprintf("error RPGCOnfig: %s", err))
	}

	rpcClientRemote, err := ethclient.Dial(rpcConfig.Url)
	if err != nil {
		panic(fmt.Sprintf("error ethclient.Dial: %s", err))
	}
	rpcClientLocal, err := ethclient.Dial("http://localhost:8545")
	if err != nil {
		panic(fmt.Sprintf("error ethclient.Dial: %s", err))
	}

	// highest block number
	highestBlockRemote, err := rpcClientRemote.BlockNumber(context.Background())
	if err != nil {
		panic(fmt.Sprintf("error rpcClientRemote.BlockNumber: %s", err))
	}
	highestBlockLocal, err := rpcClientLocal.BlockNumber(context.Background())
	if err != nil {
		panic(fmt.Sprintf("error rpcClientLocal.BlockNumber: %s", err))
	}
	highestBlockNumber := highestBlockRemote
	if highestBlockLocal < highestBlockRemote {
		highestBlockNumber = highestBlockLocal
	}

	log.Warn("Starting blockhash mismatch check", "highestBlockRemote", highestBlockRemote, "highestBlockLocal", highestBlockLocal, "working highestBlockNumber", highestBlockNumber)

	lowestBlockNumber := uint64(0)
	checkBlockNumber := highestBlockNumber

	var blockRemote, blockLocal *types.Block

	for {
		// get blocks
		blockRemote, blockLocal, err = getBlocks(*rpcClientLocal, *rpcClientRemote, checkBlockNumber)
		if err != nil {
			log.Error(fmt.Sprintf("blockNum: %d, error getBlocks: %s", checkBlockNumber, err))
			return
		}
		// if they match, go higher
		if blockRemote.Hash() == blockLocal.Hash() {
			lowestBlockNumber = checkBlockNumber + 1
		} else {
			highestBlockNumber = checkBlockNumber
		}

		checkBlockNumber = (lowestBlockNumber + highestBlockNumber) / 2
		if lowestBlockNumber >= highestBlockNumber {
			break
		}
	}

	// get blocks
	blockRemote, blockLocal, err = getBlocks(*rpcClientLocal, *rpcClientRemote, checkBlockNumber)
	if err != nil {
		log.Error(fmt.Sprintf("blockNum: %d, error getBlocks: %s", checkBlockNumber, err))
		return
	}

	if blockRemote.Hash() != blockLocal.Hash() {
		log.Warn("Blockhash mismatch", "blockNum", checkBlockNumber, "blockRemote.Hash", blockRemote.Hash().Hex(), "blockLocal.Hash", blockLocal.Hash().Hex())

		// check all fields
		if blockRemote.ParentHash() != blockLocal.ParentHash() {
			log.Warn("ParentHash", "Rpc", blockRemote.ParentHash().Hex(), "Rpc", blockLocal.ParentHash().Hex())
		}
		if blockRemote.UncleHash() != blockLocal.UncleHash() {
			log.Warn("UnclesHash", "Rpc", blockRemote.UncleHash().Hex(), "Local", blockLocal.UncleHash().Hex())
		}
		if blockRemote.Root() != blockLocal.Root() {
			log.Warn("Root", "Rpc", blockRemote.Root().Hex(), "Local", blockLocal.Root().Hex())
		}
		if blockRemote.TxHash() != blockLocal.TxHash() {
			log.Warn("TxHash", "Rpc", blockRemote.TxHash().Hex(), "Local", blockLocal.TxHash().Hex())
		}
		if blockRemote.ReceiptHash() != blockLocal.ReceiptHash() {
			log.Warn("ReceiptHash", "Rpc", blockRemote.ReceiptHash().Hex(), "Local", blockLocal.ReceiptHash().Hex())
			for _, tx := range blockRemote.Transactions() {
				receiptLocal, receiptRpc, err := getReceipt(*rpcClientLocal, *rpcClientRemote, tx.Hash())
				if err != nil {
					log.Error(fmt.Sprintf("error getReceipt: %s", err))
					return
				}
				log.Warn("-------------------------------------------------")
				log.Warn("Checking Receipts for tx.", "TxHash", tx.Hash().Hex())
				log.Warn("-------------------------------------------------")

				if receiptLocal.Status != receiptRpc.Status {
					log.Warn("ReceiptStatus", "Rpc", receiptRpc.Status, "Local", receiptLocal.Status)
				}
				if receiptLocal.CumulativeGasUsed != receiptRpc.CumulativeGasUsed {
					log.Warn("CumulativeGasUsed", "Rpc", receiptRpc.CumulativeGasUsed, "Local", receiptLocal.CumulativeGasUsed)
				}
				if !reflect.DeepEqual(receiptLocal.PostState, receiptRpc.PostState) {
					log.Warn("PostState", "Rpc", receiptRpc.PostState, "Local", receiptLocal.PostState)
				}
				if receiptLocal.ContractAddress != receiptRpc.ContractAddress {
					log.Warn("ContractAddress", "Rpc", receiptRpc.ContractAddress, "Local", receiptLocal.ContractAddress)
				}
				if receiptLocal.GasUsed != receiptRpc.GasUsed {
					log.Warn("GasUsed", "Rpc", receiptRpc.GasUsed, "Local", receiptLocal.GasUsed)
				}
				if receiptLocal.Bloom != receiptRpc.Bloom {
					log.Warn("LogsBloom", "Rpc", receiptRpc.Bloom, "Local", receiptLocal.Bloom)
				}

				for i, logRemote := range receiptRpc.Logs {
					logLocal := receiptLocal.Logs[i]
					if logRemote.Address != logLocal.Address {
						log.Warn("Log Address", "index", i, "Rpc", logRemote.Address, "Local", logLocal.Address)
					}
					if !reflect.DeepEqual(logRemote.Data, logLocal.Data) {
						log.Warn("Log Data", "index", i, "Rpc", logRemote.Data, "Local", logLocal.Data)
					}

					for j, topic := range logRemote.Topics {
						topicLocal := logLocal.Topics[j]
						if topic != topicLocal {
							log.Warn("Log Topic", "Log index", i, "Topic index", j, "Rpc", topic, "Local", topicLocal)
						}
					}
				}
				log.Warn("-------------------------------------------------")
				log.Warn("Finished tx check")
				log.Warn("-------------------------------------------------")
			}
		}
		if blockRemote.Bloom() != blockLocal.Bloom() {
			log.Warn("Bloom", "Rpc", blockRemote.Bloom(), "Local", blockLocal.Bloom())
		}
		if blockRemote.Difficulty().Cmp(blockLocal.Difficulty()) != 0 {
			log.Warn("Difficulty", "Rpc", blockRemote.Difficulty().Uint64(), "Local", blockLocal.Difficulty().Uint64())
		}
		if blockRemote.NumberU64() != blockLocal.NumberU64() {
			log.Warn("NumberU64", "Rpc", blockRemote.NumberU64(), "Local", blockLocal.NumberU64())
		}
		if blockRemote.GasLimit() != blockLocal.GasLimit() {
			log.Warn("GasLimit", "Rpc", blockRemote.GasLimit(), "Local", blockLocal.GasLimit())
		}
		if blockRemote.GasUsed() != blockLocal.GasUsed() {
			log.Warn("GasUsed", "Rpc", blockRemote.GasUsed(), "Local", blockLocal.GasUsed())
		}
		if blockRemote.Time() != blockLocal.Time() {
			log.Warn("Time", "Rpc", blockRemote.Time(), "Local", blockLocal.Time())
		}
		if blockRemote.MixDigest() != blockLocal.MixDigest() {
			log.Warn("MixDigest", "Rpc", blockRemote.MixDigest().Hex(), "Local", blockLocal.MixDigest().Hex())
		}
		if blockRemote.Nonce() != blockLocal.Nonce() {
			log.Warn("Nonce", "Rpc", blockRemote.Nonce(), "Local", blockLocal.Nonce())
		}
		if blockRemote.BaseFee() != blockLocal.BaseFee() {
			log.Warn("BaseFee", "Rpc", blockRemote.BaseFee(), "Local", blockLocal.BaseFee())
		}

		for i, txRemote := range blockRemote.Transactions() {
			txLocal := blockLocal.Transactions()[i]
			if txRemote.Hash() != txLocal.Hash() {
				log.Warn("TxHash", txRemote.Hash().Hex(), txLocal.Hash().Hex())
			}
		}
	}

	log.Warn("Check finished!")
}

func getReceipt(clientLocal, clientRemote ethclient.Client, txHash common.Hash) (*types.Receipt, *types.Receipt, error) {
	receiptsLocal, err := clientLocal.TransactionReceipt(context.Background(), txHash)
	if err != nil {
		return nil, nil, fmt.Errorf("error clientLocal.TransactionReceipts: %s", err)
	}
	receiptsRemote, err := clientRemote.TransactionReceipt(context.Background(), txHash)
	if err != nil {
		return nil, nil, fmt.Errorf("error clientRemote.TransactionReceipts: %s", err)
	}
	return receiptsLocal, receiptsRemote, nil
}

func getBlocks(clientLocal, clientRemote ethclient.Client, blockNum uint64) (*types.Block, *types.Block, error) {
	blockNumBig := new(big.Int).SetUint64(blockNum)
	blockLocal, err := clientLocal.BlockByNumber(context.Background(), blockNumBig)
	if err != nil {
		return nil, nil, fmt.Errorf("error clientLocal.BlockByNumber: %s", err)
	}
	blockRemote, err := clientRemote.BlockByNumber(context.Background(), blockNumBig)
	if err != nil {
		return nil, nil, fmt.Errorf("error clientRemote.BlockByNumber: %s", err)
	}
	return blockLocal, blockRemote, nil
}

type RpcConfig struct {
	Url          string `yaml:"url"`
	DumpFileName string `yaml:"dumpFileName"`
	Block        string `yaml:"block"`
}

func getConf() (RpcConfig, error) {
	yamlFile, err := os.ReadFile("debugToolsConfig.yaml")
	if err != nil {
		return RpcConfig{}, err
	}

	c := RpcConfig{}
	err = yaml.Unmarshal(yamlFile, &c)
	if err != nil {
		return RpcConfig{}, err
	}

	return c, nil
}
