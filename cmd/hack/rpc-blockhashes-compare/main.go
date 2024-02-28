package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"os"

	"github.com/ledgerwatch/erigon/ethclient"
	"gopkg.in/yaml.v2"
)

func main() {
	rpcConfig, err := getConf()
	if err != nil {
		panic(fmt.Sprintf("error RPGCOnfig: %s", err))
	}

	totalChecks := 59100
	wrongChecks := []int{}
	for blockNo := 59000; blockNo < totalChecks; blockNo++ {
		remoteHash := getHash(rpcConfig.Url, blockNo)
		localHash := getHash("http://localhost:8545", blockNo)
		if remoteHash != localHash {
			wrongChecks = append(wrongChecks, blockNo)
		}
	}

	fmt.Println("Check finished.")
	fmt.Printf("Wrong checks: %d/%d, %d%%\n", len(wrongChecks), totalChecks, (len(wrongChecks)/totalChecks)*100)
	if len(wrongChecks) > 0 {
		fmt.Println("Wrong checks:", wrongChecks)
	}

}

func getHash(urlString string, blockNo int) string {
	client, err := ethclient.Dial(urlString)
	if err != nil {
		log.Fatal(err)
	}

	block, err := client.BlockByNumber(context.Background(), big.NewInt(int64(blockNo)))
	if err != nil {
		log.Fatal(err)
	}

	return block.Hash().String()
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
