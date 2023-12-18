package main

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/ledgerwatch/erigon/zk/datastream"
)

func generateTestCases(maxBlock uint64, blockIncrement uint64) map[string]struct {
	fromBlock uint64
	toBlock   uint64
} {
	testCases := make(map[string]struct {
		fromBlock uint64
		toBlock   uint64
	})

	maxBlock = maxBlock
	blockIncrement = blockIncrement

	for fromBlock := uint64(0); fromBlock < maxBlock; fromBlock += blockIncrement {
		toBlock := fromBlock + blockIncrement
		testName := fmt.Sprintf("test_block_%d_%d", fromBlock, toBlock)
		testCases[testName] = struct {
			fromBlock uint64
			toBlock   uint64
		}{
			fromBlock: fromBlock,
			toBlock:   toBlock,
		}
	}

	return testCases
}

func parseUintEnvVariable(t *testing.T, envVarName string) uint64 {
	envVarValue := os.Getenv(envVarName)
	value, err := strconv.ParseUint(envVarValue, 10, 64)
	if err != nil {
		t.Fatalf("Error parsing %s: %v", envVarName, err)
	}
	return value
}

func TestDatastream(t *testing.T) {

	streamUrl := os.Getenv("streamUrl")
	if streamUrl == "" {
		t.Fatal("streamUrl environment variable not provided")
	}

	maxBlock := parseUintEnvVariable(t, "maxBlock")
	blockIncrement := parseUintEnvVariable(t, "blockIncrement")

	var testCases = generateTestCases(maxBlock, blockIncrement)
	for test_name, tc := range testCases {
		tc := tc

		t.Run(test_name, func(t *testing.T) {

			t.Parallel()

			l2Blocks, _, _, _, _ := datastream.DownloadL2Blocks(streamUrl, tc.fromBlock, int(blockIncrement))

			var expectedBlockNumber uint64 = tc.fromBlock
			var missingBlocks []uint64

			for _, l2Block := range *l2Blocks {
				if expectedBlockNumber != l2Block.L2BlockNumber {
					missingBlocks = append(missingBlocks, l2Block.L2BlockNumber)
				}
				expectedBlockNumber++
			}
			if len(missingBlocks) != 0 {
				t.Fail()
				t.Logf("There are '%d' of missing blocks", len(missingBlocks))
			}
		})
	}
}
