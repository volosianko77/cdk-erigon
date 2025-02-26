package stages

import (
	"context"
	"errors"
	"fmt"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/log/v3"

	"math/big"

	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/core/rawdb"
	ethTypes "github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/eth/ethconfig"
	"github.com/ledgerwatch/erigon/eth/stagedsync"
	"github.com/ledgerwatch/erigon/eth/stagedsync/stages"
	"github.com/ledgerwatch/erigon/zk/contracts"
	"github.com/ledgerwatch/erigon/zk/hermez_db"
	"github.com/ledgerwatch/erigon/zk/sequencer"
	"github.com/ledgerwatch/erigon/zk/types"
)

type IL1Syncer interface {

	// atomic
	IsSyncStarted() bool
	IsDownloading() bool
	GetLastCheckedL1Block() uint64

	// Channels
	GetLogsChan() chan ethTypes.Log
	GetProgressMessageChan() chan string

	Run(lastCheckedBlock uint64)
}

var ErrStateRootMismatch = fmt.Errorf("state root mismatch")

type L1SyncerCfg struct {
	db     kv.RwDB
	syncer IL1Syncer

	zkCfg *ethconfig.Zk
}

func StageL1SyncerCfg(db kv.RwDB, syncer IL1Syncer, zkCfg *ethconfig.Zk) L1SyncerCfg {
	return L1SyncerCfg{
		db:     db,
		syncer: syncer,
		zkCfg:  zkCfg,
	}
}

func SpawnStageL1Syncer(
	s *stagedsync.StageState,
	u stagedsync.Unwinder,
	ctx context.Context,
	tx kv.RwTx,
	cfg L1SyncerCfg,
	firstCycle bool,
	quiet bool,
) error {

	logPrefix := s.LogPrefix()
	log.Info(fmt.Sprintf("[%s] Starting L1 sync stage", logPrefix))
	if sequencer.IsSequencer() {
		log.Info(fmt.Sprintf("[%s] skipping -- sequencer", logPrefix))
		return nil
	}
	defer log.Info(fmt.Sprintf("[%s] Finished L1 sync stage ", logPrefix))

	if tx == nil {
		log.Debug("l1 sync: no tx provided, creating a new one")
		var err error
		tx, err = cfg.db.BeginRw(ctx)
		if err != nil {
			return fmt.Errorf("failed to open tx, %w", err)
		}
		defer tx.Rollback()
	}

	// pass tx to the hermezdb
	hermezDb := hermez_db.NewHermezDb(tx)

	// get l1 block progress from this stage's progress
	l1BlockProgress, err := stages.GetStageProgress(tx, stages.L1Syncer)
	if err != nil {
		return fmt.Errorf("failed to get l1 progress block, %w", err)
	}

	// start syncer if not started
	if !cfg.syncer.IsSyncStarted() {
		if l1BlockProgress == 0 {
			l1BlockProgress = cfg.zkCfg.L1FirstBlock - 1
		}

		// start the syncer
		cfg.syncer.Run(l1BlockProgress)
	}

	logsChan := cfg.syncer.GetLogsChan()
	progressMessageChan := cfg.syncer.GetProgressMessageChan()
	highestVerification := types.L1BatchInfo{}

	newVerificationsCount := 0
	newSequencesCount := 0
Loop:
	for {
		select {
		case l := <-logsChan:
			info := convertResultToBatchInfo(&l)
			if l.Topics[0] == contracts.SequencedBatchTopic {
				err = hermezDb.WriteSequence(info.L1BlockNo, info.BatchNo, info.L1TxHash, info.StateRoot)
				if err != nil {
					return fmt.Errorf("failed to write batch info, %w", err)
				}
				newSequencesCount++
			} else if l.Topics[0] == contracts.VerificationTopic {
				stateRootData := l.Data[:32]
				stateRoot := common.BytesToHash(stateRootData)
				info.StateRoot = stateRoot
				if info.BatchNo > highestVerification.BatchNo {
					highestVerification = info
				}
				if err := hermezDb.WriteVerification(info.L1BlockNo, info.BatchNo, info.L1TxHash, info.StateRoot); err != nil {
					return fmt.Errorf("failed to write verification for block %d, %w", info.L1BlockNo, err)
				}
				newVerificationsCount++
			} else {
				log.Warn("L1 Syncer unknown topic", "topic", l.Topics[0])
			}
		case progressMessage := <-progressMessageChan:
			log.Info(fmt.Sprintf("[%s] %s", logPrefix, progressMessage))
		default:
			if !cfg.syncer.IsDownloading() {
				break Loop
			}
		}
	}

	latestCheckedBlock := cfg.syncer.GetLastCheckedL1Block()
	if latestCheckedBlock > l1BlockProgress {
		log.Info(fmt.Sprintf("[%s] Saving L1 syncer progress", logPrefix), "latestCheckedBlock", latestCheckedBlock, "newVerificationsCount", newVerificationsCount, "newSequencesCount", newSequencesCount)

		if err := stages.SaveStageProgress(tx, stages.L1Syncer, latestCheckedBlock); err != nil {
			return fmt.Errorf("failed to save stage progress, %w", err)
		}
		if highestVerification.BatchNo > 0 {
			log.Info(fmt.Sprintf("[%s]", logPrefix), "highestVerificationBatchNo", highestVerification.BatchNo)
			if err := stages.SaveStageProgress(tx, stages.L1VerificationsBatchNo, highestVerification.BatchNo); err != nil {
				return fmt.Errorf("failed to save stage progress, %w", err)
			}
		}

		// State Root Verifications Check
		err = verifyAgainstLocalBlocks(tx, hermezDb, logPrefix)
		if err != nil {
			if errors.Is(err, ErrStateRootMismatch) {
				panic(err)
			}
			// do nothing in hope the node will recover if it isn't a stateroot mismatch
		}
	} else {
		log.Info(fmt.Sprintf("[%s] No new L1 blocks to sync", logPrefix))
	}

	if firstCycle {
		log.Debug("l1 sync: first cycle, committing tx")
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit tx, %w", err)
		}
	}

	return nil
}

func convertResultToBatchInfo(log *ethTypes.Log) types.L1BatchInfo {
	batchNumber := new(big.Int).SetBytes(log.Topics[1].Bytes())
	l1TxHash := common.BytesToHash(log.TxHash.Bytes())
	blockNumber := log.BlockNumber
	return types.L1BatchInfo{
		BatchNo:   batchNumber.Uint64(),
		L1BlockNo: blockNumber,
		L1TxHash:  l1TxHash,
	}
}

func UnwindL1SyncerStage(u *stagedsync.UnwindState, tx kv.RwTx, cfg L1SyncerCfg, ctx context.Context) (err error) {
	useExternalTx := tx != nil
	if !useExternalTx {
		tx, err = cfg.db.BeginRw(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	// TODO: implement unwind verifications stage!

	if err := u.Done(tx); err != nil {
		return err
	}
	if !useExternalTx {
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func PruneL1SyncerStage(s *stagedsync.PruneState, tx kv.RwTx, cfg L1SyncerCfg, ctx context.Context) (err error) {
	useExternalTx := tx != nil
	if !useExternalTx {
		tx, err = cfg.db.BeginRw(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	// TODO: implement prune L1 Verifications stage! (if required)

	if !useExternalTx {
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func verifyAgainstLocalBlocks(tx kv.RwTx, hermezDb *hermez_db.HermezDb, logPrefix string) (err error) {
	// get the highest hashed block
	hashedBlockNo, err := stages.GetStageProgress(tx, stages.IntermediateHashes)
	if err != nil {
		return fmt.Errorf("failed to get highest hashed block, %w", err)
	}

	// no need to check - interhashes has not yet run
	if hashedBlockNo == 0 {
		return nil
	}

	// get the highest verified block
	verifiedBlockNo, err := hermezDb.GetHighestVerifiedBlockNo()
	if err != nil {
		return fmt.Errorf("failed to get highest verified block no, %w", err)
	}

	// no verifications on l1
	if verifiedBlockNo == 0 {
		return nil
	}

	// 3 scenarios:
	//     1. verified and node both equal
	//     2. node behind l1 - verification block is higher than hashed block - use hashed block to find verification block
	//     3. l1 behind node - verification block is lower than hashed block - use verification block to find hashed block
	var blockToCheck uint64
	if verifiedBlockNo <= hashedBlockNo {
		blockToCheck = verifiedBlockNo
	} else {
		// in this case we need to find the blocknumber that is highest for the last batch
		// get the batch of the last hashed block
		hashedBatch, err := hermezDb.GetBatchNoByL2Block(hashedBlockNo)
		if err != nil {
			return err
		}

		if hashedBatch == 0 {
			log.Warn(fmt.Sprintf("[%s] No batch number found for block %d", logPrefix, hashedBlockNo))
			return nil
		}

		// we don't know if this is the latest block in this batch, so check for the previous one
		// find the higher blocknum for previous batch
		blockNumbers, err := hermezDb.GetL2BlockNosByBatch(hashedBatch)
		if err != nil {
			return err
		}

		if len(blockNumbers) == 0 {
			log.Warn(fmt.Sprintf("[%s] No block numbers found for batch %d", logPrefix, hashedBatch))
			return nil
		}

		for _, num := range blockNumbers {
			if num > blockToCheck {
				blockToCheck = num
			}
		}
	}

	// already checked
	highestChecked, err := stages.GetStageProgress(tx, stages.VerificationsStateRootCheck)
	if err != nil {
		return fmt.Errorf("failed to get highest checked block, %w", err)
	}
	if highestChecked >= blockToCheck {
		return nil
	}

	err = blockComparison(tx, hermezDb, blockToCheck, logPrefix)

	if err == nil {
		log.Info(fmt.Sprintf("[%s] State root verified in block %d", logPrefix, blockToCheck))
		if err := stages.SaveStageProgress(tx, stages.VerificationsStateRootCheck, verifiedBlockNo); err != nil {
			return fmt.Errorf("failed to save stage progress, %w", err)
		}
	}

	return err
}

func blockComparison(tx kv.RwTx, hermezDb *hermez_db.HermezDb, blockNo uint64, logPrefix string) error {
	v, err := hermezDb.GetVerificationByL2BlockNo(blockNo)
	if err != nil {
		return fmt.Errorf("failed to get verification by l2 block no, %w", err)
	}

	block, err := rawdb.ReadBlockByNumber(tx, blockNo)
	if err != nil {
		return fmt.Errorf("failed to read block by number, %w", err)
	}

	if v == nil || block == nil {
		log.Info("block or verification is nil", "block", block, "verification", v)
		return nil
	}

	if v.StateRoot != block.Root() {
		log.Error(fmt.Sprintf("[%s] State root mismatch in block %d", logPrefix, blockNo))
		return ErrStateRootMismatch
	}

	return nil
}

func min(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}
