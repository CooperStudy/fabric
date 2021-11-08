/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package pvtdatastorage

import (
	"bytes"
	"fmt"
	"math"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/core/ledger/kvledger/txmgmt/version"
	"github.com/hyperledger/fabric/core/ledger/util"
	"github.com/hyperledger/fabric/protos/ledger/rwset"
	"github.com/pkg/errors"
	"github.com/willf/bitset"
)

var (
	pendingCommitKey               = []byte{0}
	lastCommittedBlkkey            = []byte{1}
	pvtDataKeyPrefix               = []byte{2}
	expiryKeyPrefix                = []byte{3}
	eligibleMissingDataKeyPrefix   = []byte{4}
	ineligibleMissingDataKeyPrefix = []byte{5}
	collElgKeyPrefix               = []byte{6}
	lastUpdatedOldBlocksKey        = []byte{7}

	nilByte    = byte(0)
	emptyValue = []byte{}
)

func getDataKeysForRangeScanByBlockNum(blockNum uint64) (startKey, endKey []byte) {
	fmt.Println("===getDataKeysForRangeScanByBlockNum==")
	startKey = append(pvtDataKeyPrefix, version.NewHeight(blockNum, 0).ToBytes()...)
	endKey = append(pvtDataKeyPrefix, version.NewHeight(blockNum+1, 0).ToBytes()...)
	return
}

func getExpiryKeysForRangeScan(minBlkNum, maxBlkNum uint64) (startKey, endKey []byte) {
	fmt.Println("===getExpiryKeysForRangeScan==")
	startKey = append(expiryKeyPrefix, version.NewHeight(minBlkNum, 0).ToBytes()...)
	endKey = append(expiryKeyPrefix, version.NewHeight(maxBlkNum+1, 0).ToBytes()...)
	return
}

func encodeLastCommittedBlockVal(blockNum uint64) []byte {
	fmt.Println("===encodeLastCommittedBlockVal==")
	return proto.EncodeVarint(blockNum)
}

func decodeLastCommittedBlockVal(blockNumBytes []byte) uint64 {
	fmt.Println("===decodeLastCommittedBlockVal==")
	s, _ := proto.DecodeVarint(blockNumBytes)
	return s
}

func encodeDataKey(key *dataKey) []byte {
	fmt.Println("===encodeDataKey==")
	dataKeyBytes := append(pvtDataKeyPrefix, version.NewHeight(key.blkNum, key.txNum).ToBytes()...)
	dataKeyBytes = append(dataKeyBytes, []byte(key.ns)...)
	dataKeyBytes = append(dataKeyBytes, nilByte)
	return append(dataKeyBytes, []byte(key.coll)...)
}

func encodeDataValue(collData *rwset.CollectionPvtReadWriteSet) ([]byte, error) {
	fmt.Println("===encodeDataValue==")
	return proto.Marshal(collData)
}

func encodeExpiryKey(expiryKey *expiryKey) []byte {
	fmt.Println("===encodeExpiryKey==")
	// reusing version encoding scheme here
	return append(expiryKeyPrefix, version.NewHeight(expiryKey.expiringBlk, expiryKey.committingBlk).ToBytes()...)
}

func encodeExpiryValue(expiryData *ExpiryData) ([]byte, error) {
	fmt.Println("===encodeExpiryValue==")
	return proto.Marshal(expiryData)
}

func decodeExpiryKey(expiryKeyBytes []byte) *expiryKey {
	fmt.Println("===decodeExpiryKey==")
	height, _ := version.NewHeightFromBytes(expiryKeyBytes[1:])
	return &expiryKey{expiringBlk: height.BlockNum, committingBlk: height.TxNum}
}

func decodeExpiryValue(expiryValueBytes []byte) (*ExpiryData, error) {
	fmt.Println("===decodeExpiryValue==")
	expiryData := &ExpiryData{}
	err := proto.Unmarshal(expiryValueBytes, expiryData)
	return expiryData, err
}

func decodeDatakey(datakeyBytes []byte) *dataKey {
	fmt.Println("===decodeDatakey==")
	v, n := version.NewHeightFromBytes(datakeyBytes[1:])
	blkNum := v.BlockNum
	tranNum := v.TxNum
	remainingBytes := datakeyBytes[n+1:]
	nilByteIndex := bytes.IndexByte(remainingBytes, nilByte)
	ns := string(remainingBytes[:nilByteIndex])
	coll := string(remainingBytes[nilByteIndex+1:])
	return &dataKey{nsCollBlk{ns, coll, blkNum}, tranNum}
}

func decodeDataValue(datavalueBytes []byte) (*rwset.CollectionPvtReadWriteSet, error) {
	fmt.Println("===decodeDataValue==")
	collPvtdata := &rwset.CollectionPvtReadWriteSet{}
	err := proto.Unmarshal(datavalueBytes, collPvtdata)
	return collPvtdata, err
}

func encodeMissingDataKey(key *missingDataKey) []byte {
	fmt.Println("===encodeMissingDataKey==")
	if key.isEligible {
		keyBytes := append(eligibleMissingDataKeyPrefix, util.EncodeReverseOrderVarUint64(key.blkNum)...)
		keyBytes = append(keyBytes, []byte(key.ns)...)
		keyBytes = append(keyBytes, nilByte)
		return append(keyBytes, []byte(key.coll)...)
	}

	keyBytes := append(ineligibleMissingDataKeyPrefix, []byte(key.ns)...)
	keyBytes = append(keyBytes, nilByte)
	keyBytes = append(keyBytes, []byte(key.coll)...)
	keyBytes = append(keyBytes, nilByte)
	return append(keyBytes, []byte(util.EncodeReverseOrderVarUint64(key.blkNum))...)
}

func decodeMissingDataKey(keyBytes []byte) *missingDataKey {
	fmt.Println("===decodeMissingDataKey==")
	key := &missingDataKey{nsCollBlk: nsCollBlk{}}
	if keyBytes[0] == eligibleMissingDataKeyPrefix[0] {
		blkNum, numBytesConsumed := util.DecodeReverseOrderVarUint64(keyBytes[1:])

		splittedKey := bytes.Split(keyBytes[numBytesConsumed+1:], []byte{nilByte})
		key.ns = string(splittedKey[0])
		key.coll = string(splittedKey[1])
		key.blkNum = blkNum
		key.isEligible = true
		return key
	}

	splittedKey := bytes.SplitN(keyBytes[1:], []byte{nilByte}, 3) //encoded bytes for blknum may contain empty bytes
	key.ns = string(splittedKey[0])
	key.coll = string(splittedKey[1])
	key.blkNum, _ = util.DecodeReverseOrderVarUint64(splittedKey[2])
	key.isEligible = false
	return key
}

func encodeMissingDataValue(bitmap *bitset.BitSet) ([]byte, error) {
	fmt.Println("===encodeMissingDataValue==")
	return bitmap.MarshalBinary()
}

func decodeMissingDataValue(bitmapBytes []byte) (*bitset.BitSet, error) {
	fmt.Println("===decodeMissingDataValue==")
	bitmap := &bitset.BitSet{}
	if err := bitmap.UnmarshalBinary(bitmapBytes); err != nil {
		return nil, err
	}
	return bitmap, nil
}

func encodeCollElgKey(blkNum uint64) []byte {
	fmt.Println("===encodeCollElgKey==")
	return append(collElgKeyPrefix, util.EncodeReverseOrderVarUint64(blkNum)...)
}

func decodeCollElgKey(b []byte) uint64 {
	fmt.Println("===decodeCollElgKey==")
	blkNum, _ := util.DecodeReverseOrderVarUint64(b[1:])
	return blkNum
}

func encodeCollElgVal(m *CollElgInfo) ([]byte, error) {
	fmt.Println("===encodeCollElgVal==")
	return proto.Marshal(m)
}

func decodeCollElgVal(b []byte) (*CollElgInfo, error) {
	fmt.Println("===decodeCollElgVal==")
	m := &CollElgInfo{}
	if err := proto.Unmarshal(b, m); err != nil {
		return nil, errors.WithStack(err)
	}
	return m, nil
}

func createRangeScanKeysForEligibleMissingDataEntries(blkNum uint64) (startKey, endKey []byte) {
	fmt.Println("===createRangeScanKeysForEligibleMissingDataEntries==")
	startKey = append(eligibleMissingDataKeyPrefix, util.EncodeReverseOrderVarUint64(blkNum)...)
	endKey = append(eligibleMissingDataKeyPrefix, util.EncodeReverseOrderVarUint64(0)...)

	return startKey, endKey
}

func createRangeScanKeysForIneligibleMissingData(maxBlkNum uint64, ns, coll string) (startKey, endKey []byte) {
	fmt.Println("===createRangeScanKeysForIneligibleMissingData==")
	startKey = encodeMissingDataKey(
		&missingDataKey{
			nsCollBlk:  nsCollBlk{ns: ns, coll: coll, blkNum: maxBlkNum},
			isEligible: false,
		},
	)
	endKey = encodeMissingDataKey(
		&missingDataKey{
			nsCollBlk:  nsCollBlk{ns: ns, coll: coll, blkNum: 0},
			isEligible: false,
		},
	)
	return
}

func createRangeScanKeysForCollElg() (startKey, endKey []byte) {
	fmt.Println("===createRangeScanKeysForCollElg==")
	return encodeCollElgKey(math.MaxUint64),
		encodeCollElgKey(0)
}

func datakeyRange(blockNum uint64) (startKey, endKey []byte) {
	fmt.Println("===datakeyRange==")
	startKey = append(pvtDataKeyPrefix, version.NewHeight(blockNum, 0).ToBytes()...)
	endKey = append(pvtDataKeyPrefix, version.NewHeight(blockNum, math.MaxUint64).ToBytes()...)
	return
}

func eligibleMissingdatakeyRange(blkNum uint64) (startKey, endKey []byte) {
	fmt.Println("===eligibleMissingdatakeyRange==")
	startKey = append(eligibleMissingDataKeyPrefix, util.EncodeReverseOrderVarUint64(blkNum)...)
	endKey = append(eligibleMissingDataKeyPrefix, util.EncodeReverseOrderVarUint64(blkNum-1)...)
	return
}
