package vm

import (
	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/zk/hermez_db"
	"math"
	"math/big"
	"strconv"
)

var totalSteps = math.Pow(2, 23)

const (
	MCPL    = 23
	fnecHex = "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141"
	fpecHex = "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2F"
)

type Counter struct {
	remaining     int
	used          int
	name          string
	initialAmount int
}

func (c *Counter) Used() int { return c.used }

type Counters map[CounterKey]*Counter

type CounterKey string

var (
	S   CounterKey = "S"
	A   CounterKey = "A"
	B   CounterKey = "B"
	M   CounterKey = "M"
	K   CounterKey = "K"
	D   CounterKey = "D"
	P   CounterKey = "P"
	SHA CounterKey = "SHA"
)

type CounterManager struct {
	currentCounters    Counters
	currentTransaction types.Transaction
	historicalCounters []Counters
	calls              [256]executionFunc
	smtMaxLevel        int64
	smtLevels          int
	transactionStore   []types.Transaction
}

type CounterCollector struct {
	counters    Counters
	smtLevels   int
	isDeploy    bool
	transaction types.Transaction
}

func calculateSmtLevels(smtMaxLevel uint32) int {
	return len(strconv.FormatInt(int64(math.Pow(2, float64(smtMaxLevel))+250000), 2))
}

func NewCounterCollector(smtLevels int) *CounterCollector {
	return &CounterCollector{
		counters:  defaultCounters(),
		smtLevels: smtLevels,
	}
}

func (cc *CounterCollector) Deduct(key CounterKey, amount int) {
	cc.counters[key].used += amount
	cc.counters[key].remaining -= amount
}

func defaultCounters() Counters {
	return Counters{
		S: {
			remaining:     int(totalSteps),
			name:          "totalSteps",
			initialAmount: int(totalSteps),
		},
		A: {
			remaining:     int(math.Floor(totalSteps / 32)),
			name:          "arith",
			initialAmount: int(math.Floor(totalSteps / 32)),
		},
		B: {
			remaining:     int(math.Floor(totalSteps / 16)),
			name:          "binary",
			initialAmount: int(math.Floor(totalSteps / 16)),
		},
		M: {
			remaining:     int(math.Floor(totalSteps / 32)),
			name:          "memAlign",
			initialAmount: int(math.Floor(totalSteps / 32)),
		},
		K: {
			remaining:     int(math.Floor(totalSteps/155286) * 44),
			name:          "keccaks",
			initialAmount: int(math.Floor(totalSteps/155286) * 44),
		},
		D: {
			remaining:     int(math.Floor(totalSteps / 56)),
			name:          "padding",
			initialAmount: int(math.Floor(totalSteps / 56)),
		},
		P: {
			remaining:     int(math.Floor(totalSteps / 30)),
			name:          "poseidon",
			initialAmount: int(math.Floor(totalSteps / 30)),
		},
		SHA: {
			remaining:     int(math.Floor(totalSteps-1)/31488) * 7,
			name:          "sha256",
			initialAmount: int(math.Floor(totalSteps-1)/31488) * 7,
		},
	}
}

func (cc *CounterCollector) Counters() Counters {
	return cc.counters
}

func (cc *CounterCollector) SetTransaction(transaction types.Transaction) {
	cc.transaction = transaction
	cc.isDeploy = transaction.IsContractDeploy()
}

func WrapJumpTableWithZkCounters(originalTable *JumpTable, counterCalls *[256]executionFunc) *JumpTable {
	wrapper := func(original, counter executionFunc) executionFunc {
		return func(p *uint64, i *EVMInterpreter, s *ScopeContext) ([]byte, error) {
			b, err := counter(p, i, s)
			if err != nil {
				return b, err
			}
			return original(p, i, s)
		}
	}

	result := &JumpTable{}

	for idx := range originalTable {
		original := originalTable[idx]
		// if we have something in the Counter table to process wrap the function call
		if counterCalls[idx] != nil {
			originalExec := originalTable[idx].execute
			counterExec := counterCalls[idx]
			wrappedExec := wrapper(originalExec, counterExec)
			original.execute = wrappedExec
		}
		result[idx] = original
	}

	return result
}

func SimpleCounterOperations(cc *CounterCollector) *[256]executionFunc {
	calls := &[256]executionFunc{
		ADD:            cc.opAdd,
		MUL:            cc.opMul,
		SUB:            cc.opSub,
		DIV:            cc.opDiv,
		SDIV:           cc.opSDiv,
		MOD:            cc.opMod,
		SMOD:           cc.opSMod,
		ADDMOD:         cc.opAddMod,
		MULMOD:         cc.opMulMod,
		EXP:            cc.opExp,
		SIGNEXTEND:     cc.opSignExtend,
		BLOCKHASH:      cc.opBlockHash,
		COINBASE:       cc.opCoinbase,
		TIMESTAMP:      cc.opTimestamp,
		NUMBER:         cc.opNumber,
		DIFFICULTY:     cc.opDifficulty,
		GASLIMIT:       cc.opGasLimit,
		CHAINID:        cc.opChainId,
		CALLDATALOAD:   cc.opCalldataLoad,
		CALLDATASIZE:   cc.opCalldataSize,
		CALLDATACOPY:   cc.opCalldataCopy,
		CODESIZE:       cc.opCodeSize,
		EXTCODESIZE:    cc.opExtCodeSize,
		EXTCODECOPY:    cc.opExtCodeCopy,
		CODECOPY:       cc.opCodeCopy,
		RETURNDATASIZE: cc.opReturnDataSize,
		RETURNDATACOPY: cc.opReturnDataCopy,
		EXTCODEHASH:    cc.opExtCodeHash,
		LT:             cc.opLT,
		GT:             cc.opGT,
		SLT:            cc.opSLT,
		SGT:            cc.opSGT,
		EQ:             cc.opEQ,
		ISZERO:         cc.opIsZero,
		AND:            cc.opAnd,
		OR:             cc.opOr,
		XOR:            cc.opXor,
		NOT:            cc.opNot,
		BYTE:           cc.opByte,
		SHR:            cc.opSHR,
		SHL:            cc.opSHL,
		SAR:            cc.opSAR,
		STOP:           cc.opStop,
		CREATE:         cc.opCreate,
		CALL:           cc.opCall,
		CALLCODE:       cc.opCallCode,
		DELEGATECALL:   cc.opDelegateCall,
		STATICCALL:     cc.opStaticCall,
		CREATE2:        cc.opCreate2,
		RETURN:         cc.opReturn,
		REVERT:         cc.opRevert,
		SENDALL:        cc.opSendAll,
		INVALID:        cc.opInvalid,
		ADDRESS:        cc.opAddress,
		SELFBALANCE:    cc.opSelfBalance,
		ORIGIN:         cc.opOrigin,
		CALLER:         cc.opCaller,
		CALLVALUE:      cc.opCallValue,
		GASPRICE:       cc.opGasPrice,
		KECCAK256:      cc.opSha3,
		JUMP:           cc.opJump,
		JUMPI:          cc.opJumpI,
		PC:             cc.opPC,
		JUMPDEST:       cc.opJumpDest,
		LOG0:           cc.opLog0,
		LOG1:           cc.opLog1,
		LOG2:           cc.opLog2,
		LOG3:           cc.opLog3,
		LOG4:           cc.opLog4,
		PUSH0:          cc.opPush0,
		PUSH1:          cc.opPushGenerator(1),
		PUSH2:          cc.opPushGenerator(2),
		PUSH3:          cc.opPushGenerator(3),
		PUSH4:          cc.opPushGenerator(4),
		PUSH5:          cc.opPushGenerator(5),
		PUSH6:          cc.opPushGenerator(6),
		PUSH7:          cc.opPushGenerator(7),
		PUSH8:          cc.opPushGenerator(8),
		PUSH9:          cc.opPushGenerator(9),
		PUSH10:         cc.opPushGenerator(10),
		PUSH11:         cc.opPushGenerator(11),
		PUSH12:         cc.opPushGenerator(12),
		PUSH13:         cc.opPushGenerator(13),
		PUSH14:         cc.opPushGenerator(14),
		PUSH15:         cc.opPushGenerator(15),
		PUSH16:         cc.opPushGenerator(16),
		PUSH17:         cc.opPushGenerator(17),
		PUSH18:         cc.opPushGenerator(18),
		PUSH19:         cc.opPushGenerator(19),
		PUSH20:         cc.opPushGenerator(20),
		PUSH21:         cc.opPushGenerator(21),
		PUSH22:         cc.opPushGenerator(22),
		PUSH23:         cc.opPushGenerator(23),
		PUSH24:         cc.opPushGenerator(24),
		PUSH25:         cc.opPushGenerator(25),
		PUSH26:         cc.opPushGenerator(26),
		PUSH27:         cc.opPushGenerator(27),
		PUSH28:         cc.opPushGenerator(28),
		PUSH29:         cc.opPushGenerator(29),
		PUSH30:         cc.opPushGenerator(30),
		PUSH31:         cc.opPushGenerator(31),
		PUSH32:         cc.opPushGenerator(32),
		DUP1:           cc.opDup,
		DUP2:           cc.opDup,
		DUP3:           cc.opDup,
		DUP4:           cc.opDup,
		DUP5:           cc.opDup,
		DUP6:           cc.opDup,
		DUP7:           cc.opDup,
		DUP8:           cc.opDup,
		DUP9:           cc.opDup,
		DUP10:          cc.opDup,
		DUP11:          cc.opDup,
		DUP12:          cc.opDup,
		DUP13:          cc.opDup,
		DUP14:          cc.opDup,
		DUP15:          cc.opDup,
		DUP16:          cc.opDup,
		SWAP1:          cc.opSwap,
		SWAP2:          cc.opSwap,
		SWAP3:          cc.opSwap,
		SWAP4:          cc.opSwap,
		SWAP5:          cc.opSwap,
		SWAP6:          cc.opSwap,
		SWAP7:          cc.opSwap,
		SWAP8:          cc.opSwap,
		SWAP9:          cc.opSwap,
		SWAP10:         cc.opSwap,
		SWAP11:         cc.opSwap,
		SWAP12:         cc.opSwap,
		SWAP13:         cc.opSwap,
		SWAP14:         cc.opSwap,
		SWAP15:         cc.opSwap,
		SWAP16:         cc.opSwap,
		POP:            cc.opPop,
		MLOAD:          cc.opMLoad,
		MSTORE:         cc.opMStore,
		MSTORE8:        cc.opMStore8,
		MSIZE:          cc.opMSize,
		SLOAD:          cc.opSLoad,
		SSTORE:         cc.opSSTore,
	}
	return calls
}

func (cc *CounterCollector) mLoadX() {
	cc.Deduct(S, 40)
	cc.Deduct(B, 2)
	cc.Deduct(M, 1)
	cc.offsetUtil()
	cc.SHRarith()
	cc.SHLarith()
}

func (cc *CounterCollector) offsetUtil() {
	cc.Deduct(S, 10)
	cc.Deduct(B, 1)
}

func (cc *CounterCollector) SHRarith() {
	cc.Deduct(S, 50)
	cc.Deduct(B, 2)
	cc.Deduct(A, 1)
	cc.divArith()
}

func (cc *CounterCollector) SHLarith() {
	cc.Deduct(S, 100)
	cc.Deduct(B, 4)
	cc.Deduct(A, 2)
}

func (cc *CounterCollector) divArith() {
	cc.Deduct(S, 50)
	cc.Deduct(B, 3)
	cc.Deduct(A, 1)
}

func (cc *CounterCollector) opCode(scope *ScopeContext) {
	cc.Deduct(S, 12)
	if scope.Contract.IsCreate {
		cc.mLoadX()
		cc.SHRarith()
	}
}

func (cc *CounterCollector) addBatchHashData() {
	cc.Deduct(S, 10)
}

func (cc *CounterCollector) getLenBytes(l int) {
	cc.Deduct(S, l*7+12)
	cc.Deduct(B, l*1)
	cc.multiCall(cc.SHRarith, l)
}

func (cc *CounterCollector) addHashTx() {
	cc.Deduct(S, 10)
}

func (cc *CounterCollector) addL2HashTx() {
	cc.Deduct(S, 10)
}

func (cc *CounterCollector) addBatchHashByteByByte() {
	cc.Deduct(S, 25)
	cc.Deduct(B, 1)
	cc.SHRarith()
	cc.addBatchHashData()
}

func (cc *CounterCollector) ecRecover(v, r, s *uint256.Int, isPrecompiled bool) error {
	var upperLimit *uint256.Int
	fnec, err := uint256.FromHex(fnecHex)
	if err != nil {
		return err
	}
	fnecMinusOne := fnec.Sub(fnec, uint256.NewInt(1))
	if isPrecompiled {
		upperLimit = fnecMinusOne
	} else {
		upperLimit = fnec.Div(fnec, uint256.NewInt(2))
	}

	// handle a dodgy signature
	if r.Uint64() == 0 || fnecMinusOne.Lt(r) || s.Uint64() == 0 || upperLimit.Lt(s) || (v.Uint64() != 27 && v.Uint64() != 28) {
		cc.Deduct(S, 45)
		cc.Deduct(A, 2)
		cc.Deduct(B, 8)
		return nil
	}

	fpec, err := uint256.FromHex(fpecHex)
	if err != nil {
		return err
	}

	// check if we have a sqrt to avoid counters at checkSqrtFpEc (from js)
	c := uint256.NewInt(0)
	rExp := r.Clone().Exp(r, uint256.NewInt(3))
	c.Mod(
		uint256.NewInt(0).Add(rExp, uint256.NewInt(7)),
		fpec,
	)

	r2 := fpec.Clone().Sqrt(c)
	var parity uint64 = 1
	if v.Uint64() == 27 {
		parity = 0
	}
	if r2 != nil {
		r2, err = uint256.FromHex("0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF")
		if err != nil {
			return err
		}
	} else if r2.Uint64()&1 != parity {
		r2 = fpec.Clone().Neg(r)
	}

	// in js this is converting a boolean to a number and checking for 0 on the less-than check
	if r2.Lt(fpec) {
		// do not have a root
		cc.Deduct(S, 4527)
		cc.Deduct(A, 1014)
		cc.Deduct(B, 10)
	} else {
		// has a root
		cc.Deduct(S, 6294)
		cc.Deduct(A, 528)
		cc.Deduct(B, 523)
		cc.Deduct(K, 1)
	}

	return nil
}

func (cc *CounterCollector) failAssert() {
	cc.Deduct(S, 2)
}

func (cc *CounterCollector) consolidateBlock() {
	cc.Deduct(S, 20)
	cc.Deduct(B, 2)
	cc.Deduct(P, 2*MCPL)
}

func (cc *CounterCollector) finishBatchProcessing(smtLevels int) {
	cc.Deduct(S, 200)
	cc.Deduct(K, 2)
	cc.Deduct(P, smtLevels)
	cc.Deduct(B, 1)
}

func (cc *CounterCollector) decodeChangeL2Block() {
	cc.Deduct(S, 20)
	cc.multiCall(cc.addBatchHashData, 3)
}

func (cc *CounterCollector) invFnEc() {
	cc.Deduct(S, 12)
	cc.Deduct(B, 2)
	cc.Deduct(A, 2)
}

func (cc *CounterCollector) isColdAddress() {
	cc.Deduct(S, 100)
	cc.Deduct(B, 2+1)
	cc.Deduct(P, 2*MCPL)
}

func (cc *CounterCollector) addArith() {
	cc.Deduct(S, 10)
	cc.Deduct(B, 1)
}

func (cc *CounterCollector) subArith() {
	cc.Deduct(S, 10)
	cc.Deduct(B, 1)
}

func (cc *CounterCollector) mulArith() {
	cc.Deduct(S, 50)
	cc.Deduct(B, 1)
	cc.Deduct(A, 1)
}

func (cc *CounterCollector) fillBlockInfoTreeWithTxReceipt(smtLevels int) {
	cc.Deduct(S, 20)
	cc.Deduct(P, 3*smtLevels)
}

func (cc *CounterCollector) processContractCall(smtLevels int, bytecodeLength int, isDeploy bool, isCreate bool, isCreate2 bool) {
	cc.Deduct(S, 40)
	cc.Deduct(B, 4+1)
	cc.Deduct(P, 1)
	cc.Deduct(D, 1)
	cc.Deduct(P, 2*smtLevels)
	cc.moveBalances(smtLevels)

	if isDeploy || isCreate || isCreate2 {
		cc.Deduct(S, 15)
		cc.Deduct(B, 2)
		cc.Deduct(P, 2*smtLevels)
		cc.checkBytecodeStartsEF()
		cc.hashPoseidonLinearFromMemory(bytecodeLength)
		if isCreate {
			cc.Deduct(S, 40)
			cc.Deduct(K, 1)
		} else if isCreate2 {
			cc.Deduct(S, 40)
			cc.divArith()
			cc.Deduct(K, int(math.Ceil(float64(bytecodeLength+1)/136)+1))
			cc.multiCall(cc.mLoad32, int(math.Floor(float64(bytecodeLength)/32)))
			cc.mLoadX()
			cc.SHRarith()
			cc.Deduct(K, 1)
			cc.maskAddress()
		}
	} else {
		cc.Deduct(P, int(math.Ceil(float64(bytecodeLength+1)/56)))
		cc.Deduct(D, int(math.Ceil(float64(bytecodeLength+1)/56)))
		if bytecodeLength >= 56 {
			cc.divArith()
		}
	}
}

func (cc *CounterCollector) moveBalances(smtLevels int) {
	cc.Deduct(S, 25)
	cc.Deduct(B, 3+2)
	cc.Deduct(P, 4*smtLevels)
}

func (cc *CounterCollector) checkBytecodeStartsEF() {
	cc.Deduct(S, 20)
	cc.mLoadX()
	cc.SHRarith()
}

func (cc *CounterCollector) hashPoseidonLinearFromMemory(memSize int) {
	cc.Deduct(S, 50)
	cc.Deduct(B, 1+1)
	cc.Deduct(P, int(float64(memSize+1))/56)
	cc.Deduct(D, int(float64(memSize+1))/56)
	cc.divArith()
	cc.multiCall(cc.hashPoseidonLinearFromMemoryLoop, int(math.Floor(float64(memSize)/32)))
	cc.mLoadX()
	cc.SHRarith()
}

func (cc *CounterCollector) hashPoseidonLinearFromMemoryLoop() {
	cc.Deduct(S, 8)
	cc.mLoad32()
}

func (cc *CounterCollector) mLoad32() {
	cc.Deduct(S, 40)
	cc.Deduct(B, 2)
	cc.Deduct(M, 1)
	cc.offsetUtil()
	cc.SHRarith()
	cc.SHLarith()
}

func (cc *CounterCollector) maskAddress() {
	cc.Deduct(S, 6)
	cc.Deduct(B, 1)
}

func (cc *CounterCollector) processChangeL2Block() {
	cc.Deduct(S, 70)
	cc.Deduct(B, 4+4)
	cc.Deduct(P, 6*cc.smtLevels)
	cc.Deduct(K, 2)
	cc.consolidateBlock()
	cc.setupNewBlockInfoTree()
	cc.verifyMerkleProof()
}

func (cc *CounterCollector) setupNewBlockInfoTree() {
	cc.Deduct(S, 40)
	cc.Deduct(B, 7)
	cc.Deduct(P, 6*MCPL)
}

func (cc *CounterCollector) verifyMerkleProof() {
	cc.Deduct(S, 250)
	cc.Deduct(K, 33)
}

func (cc *CounterCollector) preEcRecover(v, r, s *uint256.Int) error {
	cc.Deduct(S, 35)
	cc.Deduct(B, 1)
	cc.multiCall(cc.readFromCallDataOffset, 4)
	if err := cc.ecRecover(v, r, s, true); err != nil {
		return err
	}
	cc.mStore32()
	cc.mStoreX()

	return nil
}

func (cc *CounterCollector) preECAdd() {
	cc.Deduct(S, 50)
	cc.Deduct(B, 1)
	cc.multiCall(cc.readFromCallDataOffset, 4)
	cc.multiCall(cc.mStore32, 4)
	cc.mStoreX()
	cc.ecAdd()
}

func (cc *CounterCollector) readFromCallDataOffset() {
	cc.Deduct(S, 25)
	cc.mLoadX()
}

func (cc *CounterCollector) mStore32() {
	cc.Deduct(S, 100)
	cc.Deduct(B, 1)
	cc.Deduct(M, 1)
	cc.offsetUtil()
	cc.multiCall(cc.SHRarith, 2)
	cc.multiCall(cc.SHLarith, 2)
}

func (cc *CounterCollector) mStoreX() {
	cc.Deduct(S, 100)
	cc.Deduct(B, 1)
	cc.Deduct(M, 1)
	cc.offsetUtil()
	cc.multiCall(cc.SHRarith, 2)
	cc.multiCall(cc.SHLarith, 2)
}

func (cc *CounterCollector) decodeChangeL2BlockTx() {
	cc.Deduct(S, 20)
	cc.multiCall(cc.addBatchHashData, 3)
}

func (cc *CounterCollector) ecAdd() {
	cc.Deduct(S, 323)
	cc.Deduct(B, 33)
	cc.Deduct(A, 40)
}

func (cc *CounterCollector) preECMul() {
	cc.Deduct(S, 50)
	cc.Deduct(B, 1)
	cc.multiCall(cc.readFromCallDataOffset, 3)
	cc.multiCall(cc.mStore32, 4)
	cc.mStoreX()
	cc.ecMul()
}

func (cc *CounterCollector) ecMul() {
	cc.Deduct(S, 162890)
	cc.Deduct(B, 16395)
	cc.Deduct(A, 19161)
}

func (cc *CounterCollector) preECPairing(inputsCount int) {
	cc.Deduct(S, 50)
	cc.Deduct(B, 1)
	cc.multiCall(cc.readFromCallDataOffset, 6)
	cc.divArith()
	cc.mStore32()
	cc.mStoreX()
	cc.ecPairing(inputsCount)
}

func (cc *CounterCollector) ecPairing(inputsCount int) {
	cc.Deduct(S, 16+inputsCount*184017+171253)
	cc.Deduct(B, inputsCount*3986+650)
	cc.Deduct(A, inputsCount*13694+15411)
}

func (cc *CounterCollector) preModExp(callDataLength, returnDataLength, bLen, mLen, eLen int, base, exponent, modulus *big.Int) {
	cc.Deduct(S, 100)
	cc.Deduct(B, 20)
	cc.multiCall(cc.readFromCallDataOffset, 4)
	cc.SHRarith()
	cc.multiCall(cc.addArith, 2)
	cc.multiCall(cc.divArith, 3)
	cc.multiCall(cc.mulArith, 3)
	cc.subArith()
	cc.multiCall(cc.SHLarith, 2)
	cc.multiCall(cc.mStoreX, 2)
	cc.multiCall(cc.preModExpLoop, int(math.Floor(float64(callDataLength)/32)))
	cc.multiCall(cc.preModExpLoop, int(math.Floor(float64(returnDataLength)/32)))
	if modulus.Uint64() > 0 {
		cc.modExp(bLen, mLen, eLen, base, exponent, modulus)
	}
}

func (cc *CounterCollector) modExp(bLen, mLen, eLen int, base, exponent, modulus *big.Int) {
	steps, binary, arith := expectedModExpCounters(
		int(math.Ceil(float64(bLen)/32)),
		int(math.Ceil(float64(mLen)/32)),
		int(math.Ceil(float64(eLen)/32)),
		base,
		exponent,
		modulus,
	)
	cc.Deduct(S, int(steps.Int64()))
	cc.Deduct(B, int(binary.Int64()))
	cc.Deduct(A, int(arith.Int64()))
}

func (cc *CounterCollector) preModExpLoop() {
	cc.Deduct(S, 8)
	cc.mStore32()
}

func (cc *CounterCollector) multiCall(call func(), times int) {
	for i := 0; i < times; i++ {
		call()
	}
}

func (cc *CounterCollector) preSha256(callDataLength uint64) {
	cc.Deduct(S, 100)
	cc.Deduct(B, 1)
	cc.Deduct(SHA, int(math.Ceil(float64(callDataLength+1)/64)))
	cc.multiCall(cc.divArith, 2)
	cc.mStore32()
	cc.mStoreX()
	cc.multiCall(cc.preSha256Loop, int(math.Floor(float64(callDataLength)/32)))
	cc.readFromCallDataOffset()
	cc.SHRarith()
}

func (cc *CounterCollector) preSha256Loop() {
	cc.Deduct(S, 11)
	cc.readFromCallDataOffset()
}

func (cc *CounterCollector) preIdentity(callDataLength, returnDataLength uint64) {
	cc.Deduct(S, 45)
	cc.Deduct(B, 2)
	cc.divArith()
	// identity loop
	cc.multiCall(cc.identityLoop, int(math.Floor(float64(callDataLength)/32)))
	cc.readFromCallDataOffset()
	cc.mStoreX()
	// identity return loop
	cc.multiCall(cc.identityReturnLoop, int(math.Floor(float64(returnDataLength)/32)))
	cc.mLoadX()
	cc.mStoreX()
}

func (cc *CounterCollector) identityLoop() {
	cc.Deduct(S, 8)
	cc.readFromCallDataOffset()
	cc.mStore32()
}

func (cc *CounterCollector) identityReturnLoop() {
	cc.Deduct(S, 8)
	cc.readFromCallDataOffset()
	cc.mStore32()
}

func (cc *CounterCollector) abs() {
	cc.Deduct(S, 10)
	cc.Deduct(B, 2)
}

func (cc *CounterCollector) opAdd(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 10)
	cc.Deduct(B, 1)
	return nil, nil
}

func (cc *CounterCollector) opMul(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 10)
	cc.mulArith()
	return nil, nil
}

func (cc *CounterCollector) opSub(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 10)
	cc.Deduct(B, 1)
	return nil, nil
}

func (cc *CounterCollector) opDiv(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 15)
	cc.divArith()
	return nil, nil
}

func (cc *CounterCollector) opSDiv(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 25)
	cc.Deduct(B, 1)
	cc.multiCall(cc.abs, 2)
	cc.divArith()
	return nil, nil
}

func (cc *CounterCollector) opMod(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 20)
	cc.divArith()
	return nil, nil
}

func (cc *CounterCollector) opSMod(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 20)
	cc.Deduct(B, 1)
	cc.multiCall(cc.abs, 2)
	cc.divArith()
	return nil, nil
}

func (cc *CounterCollector) opAddMod(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 30)
	cc.Deduct(B, 3)
	cc.Deduct(A, 1)
	return nil, nil
}

func (cc *CounterCollector) opMulMod(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 10)
	cc.utilMulMod()
	return nil, nil
}

func (cc *CounterCollector) utilMulMod() {
	cc.Deduct(S, 50)
	cc.Deduct(B, 4)
	cc.Deduct(A, 2)
	cc.mulArith()
}

func (cc *CounterCollector) opExp(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 10)
	exponent := scope.Stack.Peek()
	exponentLength := len(exponent.Bytes())
	cc.getLenBytes(exponentLength)
	cc.expAd(exponentLength * 8)
	return nil, nil
}

func (cc *CounterCollector) expAd(inputLength int) {
	cc.Deduct(S, 30)
	cc.Deduct(B, 2)
	cc.getLenBits(inputLength)
	for i := 0; i < inputLength; i++ {
		cc.Deduct(S, 12)
		cc.Deduct(B, 2)
		cc.divArith()
		cc.mulArith()
		cc.mulArith()
	}
}

func (cc *CounterCollector) getLenBits(inputLength int) {
	cc.Deduct(S, 12)
	for i := 0; i < inputLength; i++ {
		cc.Deduct(S, 9)
		cc.Deduct(B, 1)
		cc.divArith()
	}
}

func (cc *CounterCollector) opSignExtend(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 20)
	cc.Deduct(B, 6)
	cc.Deduct(P, 2*cc.smtLevels)
	return nil, nil
}

func (cc *CounterCollector) opBlockHash(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 15)
	cc.Deduct(P, cc.smtLevels)
	cc.Deduct(K, 1)
	return nil, nil
}

func (cc *CounterCollector) opCoinbase(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 5)
	return nil, nil
}

func (cc *CounterCollector) opTimestamp(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 5)
	return nil, nil
}

func (cc *CounterCollector) opNumber(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 5)
	return nil, nil
}

func (cc *CounterCollector) opDifficulty(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 5)
	return nil, nil
}

func (cc *CounterCollector) opGasLimit(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 5)
	return nil, nil
}

func (cc *CounterCollector) opChainId(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 5)
	return nil, nil
}

func (cc *CounterCollector) opCalldataLoad(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 15)
	cc.readFromCallDataOffset()
	return nil, nil
}

func (cc *CounterCollector) opCalldataSize(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 10)
	return nil, nil
}

func (cc *CounterCollector) opCalldataCopy(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	inputLen := int(scope.Stack.PeekAt(3).Uint64())
	cc.opCode(scope)
	cc.Deduct(S, 100)
	cc.Deduct(B, 2)
	cc.saveMem(inputLen)
	cc.offsetUtil()
	cc.multiCall(cc.opCalldataCopyLoop, int(math.Floor(float64(inputLen)/32)))
	cc.readFromCallDataOffset()
	cc.multiCall(cc.mStoreX, 2)
	return nil, nil
}

func (cc *CounterCollector) saveMem(inputSize int) {
	if inputSize == 0 {
		cc.Deduct(S, 12)
		cc.Deduct(B, 1)
		return
	}
	cc.Deduct(S, 50)
	cc.Deduct(B, 5)
	cc.mulArith()
	cc.divArith()
}

func (cc *CounterCollector) opCalldataCopyLoop() {
	cc.Deduct(S, 30)
	cc.readFromCallDataOffset()
	cc.offsetUtil()
	cc.Deduct(M, 1)
}

func (cc *CounterCollector) opCodeSize(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 10)
	cc.Deduct(P, cc.smtLevels)
	return nil, nil
}

func (cc *CounterCollector) opExtCodeSize(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 10)
	cc.Deduct(P, cc.smtLevels)
	cc.maskAddress()
	cc.isColdAddress()
	return nil, nil
}

func (cc *CounterCollector) opExtCodeCopy(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	stack := scope.Stack
	address := stack.PeekAt(1).Bytes20()
	bytecodeLen := interpreter.evm.IntraBlockState().GetCodeSize(address)
	length := int(stack.PeekAt(4).Uint64()) // no need to read contract storage as we only care about the byte length
	cc.opCode(scope)
	cc.Deduct(S, 60)
	cc.maskAddress()
	cc.isColdAddress()
	cc.Deduct(P, 2*cc.smtLevels+int(math.Ceil(float64(bytecodeLen)/56)))
	cc.Deduct(D, int(math.Ceil(float64(bytecodeLen)/56)))
	cc.multiCall(cc.divArith, 2)
	cc.saveMem(length)
	cc.mulArith()
	cc.Deduct(M, length)
	cc.multiCall(cc.opCodeCopyLoop, length)
	cc.Deduct(B, 1)
	return nil, nil
}

func (cc *CounterCollector) opCodeCopyLoop() {
	cc.Deduct(S, 30)
	cc.Deduct(B, 2)
	cc.Deduct(M, 1)
	cc.offsetUtil()
}

func (cc *CounterCollector) opCodeCopy(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	if scope.Contract.IsCreate {
		_, err := cc.opCalldataCopy(pc, interpreter, scope)
		if err != nil {
			return nil, err
		}
	} else {
		length := int(scope.Stack.PeekAt(3).Uint64())
		cc.Deduct(S, 40)
		cc.Deduct(B, 3)
		cc.saveMem(length)
		cc.divArith()
		cc.mulArith()
		cc.multiCall(cc.opCodeCopyLoop, length)
	}
	return nil, nil
}

func (cc *CounterCollector) opReturnDataSize(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 11)
	cc.Deduct(B, 1)
	return nil, nil
}

func (cc *CounterCollector) opReturnDataCopy(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	length := int(scope.Stack.PeekAt(3).Uint64())
	cc.opCode(scope)
	cc.Deduct(S, 50)
	cc.Deduct(B, 2)
	cc.saveMem(length)
	cc.divArith()
	cc.mulArith()
	cc.multiCall(cc.returnDataCopyLoop, int(math.Floor(float64(length)/32)))
	cc.mLoadX()
	cc.mStoreX()
	return nil, nil
}

func (cc *CounterCollector) returnDataCopyLoop() {
	cc.Deduct(S, 10)
	cc.mLoad32()
	cc.mStore32()
}

func (cc *CounterCollector) opExtCodeHash(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 10)
	cc.Deduct(P, cc.smtLevels)
	cc.maskAddress()
	cc.isColdAddress()
	return nil, nil
}

func (cc *CounterCollector) opLT(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 8)
	cc.Deduct(B, 1)
	return nil, nil
}

func (cc *CounterCollector) opGT(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 8)
	cc.Deduct(B, 1)
	return nil, nil
}

func (cc *CounterCollector) opSLT(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 8)
	cc.Deduct(B, 1)
	return nil, nil
}

func (cc *CounterCollector) opSGT(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 8)
	cc.Deduct(B, 1)
	return nil, nil
}

func (cc *CounterCollector) opEQ(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 8)
	cc.Deduct(B, 1)
	return nil, nil
}

func (cc *CounterCollector) opIsZero(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 8)
	cc.Deduct(B, 1)
	return nil, nil
}

func (cc *CounterCollector) opAnd(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 10)
	cc.Deduct(B, 1)
	cc.Deduct(P, cc.smtLevels)
	return nil, nil
}

func (cc *CounterCollector) opOr(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 10)
	cc.Deduct(B, 1)
	cc.Deduct(P, cc.smtLevels)
	return nil, nil
}
func (cc *CounterCollector) opXor(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 10)
	cc.Deduct(B, 1)
	cc.Deduct(P, cc.smtLevels)
	return nil, nil
}
func (cc *CounterCollector) opNot(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 10)
	cc.Deduct(B, 1)
	cc.Deduct(P, cc.smtLevels)
	return nil, nil
}
func (cc *CounterCollector) opByte(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 15)
	cc.Deduct(B, 2)
	cc.SHRarith()
	return nil, nil
}
func (cc *CounterCollector) opSHR(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 8)
	cc.Deduct(B, 1)
	cc.shrArithBit()
	return nil, nil
}

func (cc *CounterCollector) shrArithBit() {
	cc.Deduct(S, 30)
	cc.Deduct(B, 2)
	cc.divArith()
}

func (cc *CounterCollector) opSHL(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 8)
	cc.Deduct(B, 1)
	cc.shlArithBit()
	return nil, nil
}

func (cc *CounterCollector) shlArithBit() {
	cc.Deduct(S, 50)
	cc.Deduct(B, 2)
	cc.Deduct(A, 2)
}

func (cc *CounterCollector) opSAR(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 25)
	cc.Deduct(B, 5)
	cc.shrArithBit()
	return nil, nil
}

func (cc *CounterCollector) opStop(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 20)
	return nil, nil
}

func (cc *CounterCollector) opCreate(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	size := int(scope.Stack.PeekAt(3).Uint64())
	nonce := interpreter.evm.IntraBlockState().GetNonce(scope.Contract.CallerAddress) + 1
	nonceByteLength := getRelevantNumberBytes(hermez_db.Uint64ToBytes(nonce))
	cc.opCode(scope)
	cc.Deduct(S, 70)
	cc.Deduct(B, 3)
	cc.Deduct(P, 3*cc.smtLevels)
	cc.saveMem(size)
	cc.getLenBytes(nonceByteLength)
	cc.computeGasSendCall()
	cc.saveCalldataPointer()
	cc.checkpointBlockInfoTree()
	cc.checkpointTouched()

	cc.processContractCall(cc.smtLevels, size, false, true, false)

	return nil, nil
}

func (cc *CounterCollector) computeGasSendCall() {
	cc.Deduct(S, 25)
	cc.Deduct(B, 2)
}

func (cc *CounterCollector) saveCalldataPointer() {
	cc.Deduct(S, 6)
}

func (cc *CounterCollector) checkpointBlockInfoTree() {
	cc.Deduct(S, 4)
}

func (cc *CounterCollector) checkpointTouched() {
	cc.Deduct(S, 2)
}

func (cc *CounterCollector) opCall(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	inSize := int(scope.Stack.PeekAt(5).Uint64())
	outSize := int(scope.Stack.PeekAt(7).Uint64())
	cc.opCode(scope)
	cc.Deduct(S, 80)
	cc.Deduct(B, 5)
	cc.maskAddress()
	cc.saveMem(inSize)
	cc.saveMem(outSize)
	cc.isColdAddress()
	cc.isEmptyAccount()
	cc.computeGasSendCall()
	cc.saveCalldataPointer()
	cc.checkpointBlockInfoTree()
	cc.checkpointTouched()

	cc.processContractCall(cc.smtLevels, inSize, false, false, false)

	return nil, nil
}

func (cc *CounterCollector) isEmptyAccount() {
	cc.Deduct(S, 30)
	cc.Deduct(B, 3)
	cc.Deduct(P, 3*cc.smtLevels)
}

func (cc *CounterCollector) opCallCode(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	inSize := int(scope.Stack.PeekAt(5).Uint64())
	outSize := int(scope.Stack.PeekAt(7).Uint64())
	cc.opCode(scope)
	cc.Deduct(S, 80)
	cc.Deduct(B, 5)
	cc.maskAddress()
	cc.saveMem(inSize)
	cc.saveMem(outSize)
	cc.isColdAddress()
	cc.computeGasSendCall()
	cc.saveCalldataPointer()
	cc.checkpointBlockInfoTree()
	cc.checkpointTouched()

	cc.processContractCall(cc.smtLevels, inSize, false, false, false)

	return nil, nil
}

func (cc *CounterCollector) opDelegateCall(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	inSize := int(scope.Stack.PeekAt(4).Uint64())
	outSize := int(scope.Stack.PeekAt(6).Uint64())
	cc.opCode(scope)
	cc.Deduct(S, 80)
	cc.maskAddress()
	cc.saveMem(inSize)
	cc.saveMem(outSize)
	cc.isColdAddress()
	cc.computeGasSendCall()
	cc.saveCalldataPointer()
	cc.checkpointBlockInfoTree()
	cc.checkpointTouched()

	cc.processContractCall(cc.smtLevels, inSize, false, false, false)

	return nil, nil
}

func (cc *CounterCollector) opStaticCall(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	inSize := int(scope.Stack.PeekAt(4).Uint64())
	outSize := int(scope.Stack.PeekAt(6).Uint64())
	cc.opCode(scope)
	cc.Deduct(S, 80)
	cc.maskAddress()
	cc.saveMem(inSize)
	cc.saveMem(outSize)
	cc.isColdAddress()
	cc.computeGasSendCall()
	cc.saveCalldataPointer()
	cc.checkpointBlockInfoTree()
	cc.checkpointTouched()

	cc.processContractCall(cc.smtLevels, inSize, false, false, false)

	return nil, nil
}

func (cc *CounterCollector) opCreate2(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	inSize := int(scope.Stack.PeekAt(3).Uint64())
	accNonce := interpreter.evm.IntraBlockState().GetNonce(scope.Contract.CallerAddress) + 1
	nonceByteLength := getRelevantNumberBytes(hermez_db.Uint64ToBytes(accNonce))
	cc.opCode(scope)
	cc.Deduct(S, 80)
	cc.Deduct(B, 4)
	cc.Deduct(P, 2*cc.smtLevels)
	cc.saveMem(inSize)
	cc.divArith()
	cc.getLenBytes(nonceByteLength)
	cc.computeGasSendCall()
	cc.saveCalldataPointer()
	cc.checkpointBlockInfoTree()
	cc.checkpointTouched()

	cc.processContractCall(cc.smtLevels, inSize, false, false, true)

	return nil, nil
}

func (cc *CounterCollector) opReturn(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	returnSize := int(scope.Stack.PeekAt(2).Uint64())
	cc.opCode(scope)
	cc.Deduct(S, 30)
	cc.Deduct(B, 1)
	cc.saveMem(returnSize)
	if cc.isDeploy || scope.Contract.IsCreate {
		if scope.Contract.IsCreate {
			cc.Deduct(S, 25)
			cc.Deduct(B, 2)
			cc.Deduct(P, 2*cc.smtLevels)
			cc.checkBytecodeStartsEF()
			cc.hashPoseidonLinearFromMemory(returnSize)
		}
	} else {
		cc.multiCall(cc.returnLoop, int(math.Floor(float64(returnSize)/32)))
		cc.mLoadX()
		cc.mStoreX()
	}
	return nil, nil
}

func (cc *CounterCollector) returnLoop() {
	cc.Deduct(S, 12)
	cc.mLoad32()
	cc.mStore32()
}

func (cc *CounterCollector) opRevert(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	size := int(scope.Stack.PeekAt(2).Uint64())
	cc.opCode(scope)
	cc.Deduct(S, 40)
	cc.Deduct(B, 1)
	cc.revertTouched()
	cc.revertBlockInfoTree()
	cc.saveMem(size)
	cc.multiCall(cc.revertLoop, int(math.Floor(float64(size)/32)))
	cc.mLoadX()
	cc.mStoreX()
	return nil, nil
}

func (cc *CounterCollector) revertTouched() {
	cc.Deduct(S, 2)
}

func (cc *CounterCollector) revertBlockInfoTree() {
	cc.Deduct(S, 4)
}

func (cc *CounterCollector) revertLoop() {
	cc.Deduct(S, 12)
	cc.mLoad32()
	cc.mStore32()
}

func (cc *CounterCollector) opSendAll(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 60)
	cc.Deduct(B, 2+1)
	cc.Deduct(P, 4*cc.smtLevels)
	cc.maskAddress()
	cc.isEmptyAccount()
	cc.isColdAddress()
	cc.addArith()
	return nil, nil
}

func (cc *CounterCollector) opInvalid(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 50)
	return nil, nil
}

func (cc *CounterCollector) opAddress(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 6)
	return nil, nil
}

func (cc *CounterCollector) opSelfBalance(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 8)
	cc.Deduct(P, cc.smtLevels)
	return nil, nil
}

func (cc *CounterCollector) opBalance(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 8)
	cc.Deduct(P, cc.smtLevels)
	cc.maskAddress()
	cc.isColdAddress()
	return nil, nil
}

func (cc *CounterCollector) opOrigin(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 5)
	return nil, nil
}

func (cc *CounterCollector) opCaller(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 5)
	return nil, nil
}

func (cc *CounterCollector) opCallValue(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 5)
	return nil, nil
}

func (cc *CounterCollector) opGasPrice(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 5)
	return nil, nil
}

func (cc *CounterCollector) opGas(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 4)
	return nil, nil
}

func (cc *CounterCollector) opSha3(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	size := int(scope.Stack.PeekAt(2).Uint64())
	cc.opCode(scope)
	cc.Deduct(S, 40)
	cc.Deduct(K, int(math.Ceil(float64(size)+1)/32))
	cc.saveMem(size)
	cc.multiCall(cc.divArith, 2)
	cc.mulArith()
	cc.multiCall(cc.sha3Loop, int(math.Floor(float64(size)/32)))
	cc.mLoadX()
	cc.SHRarith()
	return nil, nil
}

func (cc *CounterCollector) sha3Loop() {
	cc.Deduct(S, 8)
	cc.mLoad32()
}

func (cc *CounterCollector) opJump(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 5)
	cc.checkJumpDest(scope)
	return nil, nil
}

func (cc *CounterCollector) checkJumpDest(scope *ScopeContext) {
	cc.Deduct(S, 10)
	if scope.Contract.IsCreate {
		cc.Deduct(B, 1)
		if cc.isDeploy {
			cc.mLoadX()
		}
	}
}

func (cc *CounterCollector) opJumpI(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 10)
	cc.Deduct(B, 1)
	cc.checkJumpDest(scope)
	return nil, nil
}

func (cc *CounterCollector) opPC(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 4)
	return nil, nil
}

func (cc *CounterCollector) opJumpDest(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 2)
	return nil, nil
}

func (cc *CounterCollector) log(scope *ScopeContext) {
	size := int(scope.Stack.PeekAt(2).Uint64())
	cc.opCode(scope)
	cc.Deduct(S, 30+8*4)
	cc.saveMem(size)
	cc.mulArith()
	cc.divArith()
	cc.Deduct(P, int(math.Ceil(float64(size)/56)+4))
	cc.Deduct(D, int(math.Ceil(float64(size)/56)+4))
	cc.multiCall(cc.logLoop, int(math.Floor(float64(size)+1/32)))
	cc.mLoadX()
	cc.SHRarith()
	cc.fillBlockInfoTreeWithLog()
	cc.Deduct(B, 1)
}

func (cc *CounterCollector) logLoop() {
	cc.Deduct(S, 10)
	cc.mLoad32()
}

func (cc *CounterCollector) fillBlockInfoTreeWithLog() {
	cc.Deduct(S, 11)
	cc.Deduct(P, MCPL)
	cc.Deduct(B, 1)
}

func (cc *CounterCollector) opLog0(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.log(scope)
	return nil, nil
}

func (cc *CounterCollector) opLog1(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.log(scope)
	return nil, nil
}

func (cc *CounterCollector) opLog2(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.log(scope)
	return nil, nil
}

func (cc *CounterCollector) opLog3(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.log(scope)
	return nil, nil
}

func (cc *CounterCollector) opLog4(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.log(scope)
	return nil, nil
}

func (cc *CounterCollector) opPush0(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 4)
	return nil, nil
}

func (cc *CounterCollector) opPushGenerator(num int) executionFunc {
	return func(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
		cc.opPush(num, scope)
		return nil, nil
	}
}

func (cc *CounterCollector) opPush(num int, scope *ScopeContext) {
	cc.opCode(scope)
	cc.Deduct(S, 4)
	if scope.Contract.IsCreate || cc.isDeploy {
		cc.Deduct(B, 1)
		if scope.Contract.IsCreate {
			cc.Deduct(S, 20)
			cc.mLoadX()
			cc.SHRarith()
		} else {
			cc.Deduct(S, 10)
			for i := 0; i < num; i++ {
				cc.Deduct(S, 10)
				cc.SHLarith()
			}
		}
	} else {
		cc.Deduct(S, 10)
		cc.readPush(num)
	}
}

func (cc *CounterCollector) readPush(num int) {
	cc.Deduct(S, 15)
	cc.Deduct(B, 1)
	numBlocks := int(math.Ceil(float64(num) / 4))
	leftBytes := num % 4

	for i := 0; i <= numBlocks; i++ {
		cc.Deduct(S, 20)
		cc.Deduct(B, 1)
		for j := i - 1; j > 0; j-- {
			cc.Deduct(S, 8)
		}
	}

	for i := 0; i < leftBytes; i++ {
		cc.Deduct(S, 40)
		cc.Deduct(B, 4)
	}
}

func (cc *CounterCollector) opDup(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 6)
	return nil, nil
}

func (cc *CounterCollector) opSwap(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 7)
	return nil, nil
}

func (cc *CounterCollector) opPop(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 3)
	return nil, nil
}

func (cc *CounterCollector) opMLoad(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 8)
	cc.saveMem(32)
	cc.mLoad32()
	return nil, nil
}

func (cc *CounterCollector) opMStore(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 22)
	cc.Deduct(M, 1)
	cc.saveMem(32)
	cc.offsetUtil()
	return nil, nil
}

func (cc *CounterCollector) opMStore8(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 13)
	cc.Deduct(M, 1)
	cc.saveMem(1)
	cc.offsetUtil()
	return nil, nil
}

func (cc *CounterCollector) opMSize(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 15)
	cc.divArith()
	return nil, nil
}

func (cc *CounterCollector) opSLoad(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 10)
	cc.Deduct(P, cc.smtLevels)
	cc.isColdSlot()
	return nil, nil
}

func (cc *CounterCollector) isColdSlot() {
	cc.Deduct(S, 20)
	cc.Deduct(B, 1)
	cc.Deduct(P, 2*MCPL)
}

func (cc *CounterCollector) opSSTore(pc *uint64, interpreter *EVMInterpreter, scope *ScopeContext) ([]byte, error) {
	cc.opCode(scope)
	cc.Deduct(S, 70)
	cc.Deduct(B, 8)
	cc.Deduct(P, 3*cc.smtLevels)
	cc.isColdSlot()
	return nil, nil
}

func getRelevantNumberBytes(input []byte) int {
	totalLength := len(input)
	rel := 0
	for i := 0; i < totalLength; i++ {
		if input[i] == 0 {
			continue
		}
		rel = i
		break
	}
	return totalLength - rel
}
