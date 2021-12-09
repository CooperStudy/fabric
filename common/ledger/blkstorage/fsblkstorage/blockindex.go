/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package fsblkstorage

import (
	"bytes"
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/common/ledger/blkstorage"
	"github.com/hyperledger/fabric/common/ledger/util"
	"github.com/hyperledger/fabric/common/ledger/util/leveldbhelper"
	ledgerUtil "github.com/hyperledger/fabric/core/ledger/util"
	"github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric/protos/peer"
	"github.com/pkg/errors"
)

const (
	blockNumIdxKeyPrefix           = 'n'
	blockHashIdxKeyPrefix          = 'h'
	txIDIdxKeyPrefix               = 't'
	blockNumTranNumIdxKeyPrefix    = 'a'
	blockTxIDIdxKeyPrefix          = 'b'
	txValidationResultIdxKeyPrefix = 'v'
	indexCheckpointKeyStr          = "indexCheckpointKey"
)

var indexCheckpointKey = []byte(indexCheckpointKeyStr)
var errIndexEmpty = errors.New("NoBlockIndexed")

type index interface {
	getLastBlockIndexed() (uint64, error)
	indexBlock(blockIdxInfo *blockIdxInfo) error
	getBlockLocByHash(blockHash []byte) (*fileLocPointer, error)
	getBlockLocByBlockNum(blockNum uint64) (*fileLocPointer, error)
	getTxLoc(txID string) (*fileLocPointer, error)
	getTXLocByBlockNumTranNum(blockNum uint64, tranNum uint64) (*fileLocPointer, error)
	getBlockLocByTxID(txID string) (*fileLocPointer, error)
	getTxValidationCodeByTxID(txID string) (peer.TxValidationCode, error)
}

type blockIdxInfo struct {
	blockNum  uint64
	blockHash []byte
	flp       *fileLocPointer
	txOffsets []*txindexInfo
	metadata  *common.BlockMetadata
}

type blockIndex struct {
	indexItemsMap map[blkstorage.IndexableAttr]bool
	db            *leveldbhelper.DBHandle
}

func newBlockIndex(indexConfig *blkstorage.IndexConfig, db *leveldbhelper.DBHandle) (*blockIndex, error) {
	logger.Info("====newBlockIndex=====")
	indexItems := indexConfig.AttrsToIndex
	logger.Infof("newBlockIndex() - indexItems:[%s]", indexItems)
	indexItemsMap := make(map[blkstorage.IndexableAttr]bool)
	for _, indexItem := range indexItems {
		indexItemsMap[indexItem] = true
	}
	// This dependency is needed because the index 'IndexableAttrTxID' is used for detecting the duplicate txid
	// and the results are reused in the other two indexes. Ideally, all three index should be merged into one
	// for efficiency purpose - [FAB-10587]
	if (indexItemsMap[blkstorage.IndexableAttrTxValidationCode] || indexItemsMap[blkstorage.IndexableAttrBlockTxID]) &&
		!indexItemsMap[blkstorage.IndexableAttrTxID] {
		return nil, errors.Errorf("dependent index [%s] is not enabled for [%s] or [%s]",
			blkstorage.IndexableAttrTxID, blkstorage.IndexableAttrTxValidationCode, blkstorage.IndexableAttrBlockTxID)
	}
	return &blockIndex{indexItemsMap, db}, nil
}

func (index *blockIndex) getLastBlockIndexed() (uint64, error) {
	logger.Info("==========func (index *blockIndex) getLastBlockIndexed() (uint64, error)===============")
	var blockNumBytes []byte
	var err error
	if blockNumBytes, err = index.db.Get(indexCheckpointKey); err != nil {
		return 0, err
	}
	if blockNumBytes == nil {
		return 0, errIndexEmpty
	}
	return decodeBlockNum(blockNumBytes), nil
}

func (index *blockIndex) indexBlock(blockIdxInfo *blockIdxInfo) error {
	logger.Info("====blockIndex===indexBlock==")
	// do not index anything
	logger.Info("=====index.indexItemsMap ===========",index.indexItemsMap)//
	/*
	1.join
	index.indexItemsMap =6
	1.docker
	len(index.indexItemsMap) 1
	 */
	if len(index.indexItemsMap) == 0 {
		logger.Debug("Not indexing block... as nothing to index")
		return nil
	}
	logger.Info("Indexing block [%s]", blockIdxInfo)
	flp := blockIdxInfo.flp
	txOffsets := blockIdxInfo.txOffsets
	logger.Info("======txOffsets=================",txOffsets)
	//[0xc000227320]
	txsfltr := ledgerUtil.TxValidationFlags(blockIdxInfo.metadata.Metadata[common.BlockMetadataIndex_TRANSACTIONS_FILTER])
	batch := leveldbhelper.NewUpdateBatch()
	flpBytes, err := flp.marshal()
	if err != nil {
		return err
	}

	//Index1
	if _, ok := index.indexItemsMap[blkstorage.IndexableAttrBlockHash]; ok {
		batch.Put(constructBlockHashKey(blockIdxInfo.blockHash), flpBytes)
	}

	//Index2
	if _, ok := index.indexItemsMap[blkstorage.IndexableAttrBlockNum]; ok {
		batch.Put(constructBlockNumKey(blockIdxInfo.blockNum), flpBytes)
	}

	//Index3 Used to find a transaction by it's transaction id
	if _, ok := index.indexItemsMap[blkstorage.IndexableAttrTxID]; ok {
		if err = index.markDuplicateTxids(blockIdxInfo); err != nil {
			logger.Errorf("error detecting duplicate txids: %s", err)
			return errors.WithMessage(err, "error detecting duplicate txids")
		}
		for _, txoffset := range txOffsets {
			if txoffset.isDuplicate { // do not overwrite txid entry in the index - FAB-8557
				logger.Debugf("txid [%s] is a duplicate of a previous tx. Not indexing in txid-index", txoffset.txID)
				continue
			}
			txFlp := newFileLocationPointer(flp.fileSuffixNum, flp.offset, txoffset.loc)
			logger.Debugf("Adding txLoc [%s] for tx ID: [%s] to txid-index", txFlp, txoffset.txID)
			txFlpBytes, marshalErr := txFlp.marshal()
			if marshalErr != nil {
				return marshalErr
			}
			batch.Put(constructTxIDKey(txoffset.txID), txFlpBytes)
		}
	}

	//Index4 - Store BlockNumTranNum will be used to query history data
	if _, ok := index.indexItemsMap[blkstorage.IndexableAttrBlockNumTranNum]; ok {
		for txIterator, txoffset := range txOffsets {
			txFlp := newFileLocationPointer(flp.fileSuffixNum, flp.offset, txoffset.loc)
			logger.Debugf("Adding txLoc [%s] for tx number:[%d] ID: [%s] to blockNumTranNum index", txFlp, txIterator, txoffset.txID)
			txFlpBytes, marshalErr := txFlp.marshal()
			if marshalErr != nil {
				return marshalErr
			}
			batch.Put(constructBlockNumTranNumKey(blockIdxInfo.blockNum, uint64(txIterator)), txFlpBytes)
		}
	}

	// Index5 - Store BlockNumber will be used to find block by transaction id
	if _, ok := index.indexItemsMap[blkstorage.IndexableAttrBlockTxID]; ok {
		for _, txoffset := range txOffsets {
			if txoffset.isDuplicate { // do not overwrite txid entry in the index - FAB-8557
				continue
			}
			batch.Put(constructBlockTxIDKey(txoffset.txID), flpBytes)
		}
	}

	// Index6 - Store transaction validation result by transaction id
	if _, ok := index.indexItemsMap[blkstorage.IndexableAttrTxValidationCode]; ok {
		for idx, txoffset := range txOffsets {
			if txoffset.isDuplicate { // do not overwrite txid entry in the index - FAB-8557
				continue
			}
			batch.Put(constructTxValidationCodeIDKey(txoffset.txID), []byte{byte(txsfltr.Flag(idx))})
		}
	}

	batch.Put(indexCheckpointKey, encodeBlockNum(blockIdxInfo.blockNum))
	// Setting snyc to true as a precaution, false may be an ok optimization after further testing.
	if err := index.db.WriteBatch(batch, true); err != nil {
		return err
	}
	return nil
}

func (index *blockIndex) markDuplicateTxids(blockIdxInfo *blockIdxInfo) error {
	logger.Info("====blockIndex===markDuplicateTxids==")
	uniqueTxids := make(map[string]bool)
	for _, txIdxInfo := range blockIdxInfo.txOffsets {
		txid := txIdxInfo.txID
		if uniqueTxids[txid] { // txid is duplicate of a previous tx in the block
			txIdxInfo.isDuplicate = true
			continue
		}

		loc, err := index.getTxLoc(txid)
		if loc != nil { // txid is duplicate of a previous tx in the index
			txIdxInfo.isDuplicate = true
			continue
		}
		if err != blkstorage.ErrNotFoundInIndex {
			return err
		}
		uniqueTxids[txid] = true
	}
	return nil
}

func (index *blockIndex) getBlockLocByHash(blockHash []byte) (*fileLocPointer, error) {
	logger.Info("====blockIndex===getBlockLocByHash==")
	if _, ok := index.indexItemsMap[blkstorage.IndexableAttrBlockHash]; !ok {
		return nil, blkstorage.ErrAttrNotIndexed
	}
	b, err := index.db.Get(constructBlockHashKey(blockHash))
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, blkstorage.ErrNotFoundInIndex
	}
	blkLoc := &fileLocPointer{}
	blkLoc.unmarshal(b)
	return blkLoc, nil
}

func (index *blockIndex) getBlockLocByBlockNum(blockNum uint64) (*fileLocPointer, error) {
	logger.Info("====blockIndex===getBlockLocByBlockNum==")
	if _, ok := index.indexItemsMap[blkstorage.IndexableAttrBlockNum]; !ok {
		return nil, blkstorage.ErrAttrNotIndexed
	}
	b, err := index.db.Get(constructBlockNumKey(blockNum))
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, blkstorage.ErrNotFoundInIndex
	}
	blkLoc := &fileLocPointer{}
	blkLoc.unmarshal(b)
	return blkLoc, nil
}

func (index *blockIndex) getTxLoc(txID string) (*fileLocPointer, error) {
	logger.Info("====blockIndex===getTxLoc==")
	if _, ok := index.indexItemsMap[blkstorage.IndexableAttrTxID]; !ok {
		return nil, blkstorage.ErrAttrNotIndexed
	}
	b, err := index.db.Get(constructTxIDKey(txID))
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, blkstorage.ErrNotFoundInIndex
	}
	txFLP := &fileLocPointer{}
	txFLP.unmarshal(b)
	return txFLP, nil
}

func (index *blockIndex) getBlockLocByTxID(txID string) (*fileLocPointer, error) {
	logger.Info("====blockIndex===getBlockLocByTxID==")
	if _, ok := index.indexItemsMap[blkstorage.IndexableAttrBlockTxID]; !ok {
		return nil, blkstorage.ErrAttrNotIndexed
	}
	b, err := index.db.Get(constructBlockTxIDKey(txID))
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, blkstorage.ErrNotFoundInIndex
	}
	txFLP := &fileLocPointer{}
	txFLP.unmarshal(b)
	return txFLP, nil
}

func (index *blockIndex) getTXLocByBlockNumTranNum(blockNum uint64, tranNum uint64) (*fileLocPointer, error) {
	logger.Info("====blockIndex===getTXLocByBlockNumTranNum==")
	if _, ok := index.indexItemsMap[blkstorage.IndexableAttrBlockNumTranNum]; !ok {
		return nil, blkstorage.ErrAttrNotIndexed
	}
	b, err := index.db.Get(constructBlockNumTranNumKey(blockNum, tranNum))
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, blkstorage.ErrNotFoundInIndex
	}
	txFLP := &fileLocPointer{}
	txFLP.unmarshal(b)
	return txFLP, nil
}

func (index *blockIndex) getTxValidationCodeByTxID(txID string) (peer.TxValidationCode, error) {
	logger.Info("====blockIndex===getTxValidationCodeByTxID==")
	if _, ok := index.indexItemsMap[blkstorage.IndexableAttrTxValidationCode]; !ok {
		return peer.TxValidationCode(-1), blkstorage.ErrAttrNotIndexed
	}

	raw, err := index.db.Get(constructTxValidationCodeIDKey(txID))

	if err != nil {
		return peer.TxValidationCode(-1), err
	} else if raw == nil {
		return peer.TxValidationCode(-1), blkstorage.ErrNotFoundInIndex
	} else if len(raw) != 1 {
		return peer.TxValidationCode(-1), errors.New("invalid value in indexItems")
	}

	result := peer.TxValidationCode(int32(raw[0]))

	return result, nil
}

func constructBlockNumKey(blockNum uint64) []byte {
	logger.Info("====constructBlockNumKey===")
	blkNumBytes := util.EncodeOrderPreservingVarUint64(blockNum)
	return append([]byte{blockNumIdxKeyPrefix}, blkNumBytes...)
}

func constructBlockHashKey(blockHash []byte) []byte {
	logger.Info("====constructBlockHashKey===")
	return append([]byte{blockHashIdxKeyPrefix}, blockHash...)
}

func constructTxIDKey(txID string) []byte {
	logger.Info("====constructTxIDKey===")
	return append([]byte{txIDIdxKeyPrefix}, []byte(txID)...)
}

func constructBlockTxIDKey(txID string) []byte {
	logger.Info("====constructBlockTxIDKey===")
	return append([]byte{blockTxIDIdxKeyPrefix}, []byte(txID)...)
}

func constructTxValidationCodeIDKey(txID string) []byte {
	logger.Info("====constructTxValidationCodeIDKey===")
	return append([]byte{txValidationResultIdxKeyPrefix}, []byte(txID)...)
}

func constructBlockNumTranNumKey(blockNum uint64, txNum uint64) []byte {
	logger.Info("====constructBlockNumTranNumKey===")
	blkNumBytes := util.EncodeOrderPreservingVarUint64(blockNum)
	tranNumBytes := util.EncodeOrderPreservingVarUint64(txNum)
	key := append(blkNumBytes, tranNumBytes...)
	return append([]byte{blockNumTranNumIdxKeyPrefix}, key...)
}

func encodeBlockNum(blockNum uint64) []byte {
	logger.Info("====encodeBlockNum===")
	return proto.EncodeVarint(blockNum)
}

func decodeBlockNum(blockNumBytes []byte) uint64 {
	logger.Info("====decodeBlockNum===")
	blockNum, _ := proto.DecodeVarint(blockNumBytes)
	return blockNum
}

type locPointer struct {
	offset      int
	bytesLength int
}

func (lp *locPointer) String() string {
	logger.Info("====func (lp *locPointer) String() string=")
	a :=  fmt.Sprintf("offset=%d, bytesLength=%d", lp.offset, lp.bytesLength)
	logger.Info("=======a========",a)
	/*
	1.offset=38, bytesLength=12918

	*/
	return a
}

// fileLocPointer
type fileLocPointer struct {
	fileSuffixNum int
	locPointer
}

func newFileLocationPointer(fileSuffixNum int, beginningOffset int, relativeLP *locPointer) *fileLocPointer {
	logger.Info("====newFileLocationPointer=")
	logger.Info("=======fileSuffixNum=========",fileSuffixNum)
	flp := &fileLocPointer{fileSuffixNum: fileSuffixNum}
	flp.offset = beginningOffset + relativeLP.offset
	logger.Infof("=============flp.offset:%v = beginningOffset:%v+ relativeLP.offset:%v=====================",flp.offset,beginningOffset,relativeLP.offset)
	flp.bytesLength = relativeLP.bytesLength
	logger.Info("=======flp.bytesLength=========",flp.bytesLength)
	logger.Info("============flp",flp)
	return flp
}

func (flp *fileLocPointer) marshal() ([]byte, error) {
	logger.Info("===fileLocPointer=marshal=")
	buffer := proto.NewBuffer([]byte{})
	logger.Info("=========e := buffer.EncodeVarint(uint64(flp.fileSuffixNum))============",uint64(flp.fileSuffixNum))//0
	e := buffer.EncodeVarint(uint64(flp.fileSuffixNum))
	if e != nil {
		return nil, e
	}
	logger.Info("===================buffer.EncodeVarint(uint64(flp.offset))=======================",uint64(flp.offset))
	//0
	e = buffer.EncodeVarint(uint64(flp.offset))
	if e != nil {
		return nil, e
	}
	e = buffer.EncodeVarint(uint64(flp.bytesLength))
	if e != nil {
		return nil, e
	}
	logger.Info("==================e = buffer.EncodeVarint(uint64(flp.bytesLength))=======================",uint64(flp.bytesLength))//0

	logger.Info("=======================buffer.Bytes()=======",buffer.Bytes())//[0 0 0]


	return buffer.Bytes(), nil
}

func (flp *fileLocPointer) unmarshal(b []byte) error {
	logger.Info("===fileLocPointer=unmarshal=")
	buffer := proto.NewBuffer(b)
	logger.Info("============b==================",b)
	i, e := buffer.DecodeVarint()
	logger.Info("=========i, e := buffer.DecodeVarint()===============",i)
	if e != nil {
		return e
	}

	flp.fileSuffixNum = int(i)

	logger.Info("==========flp.fileSuffixNum===========",flp.fileSuffixNum)

	i, e = buffer.DecodeVarint()
	if e != nil {
		return e
	}
	flp.offset = int(i)
	logger.Info("==========flp.offset===========",flp.offset)
	i, e = buffer.DecodeVarint()
	if e != nil {
		return e
	}

	flp.bytesLength = int(i)
	logger.Info("==========flp.bytesLength===========",flp.bytesLength)
	return nil
}

func (flp *fileLocPointer) String() string {
	logger.Info("===fileLocPointer=String=")
	a:= fmt.Sprintf("fileSuffixNum=%d, %s", flp.fileSuffixNum, flp.locPointer.String())


	logger.Info("==================a===============",a)
	return a
}

func (blockIdxInfo *blockIdxInfo) String() string {
	logger.Info("===blockIdxInfo=String=")
	var buffer bytes.Buffer
	logger.Info("===========blockIdxInfo.txOffsets===================",blockIdxInfo.txOffsets)
	logger.Info("===========len(blockIdxInfo.txOffsets)===================",len(blockIdxInfo.txOffsets))
	for _, txOffset := range blockIdxInfo.txOffsets {
		buffer.WriteString("txId=")
		logger.Info("==========txId=txOffset.txID=============",txOffset.txID)
		//e691f573514613025eee38ccdaa28bd8ed8858015ae52968031c2388bfae7be5
		buffer.WriteString(txOffset.txID)
		buffer.WriteString(" locPointer=")
		buffer.WriteString(txOffset.loc.String())
		buffer.WriteString("\n")
		logger.Infof("=============txId=txOffset.txID:%v locPointer=txOffset.loc.String():%v===================================",txOffset.txID,txOffset.loc.String())
	   //txId=txOffset.txID:e691f573514613025eee38ccdaa28bd8ed8858015ae52968031c2388bfae7be5 locPointer=txOffset.loc.String():offset=38, bytesLength=12918=
	}
	txOffsetsString := buffer.String()

	a  := fmt.Sprintf("blockNum=%d, blockHash=%#v txOffsets=%s", blockIdxInfo.blockNum, blockIdxInfo.blockHash, txOffsetsString)
	logger.Info("=====================a=====",a)
	/*
	blockNum=0,
	blockHash=[]byte{0x4c, 0x1e, 0xcc, 0x9f, 0x9d, 0xb7, 0xe6, 0xad, 0xf3, 0xd3, 0x3e, 0xad, 0xda, 0x7e, 0x24, 0xbc, 0xc8, 0x9f, 0xab, 0x15, 0xec, 0x24, 0x25, 0xbe, 0x3d, 0x11, 0x8e, 0xa9, 0xd0, 0x87, 0xb4, 0xe4}
	txOffsets=txId=e691f573514613025eee38ccdaa28bd8ed8858015ae52968031c2388bfae7be5 locPointer=offset=38, bytesLength=12918

	*/
	return a
}
