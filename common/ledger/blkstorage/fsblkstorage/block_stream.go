/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package fsblkstorage

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
)

// ErrUnexpectedEndOfBlockfile error used to indicate an unexpected end of a file segment
// this can happen mainly if a crash occurs during appening a block and partial block contents
// get written towards the end of the file
var ErrUnexpectedEndOfBlockfile = errors.New("unexpected end of blockfile")

// blockfileStream reads blocks sequentially from a single file.
// It starts from the given offset and can traverse till the end of the file
type blockfileStream struct {
	fileNum       int
	file          *os.File
	reader        *bufio.Reader
	currentOffset int64
}

// blockStream reads blocks sequentially from multiple files.
// it starts from a given file offset and continues with the next
// file segment until the end of the last segment (`endFileNum`)
type blockStream struct {
	rootDir           string
	currentFileNum    int
	endFileNum        int
	currentFileStream *blockfileStream
}

// blockPlacementInfo captures the information related
// to block's placement in the file.
type blockPlacementInfo struct {
	fileNum          int
	blockStartOffset int64
	blockBytesOffset int64
}

///////////////////////////////////
// blockfileStream functions
////////////////////////////////////
func newBlockfileStream(rootDir string, fileNum int, startOffset int64) (*blockfileStream, error) {
	logger.Info("===newBlockfileStream===")
	logger.Info("================rootDir=====================",rootDir)
	logger.Info("================fileNum=====================",fileNum)
	logger.Info("=================startOffset================",startOffset)
	filePath := deriveBlockfilePath(rootDir, fileNum)
	logger.Info("newBlockfileStream(): filePath=[%s], startOffset=[%d]", filePath, startOffset)
	var file *os.File
	var err error
	file, err = os.OpenFile(filePath, os.O_RDONLY, 0600)
	if err != nil {
		return nil, errors.Wrapf(err, "error opening block file %s", filePath)
	}
	var newPosition int64
	 newPosition, err = file.Seek(startOffset, 0)
	 logger.Infof("============newPosition:%v, err = file.Seek(startOffset:%v, 0)=",newPosition,startOffset)
	if err != nil {
		return nil, errors.Wrapf(err, "error seeking block file [%s] to startOffset [%d]", filePath, startOffset)
	}
	logger.Info("=========newPosition===========",newPosition)
	logger.Info("=========startOffset===========",startOffset)
	if newPosition != startOffset {
		panic(fmt.Sprintf("Could not seek block file [%s] to startOffset [%d]. New position = [%d]",
			filePath, startOffset, newPosition))
	}
	logger.Info("&blockfileStream{fileNum, file, bufio.NewReader(file), startOffset}")
	s := &blockfileStream{fileNum, file, bufio.NewReader(file), startOffset}
	return s, nil
}

func (s *blockfileStream) nextBlockBytes() ([]byte, error) {
	logger.Info("===blockfileStream===nextBlockBytes==")
	blockBytes, _, err := s.nextBlockBytesAndPlacementInfo()
	return blockBytes, err
}

// nextBlockBytesAndPlacementInfo returns bytes for the next block
// along with the offset information in the block file.
// An error `ErrUnexpectedEndOfBlockfile` is returned if a partial written data is detected
// which is possible towards the tail of the file if a crash had taken place during appending of a block
func (s *blockfileStream) nextBlockBytesAndPlacementInfo() ([]byte, *blockPlacementInfo, error) {
	logger.Info("===blockfileStream===nextBlockBytesAndPlacementInfo==")
	var lenBytes []byte
	var err error
	var fileInfo os.FileInfo
	moreContentAvailable := true

	fileInfo, err = s.file.Stat()
	logger.Info("====fileInfo=====",fileInfo)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "error getting block file stat")
	}
	logger.Info("====s.currentOffset=========",s.currentOffset)
	logger.Info("====fileInfo.Size()=========",fileInfo.Size())
	if s.currentOffset == fileInfo.Size() {
		logger.Info("Finished reading file number [%d]", s.fileNum)
		return nil, nil, nil
	}


	remainingBytes := fileInfo.Size() - s.currentOffset
	logger.Infof("==========remainingBytes:%v := fileInfo.Size():%v - s.currentOffset:%v=====",remainingBytes,fileInfo.Size(),s.currentOffset)
	// Peek 8 or smaller number of bytes (if remaining bytes are less than 8)
	// Assumption is that a block size would be small enough to be represented in 8 bytes varint
	peekBytes := 8

	if remainingBytes < int64(peekBytes) {
		logger.Infof("===========remainingBytes < 0===========================")
		peekBytes = int(remainingBytes)
		moreContentAvailable = false
	}
	logger.Info("Remaining bytes=[%d], Going to peek [%d] bytes", remainingBytes, peekBytes)
	 lenBytes, err = s.reader.Peek(peekBytes)
	 if err != nil {
		return nil, nil, errors.Wrapf(err, "error peeking [%d] bytes from block file", peekBytes)
	}
	logger.Info("=========lenBytes===========",lenBytes)
	length, n := proto.DecodeVarint(lenBytes)
	logger.Info("=========length===========",length)
	if n == 0 {
		// proto.DecodeVarint did not consume any byte at all which means that the bytes
		// representing the size of the block are partial bytes
		if !moreContentAvailable {
			return nil, nil, ErrUnexpectedEndOfBlockfile
		}
		panic(errors.Errorf("Error in decoding varint bytes [%#v]", lenBytes))
	}
	bytesExpected := int64(n) + int64(length)
	logger.Infof("=======bytesExpected:%v := int64(n):%v + int64(length):%v==========================",bytesExpected,n,length)
	if bytesExpected > remainingBytes {
		logger.Info("==========bytesExpected > remainingBytes========================")
		logger.Debugf("At least [%d] bytes expected. Remaining bytes = [%d]. Returning with error [%s]",
			bytesExpected, remainingBytes, ErrUnexpectedEndOfBlockfile)
		return nil, nil, ErrUnexpectedEndOfBlockfile
	}
	// skip the bytes representing the block size
	 _, err = s.reader.Discard(n)
	 logger.Info("========= s.reader.Discard(n)=====",n)
	 if err != nil {
		return nil, nil, errors.Wrapf(err, "error discarding [%d] bytes", n)
	}
	blockBytes := make([]byte, length)
	logger.Info("=================_, err = io.ReadAtLeast(s.reader, blockBytes, int(length))=======================")
	_, err = io.ReadAtLeast(s.reader, blockBytes, int(length))
	if err != nil {
		logger.Errorf("Error reading [%d] bytes from file number [%d], error: %s", length, s.fileNum, err)
		return nil, nil, errors.Wrapf(err, "error reading [%d] bytes from file number [%d]", length, s.fileNum)
	}
	blockPlacementInfo := &blockPlacementInfo{
		fileNum:          s.fileNum,
		blockStartOffset: s.currentOffset,
		blockBytesOffset: s.currentOffset + int64(n)}

	logger.Info("+==========fileNum========",s.fileNum)
	logger.Info("+==========blockStartOffset========",s.currentOffset)
	logger.Info("+==========blockBytesOffset========",s.currentOffset + int64(n))

	s.currentOffset += int64(n) + int64(length)
	logger.Infof("Returning blockbytes - length=[%d], placementInfo={%s}", len(blockBytes), blockPlacementInfo)
	return blockBytes, blockPlacementInfo, nil
}

func (s *blockfileStream) close() error {
	return errors.WithStack(s.file.Close())
}

///////////////////////////////////
// blockStream functions
////////////////////////////////////
func newBlockStream(rootDir string, startFileNum int, startOffset int64, endFileNum int) (*blockStream, error) {
	logger.Info("===newBlockStream===")
	logger.Infof("===============rootDir:%v,startFileNum:%v,startOffset:%v================",rootDir,startFileNum,startOffset)
	startFileStream, err := newBlockfileStream(rootDir, startFileNum, startOffset)
	if err != nil {
		return nil, err
	}
	return &blockStream{rootDir, startFileNum, endFileNum, startFileStream}, nil
}

func (s *blockStream) moveToNextBlockfileStream() error {
	logger.Info("===blockStream===moveToNextBlockfileStream==")
	var err error
	if err = s.currentFileStream.close(); err != nil {
		return err
	}
	s.currentFileNum++
	logger.Info("=========s.currentFileNum++===========",s.currentFileNum)

	logger.Info("========s.rootDir=================",s.rootDir)
	logger.Info("=======s.currentFileNum================",s.currentFileNum)
	s.currentFileStream, err = newBlockfileStream(s.rootDir, s.currentFileNum, 0)

	if err != nil {
		return err
	}
	return nil
}

func (s *blockStream) nextBlockBytes() ([]byte, error) {
	logger.Info("===blockStream===nextBlockBytes==")
	blockBytes, _, err := s.nextBlockBytesAndPlacementInfo()
	return blockBytes, err
}

func (s *blockStream) nextBlockBytesAndPlacementInfo() ([]byte, *blockPlacementInfo, error) {
	logger.Info("===blockStream===nextBlockBytesAndPlacementInfo==")
	var blockBytes []byte
	var blockPlacementInfo *blockPlacementInfo
	var err error
	blockBytes, blockPlacementInfo, err = s.currentFileStream.nextBlockBytesAndPlacementInfo()
	logger.Info("==========blockBytes===========",blockBytes)

	if err != nil {
		logger.Errorf("Error reading next block bytes from file number [%d]: %s", s.currentFileNum, err)
		return nil, nil, err
	}
	logger.Info("blockbytes [%d] read from file [%d]", len(blockBytes), s.currentFileNum)
	if blockBytes == nil && (s.currentFileNum < s.endFileNum || s.endFileNum < 0) {
		logger.Info("current file [%d] exhausted. Moving to next file", s.currentFileNum)
		if err = s.moveToNextBlockfileStream(); err != nil {
			return nil, nil, err
		}
		return s.nextBlockBytesAndPlacementInfo()
	}
	return blockBytes, blockPlacementInfo, nil
}

func (s *blockStream) close() error {
	logger.Info("===blockStream===close==")
	return s.currentFileStream.close()
}

func (i *blockPlacementInfo) String() string {
	logger.Info("===blockPlacementInfo===String==")
	return fmt.Sprintf("fileNum=[%d], startOffset=[%d], bytesOffset=[%d]",
		i.fileNum, i.blockStartOffset, i.blockBytesOffset)
}
