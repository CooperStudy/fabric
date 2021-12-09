/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package utils

import (
	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/common/flogging"
	cb "github.com/hyperledger/fabric/protos/common"
	"github.com/pkg/errors"
)

// GetChainIDFromBlockBytes returns chain ID given byte array which represents
// the block
var logger = flogging.MustGetLogger("protos.utils")
func GetChainIDFromBlockBytes(bytes []byte) (string, error) {
	logger.Info("===========GetChainIDFromBlockBytes========")
	block, err := GetBlockFromBlockBytes(bytes)
	if err != nil {
		return "", err
	}

	return GetChainIDFromBlock(block)
}

// GetChainIDFromBlock returns chain ID in the block
func GetChainIDFromBlock(block *cb.Block) (string, error) {
	logger.Info("===========GetChainIDFromBlock========")
	if block == nil || block.Data == nil || block.Data.Data == nil || len(block.Data.Data) == 0 {
		return "", errors.Errorf("failed to retrieve channel id - block is empty")
	}
	var err error
	envelope, err := GetEnvelopeFromBlock(block.Data.Data[0])
	if err != nil {
		return "", err
	}
	payload, err := GetPayload(envelope)
	if err != nil {
		return "", err
	}

	if payload.Header == nil {
		return "", errors.Errorf("failed to retrieve channel id - payload header is empty")
	}
	chdr, err := UnmarshalChannelHeader(payload.Header.ChannelHeader)
	if err != nil {
		return "", err
	}

	return chdr.ChannelId, nil
}

func GetChannelHeaderFromBlock(block *cb.Block) (*cb.ChannelHeader, error) {
	logger.Info("===========GetChainIDFromBlock========")
	if block == nil || block.Data == nil || block.Data.Data == nil || len(block.Data.Data) == 0 {
		return nil, errors.Errorf("failed to retrieve channel id - block is empty")
	}
	var err error
	envelope, err := GetEnvelopeFromBlock(block.Data.Data[0])
	if err != nil {
		return nil, err
	}
	payload, err := GetPayload(envelope)
	if err != nil {
		return nil, err
	}

	if payload.Header == nil {
		return nil, errors.Errorf("failed to retrieve channel id - payload header is empty")
	}
	chdr, err := UnmarshalChannelHeader(payload.Header.ChannelHeader)
	if err != nil {
		return nil, err
	}

	return chdr, nil
}

// GetMetadataFromBlock retrieves metadata at the specified index.
func GetMetadataFromBlock(block *cb.Block, index cb.BlockMetadataIndex) (*cb.Metadata, error) {
	logger.Info("===========GetMetadataFromBlock========")
	md := &cb.Metadata{}
	err := proto.Unmarshal(block.Metadata.Metadata[index], md)
	if err != nil {
		return nil, errors.Wrapf(err, "error unmarshaling metadata from block at index [%s]", index)
	}
	return md, nil
}

// GetMetadataFromBlockOrPanic retrieves metadata at the specified index, or
// panics on error
func GetMetadataFromBlockOrPanic(block *cb.Block, index cb.BlockMetadataIndex) *cb.Metadata {
	logger.Info("===========GetMetadataFromBlockOrPanic========")
	md, err := GetMetadataFromBlock(block, index)
	if err != nil {
		panic(err)
	}
	return md
}

// GetLastConfigIndexFromBlock retrieves the index of the last config block as
// encoded in the block metadata
func GetLastConfigIndexFromBlock(block *cb.Block) (uint64, error) {
	logger.Info("===========GetLastConfigIndexFromBlock========")
	md, err := GetMetadataFromBlock(block, cb.BlockMetadataIndex_LAST_CONFIG)
	if err != nil {
		return 0, err
	}
	lc := &cb.LastConfig{}
	err = proto.Unmarshal(md.Value, lc)
	if err != nil {
		return 0, errors.Wrap(err, "error unmarshaling LastConfig")
	}
	return lc.Index, nil
}

// GetLastConfigIndexFromBlockOrPanic retrieves the index of the last config
// block as encoded in the block metadata, or panics on error
func GetLastConfigIndexFromBlockOrPanic(block *cb.Block) uint64 {
	logger.Info("===========GetLastConfigIndexFromBlockOrPanic========")
	index, err := GetLastConfigIndexFromBlock(block)
	if err != nil {
		panic(err)
	}
	return index
}

// GetBlockFromBlockBytes marshals the bytes into Block
func GetBlockFromBlockBytes(blockBytes []byte) (*cb.Block, error) {
	logger.Info("===========GetBlockFromBlockBytes========")
	block := &cb.Block{}
	err := proto.Unmarshal(blockBytes, block)
	if err != nil {
		return block, errors.Wrap(err, "error unmarshaling block")
	}
	return block, nil
}

// CopyBlockMetadata copies metadata from one block into another
func CopyBlockMetadata(src *cb.Block, dst *cb.Block) {
	logger.Info("===========CopyBlockMetadata========")
	dst.Metadata = src.Metadata
	// Once copied initialize with rest of the
	// required metadata positions.
	InitBlockMetadata(dst)
}

// InitBlockMetadata copies metadata from one block into another
func InitBlockMetadata(block *cb.Block) {
	logger.Info("===========InitBlockMetadata========")
	if block.Metadata == nil {
		block.Metadata = &cb.BlockMetadata{Metadata: [][]byte{{}, {}, {}}}
	} else if len(block.Metadata.Metadata) < int(cb.BlockMetadataIndex_TRANSACTIONS_FILTER+1) {
		for i := int(len(block.Metadata.Metadata)); i <= int(cb.BlockMetadataIndex_TRANSACTIONS_FILTER); i++ {
			block.Metadata.Metadata = append(block.Metadata.Metadata, []byte{})
		}
	}
}
