/*
Copyright IBM Corp All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package discovery

import (
	"encoding/hex"
	"sync"

	"github.com/hyperledger/fabric/common/util"
)

// MemoizeSigner signs messages with the same signature
// if the message was signed recently
type MemoizeSigner struct {
	maxEntries uint
	sync.RWMutex
	memory map[string][]byte
	sign   Signer
}

// NewMemoizeSigner creates a new MemoizeSigner that signs
// message with the given sign function
func NewMemoizeSigner(signFunc Signer, maxEntries uint) *MemoizeSigner {
	logger.Info("====NewMemoizeSigner==")
	return &MemoizeSigner{
		maxEntries: maxEntries,
		memory:     make(map[string][]byte),
		sign:       signFunc,
	}
}

// Signer signs a message and returns the signature and nil,
// or nil and error on failure
func (ms *MemoizeSigner) Sign(msg []byte) ([]byte, error) {
	logger.Info("====MemoizeSigner==Sign==")
	logger.Info("====msg====",msg)
	sig, isInMemory := ms.lookup(msg)
	logger.Info("===sig===",sig)//[]
	logger.Info("===isInMemory===",isInMemory)//false
	if isInMemory {
		return sig, nil
	}
	sig, err := ms.sign(msg)
	if err != nil {
		return nil, err
	}
	ms.memorize(msg, sig)
	return sig, nil
}

// lookup looks up the given message in memory and returns
// the signature, if the message is in memory
func (ms *MemoizeSigner) lookup(msg []byte) ([]byte, bool) {
	logger.Info("====MemoizeSigner==lookup==")
	ms.RLock()
	defer ms.RUnlock()
	logger.Info("=========msg=",msg)//=========msg= [10 39 10 3 1 2 3 18
	a := msgDigest(msg)
	logger.Info("=========a=",a)//a369a57fa4d2f3cd5a0636b58cac5f8054b14db5964b07136e3585e95cf8cce7
	sig, exists := ms.memory[a]
	logger.Info("==========sig=",sig)//[]
	logger.Info("==========exists=",exists)//false

	return sig, exists
}

func (ms *MemoizeSigner) memorize(msg, signature []byte) {
	logger.Info("====MemoizeSigner==memorize==")
	logger.Info("======msg",ms)
	logger.Info("========signature",signature)
	logger.Info("=======ms.maxEntries",ms.maxEntries)
	if ms.maxEntries == 0 {
		return
	}
	ms.RLock()

	shouldShrink := len(ms.memory) >= (int)(ms.maxEntries)
	logger.Info("=========len(ms.memory)=====",len(ms.memory))
	logger.Info("=========(int)(ms.maxEntries)=====",(int)(ms.maxEntries))
	logger.Info("===========shouldShrink=========",shouldShrink)
	ms.RUnlock()

	if shouldShrink {
		ms.shrinkMemory()
	}
	ms.Lock()
	defer ms.Unlock()
	ms.memory[msgDigest(msg)] = signature

}

// evict evicts random messages from memory
// until its size is smaller than maxEntries
func (ms *MemoizeSigner) shrinkMemory() {
	logger.Info("====MemoizeSigner==shrinkMemory==")
	ms.Lock()
	defer ms.Unlock()
	logger.Info("===len(ms.memory)==",len(ms.memory))
	logger.Info("===(int)(ms.maxEntries)==",(int)(ms.maxEntries))
	for len(ms.memory) > (int)(ms.maxEntries) {
		ms.evictFromMemory()
	}
}

// evictFromMemory evicts a random message from memory
func (ms *MemoizeSigner) evictFromMemory() {
	logger.Info("====MemoizeSigner==evictFromMemory==")
	for dig := range ms.memory {
		logger.Info("===========dig",dig)
		logger.Info("====ms.memory==",ms.memory)
		logger.Info("=====dig===",dig)
		delete(ms.memory, dig)
		return
	}
}

// msgDigest returns a digest of a given message
func msgDigest(msg []byte) string {
	logger.Info("====msgDigest==")
	logger.Info("====msg====",msg)//====msg==== [10 39 10 ]
	logger.Info("======util.ComputeSHA256(msg)=====",util.ComputeSHA256(msg))//[163 105 165 127 164 210 243
	a:= hex.EncodeToString(util.ComputeSHA256(msg))
	logger.Info("=============a===========",a)//a369a57fa4d2f3cd5a0636b58cac5f8054b14db5964b07136e3585e95cf8cce7
	return a
}
