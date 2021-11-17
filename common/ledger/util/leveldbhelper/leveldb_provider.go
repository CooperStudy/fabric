/*
Copyright IBM Corp. 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package leveldbhelper

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/iterator"
)

var dbNameKeySep = []byte{0x00}
var lastKeyIndicator = byte(0x01)

// Provider enables to use a single leveldb as multiple logical leveldbs
type Provider struct {
	db        *DB
	dbHandles map[string]*DBHandle
	mux       sync.Mutex
}

// NewProvider constructs a Provider
func NewProvider(conf *Conf) *Provider {
	fmt.Println("====NewProvider==")
	db := CreateDB(conf)
	db.Open()
	return &Provider{db, make(map[string]*DBHandle), sync.Mutex{}}
}

// GetDBHandle returns a handle to a named db
func (p *Provider) GetDBHandle(dbName string) *DBHandle {
	fmt.Println("====Provider==GetDBHandle==")
	p.mux.Lock()
	defer p.mux.Unlock()
	fmt.Println("======dbName====",dbName)
	dbHandle := p.dbHandles[dbName]
	if dbHandle == nil {
		fmt.Println("===========dbHandle not exist======",dbHandle)
		dbHandle = &DBHandle{dbName, p.db}
		p.dbHandles[dbName] = dbHandle
	}
	fmt.Println("===========dbHandle exist======",dbHandle)
	return dbHandle
}

// Close closes the underlying leveldb
func (p *Provider) Close() {
	fmt.Println("====Provider==Close==")
	p.db.Close()
}

// DBHandle is an handle to a named db
type DBHandle struct {
	dbName string
	db     *DB
}

// Get returns the value for the given key
func (h *DBHandle) Get(key []byte) ([]byte, error) {
	fmt.Println("====DBHandle==Get==")
	return h.db.Get(constructLevelKey(h.dbName, key))
}

// Put saves the key/value
func (h *DBHandle) Put(key []byte, value []byte, sync bool) error {
	fmt.Println("====DBHandle==Put==")
	return h.db.Put(constructLevelKey(h.dbName, key), value, sync)
}

// Delete deletes the given key
func (h *DBHandle) Delete(key []byte, sync bool) error {
	fmt.Println("====DBHandle==Delete==")
	return h.db.Delete(constructLevelKey(h.dbName, key), sync)
}

// WriteBatch writes a batch in an atomic way
func (h *DBHandle) WriteBatch(batch *UpdateBatch, sync bool) error {
	fmt.Println("====DBHandle==WriteBatch==")
	if len(batch.KVs) == 0 {
		return nil
	}
	levelBatch := &leveldb.Batch{}
	for k, v := range batch.KVs {
		key := constructLevelKey(h.dbName, []byte(k))
		if v == nil {
			levelBatch.Delete(key)
		} else {
			levelBatch.Put(key, v)
		}
	}
	if err := h.db.WriteBatch(levelBatch, sync); err != nil {
		return err
	}
	return nil
}

// GetIterator gets an handle to iterator. The iterator should be released after the use.
// The resultset contains all the keys that are present in the db between the startKey (inclusive) and the endKey (exclusive).
// A nil startKey represents the first available key and a nil endKey represent a logical key after the last available key
func (h *DBHandle) GetIterator(startKey []byte, endKey []byte) *Iterator {
	fmt.Println("====DBHandle==GetIterator==")
	sKey := constructLevelKey(h.dbName, startKey)
	eKey := constructLevelKey(h.dbName, endKey)
	if endKey == nil {
		// replace the last byte 'dbNameKeySep' by 'lastKeyIndicator'
		eKey[len(eKey)-1] = lastKeyIndicator
	}
	logger.Debugf("Getting iterator for range [%#v] - [%#v]", sKey, eKey)
	return &Iterator{h.db.GetIterator(sKey, eKey)}
}

// UpdateBatch encloses the details of multiple `updates`
type UpdateBatch struct {
	KVs map[string][]byte
}

// NewUpdateBatch constructs an instance of a Batch
func NewUpdateBatch() *UpdateBatch {
	fmt.Println("====NewUpdateBatch=")
	return &UpdateBatch{make(map[string][]byte)}
}

// Put adds a KV
func (batch *UpdateBatch) Put(key []byte, value []byte) {
	fmt.Println("====UpdateBatch===Put==")
	if value == nil {
		panic("Nil value not allowed")
	}
	batch.KVs[string(key)] = value
}

// Delete deletes a Key and associated value
func (batch *UpdateBatch) Delete(key []byte) {
	fmt.Println("====UpdateBatch===Delete==")
	batch.KVs[string(key)] = nil
}

// Len returns the number of entries in the batch
func (batch *UpdateBatch) Len() int {
	fmt.Println("====UpdateBatch===Len==")
	return len(batch.KVs)
}

// Iterator extends actual leveldb iterator
type Iterator struct {
	iterator.Iterator
}

// Key wraps actual leveldb iterator method
func (itr *Iterator) Key() []byte {
	fmt.Println("====Iterator===Key==")
	return retrieveAppKey(itr.Iterator.Key())
}

func constructLevelKey(dbName string, key []byte) []byte {
	fmt.Println("====constructLevelKey=")
	return append(append([]byte(dbName), dbNameKeySep...), key...)
}

func retrieveAppKey(levelKey []byte) []byte {
	fmt.Println("====retrieveAppKey====")
	return bytes.SplitN(levelKey, dbNameKeySep, 2)[1]
}
