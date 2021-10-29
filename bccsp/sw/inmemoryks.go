/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package sw

import (
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/hyperledger/fabric/bccsp"
	"github.com/pkg/errors"
)

// NewInMemoryKeyStore instantiates an ephemeral in-memory keystore
func NewInMemoryKeyStore() bccsp.KeyStore {
	fmt.Println("==NewInMemoryKeyStore==")
	eks := &inmemoryKeyStore{}
	eks.keys = make(map[string]bccsp.Key)
	return eks
}

type inmemoryKeyStore struct {
	// keys maps the hex-encoded SKI to keys
	keys map[string]bccsp.Key
	m    sync.RWMutex
}

// ReadOnly returns false - the key store is not read-only
func (ks *inmemoryKeyStore) ReadOnly() bool {
	fmt.Println("==inmemoryKeyStore=ReadOnly=")
	return false
}

// GetKey returns a key object whose SKI is the one passed.
func (ks *inmemoryKeyStore) GetKey(ski []byte) (bccsp.Key, error) {
	fmt.Println("==inmemoryKeyStore=GetKey=")
	if len(ski) == 0 {
		return nil, errors.New("ski is nil or empty")
	}

	skiStr := hex.EncodeToString(ski)

	ks.m.RLock()
	defer ks.m.RUnlock()
	if key, found := ks.keys[skiStr]; found {
		return key, nil
	}
	return nil, errors.Errorf("no key found for ski %x", ski)
}

// StoreKey stores the key k in this KeyStore.
func (ks *inmemoryKeyStore) StoreKey(k bccsp.Key) error {
	fmt.Println("==inmemoryKeyStore=StoreKey=")
	if k == nil {
		return errors.New("key is nil")
	}

	ski := hex.EncodeToString(k.SKI())

	ks.m.Lock()
	defer ks.m.Unlock()

	if _, found := ks.keys[ski]; found {
		return errors.Errorf("ski %x already exists in the keystore", k.SKI())
	}
	ks.keys[ski] = k

	return nil
}
