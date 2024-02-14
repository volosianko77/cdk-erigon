package tx

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"

	"bytes"

	"regexp"
	"strings"

	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/rlp"
	"github.com/ledgerwatch/erigon/smt/pkg/utils"
	"github.com/ledgerwatch/erigon/zkevm/hex"
	"github.com/ledgerwatch/log/v3"
)

const (
	forkID4      = 4
	forkID5      = 5
	double       = 2
	ether155V    = 27
	etherPre155V = 35
	// MaxEffectivePercentage is the maximum value that can be used as effective percentage
	MaxEffectivePercentage = uint8(255)
	// Decoding constants
	headerByteLength               uint64 = 1
	sLength                        uint64 = 32
	rLength                        uint64 = 32
	vLength                        uint64 = 1
	c0                             uint64 = 192 // 192 is c0. This value is defined by the rlp protocol
	ff                             uint64 = 255 // max value of rlp header
	shortRlp                       uint64 = 55  // length of the short rlp codification
	f7                             uint64 = 247 // 192 + 55 = c0 + shortRlp
	efficiencyPercentageByteLength uint64 = 1

	changeL2BlockTransactionId = 11
)

var (
	ErrInvalidData = errors.New("invalid data")
)

func DecodeTxs(txsData []byte, forkID uint64) ([]types.Transaction, []byte, []uint8, error) {
	var pos uint64
	var txs []types.Transaction
	var efficiencyPercentages []uint8
	txDataLength := uint64(len(txsData))
	if txDataLength == 0 {
		return txs, txsData, nil, nil
	}
	for pos < txDataLength {
		num, err := strconv.ParseUint(hex.EncodeToString(txsData[pos:pos+1]), hex.Base, hex.BitSize64)
		if err != nil {
			log.Debug("error parsing header length: ", err)
			return []types.Transaction{}, txsData, []uint8{}, err
		}

		// if num is 11 then we are trying to parse a `changeL2Block` transaction so skip it
		if num == changeL2BlockTransactionId {
			pos += 9
			continue
		}

		// First byte is the length and must be ignored
		if num < c0 {
			log.Debug("error num < c0 : %d, %d", num, c0)
			return []types.Transaction{}, txsData, []uint8{}, ErrInvalidData
		}
		length := num - c0
		if length > shortRlp { // If rlp is bigger than length 55
			// n is the length of the rlp data without the header (1 byte) for example "0xf7"
			if (pos + 1 + num - f7) > txDataLength {
				log.Debug("error parsing length: ", err)
				return []types.Transaction{}, txsData, []uint8{}, err
			}
			n, err := strconv.ParseUint(hex.EncodeToString(txsData[pos+1:pos+1+num-f7]), hex.Base, hex.BitSize64) // +1 is the header. For example 0xf7
			if err != nil {
				log.Debug("error parsing length: ", err)
				return []types.Transaction{}, txsData, []uint8{}, err
			}
			if n+num < f7 {
				log.Debug("error n + num < f7: ", err)
				return []types.Transaction{}, txsData, []uint8{}, ErrInvalidData
			}
			length = n + num - f7 // num - f7 is the header. For example 0xf7
		}

		endPos := pos + length + rLength + sLength + vLength + headerByteLength

		if forkID >= forkID5 {
			endPos += efficiencyPercentageByteLength
		}

		if endPos > txDataLength {
			err := fmt.Errorf("endPos %d is bigger than txDataLength %d", endPos, txDataLength)
			log.Debug("error parsing header: ", err)
			return []types.Transaction{}, txsData, []uint8{}, ErrInvalidData
		}

		if endPos < pos {
			err := fmt.Errorf("endPos %d is smaller than pos %d", endPos, pos)
			log.Debug("error parsing header: ", err)
			return []types.Transaction{}, txsData, []uint8{}, ErrInvalidData
		}

		if endPos < pos {
			err := fmt.Errorf("endPos %d is smaller than pos %d", endPos, pos)
			log.Debug("error parsing header: ", err)
			return []types.Transaction{}, txsData, []uint8{}, ErrInvalidData
		}

		fullDataTx := txsData[pos:endPos]
		dataStart := pos + length + headerByteLength
		txInfo := txsData[pos:dataStart]
		rData := txsData[dataStart : dataStart+rLength]
		sData := txsData[dataStart+rLength : dataStart+rLength+sLength]
		vData := txsData[dataStart+rLength+sLength : dataStart+rLength+sLength+vLength]

		if forkID >= forkID5 {
			efficiencyPercentage := txsData[dataStart+rLength+sLength+vLength : endPos]
			efficiencyPercentages = append(efficiencyPercentages, efficiencyPercentage[0])
		}

		pos = endPos

		// Decode rlpFields
		var rlpFields [][]byte
		err = rlp.DecodeBytes(txInfo, &rlpFields)
		if err != nil {
			log.Error("error decoding tx Bytes: ", err, ". fullDataTx: ", hex.EncodeToString(fullDataTx), "\n tx: ", hex.EncodeToString(txInfo), "\n Txs received: ", hex.EncodeToString(txsData))
			return []types.Transaction{}, txsData, []uint8{}, ErrInvalidData
		}

		legacyTx, err := rlpFieldsToLegacyTx(rlpFields, vData, rData, sData)
		if err != nil {
			log.Debug("error creating tx from rlp fields: ", err, ". fullDataTx: ", hex.EncodeToString(fullDataTx), "\n tx: ", hex.EncodeToString(txInfo), "\n Txs received: ", hex.EncodeToString(txsData))
			return []types.Transaction{}, txsData, []uint8{}, err
		}

		txs = append(txs, legacyTx)
	}
	return txs, txsData, efficiencyPercentages, nil
}

func DecodeTx(encodedTx []byte, efficiencyPercentage byte, forkId uint16) (types.Transaction, uint8, error) {
	// efficiencyPercentage := uint8(0)
	if forkId >= forkID5 {
		encodedTx = append(encodedTx, efficiencyPercentage)
	}

	tx, err := types.DecodeTransaction(rlp.NewStream(bytes.NewReader(encodedTx), 0))
	if err != nil {
		return nil, 0, err
	}
	return tx, efficiencyPercentage, nil
}

// RlpFieldsToLegacyTx parses the rlp fields slice into a type.LegacyTx
// in this specific order:
//
// required fields:
// [0] Nonce    uint64
// [1] GasPrice *big.Int
// [2] Gas      uint64
// [3] To       *common.Address
// [4] Value    *big.Int
// [5] Data     []byte
//
// optional fields:
// [6] V        *big.Int
// [7] R        *big.Int
// [8] S        *big.Int
func rlpFieldsToLegacyTx(fields [][]byte, v, r, s []byte) (tx *types.LegacyTx, err error) {
	const (
		fieldsSizeWithoutChainID = 6
		fieldsSizeWithChainID    = 7
	)

	if len(fields) < fieldsSizeWithoutChainID {
		return nil, types.ErrTxTypeNotSupported
	}

	nonce := big.NewInt(0).SetBytes(fields[0]).Uint64()

	gasPriceI := &uint256.Int{}
	gasPriceI.SetBytes(fields[1])

	gas := big.NewInt(0).SetBytes(fields[2]).Uint64()
	var to *common.Address

	if fields[3] != nil && len(fields[3]) != 0 {
		tmp := common.BytesToAddress(fields[3])
		to = &tmp
	}
	value := big.NewInt(0).SetBytes(fields[4])
	data := fields[5]

	txV := big.NewInt(0).SetBytes(v)
	if len(fields) >= fieldsSizeWithChainID {
		chainID := big.NewInt(0).SetBytes(fields[6])

		// a = chainId * 2
		// b = v - 27
		// c = a + 35
		// v = b + c
		//
		// same as:
		// v = v-27+chainId*2+35
		a := new(big.Int).Mul(chainID, big.NewInt(double))
		b := new(big.Int).Sub(new(big.Int).SetBytes(v), big.NewInt(ether155V))
		c := new(big.Int).Add(a, big.NewInt(etherPre155V))
		txV = new(big.Int).Add(b, c)
	}

	txVi := uint256.Int{}
	txVi.SetBytes(txV.Bytes())

	txRi := uint256.Int{}
	txRi.SetBytes(r)

	txSi := uint256.Int{}
	txSi.SetBytes(s)

	valueI := &uint256.Int{}
	valueI.SetBytes(value.Bytes())

	return &types.LegacyTx{
		CommonTx: types.CommonTx{
			Nonce: nonce,
			Gas:   gas,
			To:    to,
			Value: valueI,
			Data:  data,
			V:     txVi,
			R:     txRi,
			S:     txSi,
		},
		GasPrice: gasPriceI,
	}, nil
}

func ComputeL2TxHash(
	chainId *big.Int,
	value, gasPrice *uint256.Int,
	nonce, txGasLimit uint64,
	to, from *common.Address,
	data []byte,
) (common.Hash, error) {

	txType := "01"
	if chainId != nil && chainId.Cmp(big.NewInt(0)) == 0 {
		txType = "00"
	}

	// add txType, nonce, gasPrice and gasLimit
	noncePart, err := formatL2TxHashParam(nonce, 8)
	if err != nil {
		return common.Hash{}, err

	}
	gasPricePart, err := formatL2TxHashParam(gasPrice, 32)
	if err != nil {
		return common.Hash{}, err
	}
	gasLimitPart, err := formatL2TxHashParam(txGasLimit, 8)
	if err != nil {
		return common.Hash{}, err
	}
	hash := fmt.Sprintf("%s%s%s%s", txType, noncePart, gasPricePart, gasLimitPart)

	// check is deploy
	if to == nil || to.Hex() == "0x0000000000000000000000000000000000000000" {
		hash += "01"
	} else {
		toPart, err := formatL2TxHashParam(to.Hex(), 20)
		if err != nil {
			return common.Hash{}, err
		}
		hash += fmt.Sprintf("00%s", toPart)
	}
	// add value
	valuePart, err := formatL2TxHashParam(value, 32)
	if err != nil {
		return common.Hash{}, err
	}
	hash += valuePart

	// compute data length
	dataStr := hex.EncodeToHex(data)
	if len(dataStr) > 1 && dataStr[:2] == "0x" {
		dataStr = dataStr[2:]
	}

	//round to ceil
	dataLength := (len(dataStr) + 1) / 2
	dataLengthPart, err := formatL2TxHashParam(dataLength, 3)
	if err != nil {
		return common.Hash{}, err
	}
	hash += dataLengthPart

	if dataLength > 0 {
		dataPart, err := formatL2TxHashParam(dataStr, dataLength)
		if err != nil {
			return common.Hash{}, err
		}
		hash += dataPart
	}

	// add chainID
	if chainId != nil {
		chainIDPart, err := formatL2TxHashParam(chainId, 8)
		if err != nil {
			return common.Hash{}, err
		}
		hash += chainIDPart
	}

	// add from
	fromPart, err := formatL2TxHashParam(from.Hex(), 20)
	if err != nil {
		return common.Hash{}, err
	}
	hash += fromPart

	hashed, err := utils.HashContractBytecode(hash)
	if err != nil {
		return common.Hash{}, err
	}

	return common.HexToHash(hashed), nil
}

func formatL2TxHashParam(param interface{}, paramLength int) (string, error) {
	var paramStr string

	switch v := param.(type) {
	case int:
		if v == 0 {
			paramStr = "0"
		} else {
			paramStr = strconv.FormatInt(int64(v), 16)
		}
	case int64:
		if v == 0 {
			paramStr = "0"
		} else {
			paramStr = strconv.FormatInt(v, 16)
		}
	case uint:
		if v == 0 {
			paramStr = "0"
		} else {
			paramStr = strconv.FormatUint(uint64(v), 16)
		}
	case uint64:
		if v == 0 {
			paramStr = "0"
		} else {
			paramStr = strconv.FormatUint(v, 16)
		}
	case *big.Int:
		if v.Sign() == 0 {
			paramStr = "0"
		} else {
			paramStr = v.Text(16)
		}
	case *uint256.Int:
		if v == nil {
			paramStr = "0"
		} else {
			paramStr = v.Hex()
		}
	case []uint8:
		paramStr = hex.EncodeToHex(v)
	case common.Address:
		paramStr = v.Hex()
	case string:
		paramStr = v
	default:
		return "", fmt.Errorf("unsupported parameter type")
	}

	if strings.HasPrefix(paramStr, "0x") {
		paramStr = paramStr[2:]
	}

	if len(paramStr)%2 == 1 {
		paramStr = "0" + paramStr
	}

	matched, err := regexp.MatchString("^[0-9a-fA-F]+$", paramStr)
	if err != nil {
		return "", err
	}
	if !matched {
		return "", fmt.Errorf("invalid hex string")
	}

	paramStr = padStart(paramStr, paramLength*2, "0")

	return paramStr, nil
}

func padStart(str string, length int, pad string) string {
	if len(str) >= length {
		return str
	}
	padding := strings.Repeat(pad, length-len(str))
	return padding[:length-len(str)] + str
}
