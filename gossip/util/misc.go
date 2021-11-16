/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package util

import (
	cryptorand "crypto/rand"
	"fmt"
	"io"
	"math/big"
	"math/rand"
	"reflect"
	"runtime"
	"sync"
	"time"

	"github.com/spf13/viper"
)

// Equals returns whether a and b are the same
type Equals func(a interface{}, b interface{}) bool

var viperLock sync.RWMutex

// Contains returns whether a given slice a contains a string s
func Contains(s string, a []string) bool {
	//fmt.Println("===Contains==")
	for _, e := range a {
		if e == s {
			return true
		}
	}
	return false
}

// IndexInSlice returns the index of given object o in array
func IndexInSlice(array interface{}, o interface{}, equals Equals) int {
	//fmt.Println("===IndexInSlice==")
	arr := reflect.ValueOf(array)
	for i := 0; i < arr.Len(); i++ {
		if equals(arr.Index(i).Interface(), o) {
			return i
		}
	}
	return -1
}

func numbericEqual(a interface{}, b interface{}) bool {
	//fmt.Println("===numbericEqual==")
	return a.(int) == b.(int)
}

// GetRandomIndices returns a slice of random indices
// from 0 to given highestIndex
func GetRandomIndices(indiceCount, highestIndex int) []int {
	//fmt.Println("===GetRandomIndices==")
	if highestIndex+1 < indiceCount {
		return nil
	}

	indices := make([]int, 0)
	if highestIndex+1 == indiceCount {
		for i := 0; i < indiceCount; i++ {
			indices = append(indices, i)
		}
		return indices
	}

	for len(indices) < indiceCount {
		n := RandomInt(highestIndex + 1)
		if IndexInSlice(indices, n, numbericEqual) != -1 {
			continue
		}
		indices = append(indices, n)
	}
	return indices
}

// Set is a generic and thread-safe
// set container
type Set struct {
	items map[interface{}]struct{}
	lock  *sync.RWMutex
}

// NewSet returns a new set
func NewSet() *Set {
	//fmt.Println("===NewSet==")
	return &Set{lock: &sync.RWMutex{}, items: make(map[interface{}]struct{})}
}

// Add adds given item to the set
func (s *Set) Add(item interface{}) {
	//fmt.Println("===Set==Add==")
	s.lock.Lock()
	defer s.lock.Unlock()
	s.items[item] = struct{}{}
}

// Exists returns true whether given item is in the set
func (s *Set) Exists(item interface{}) bool {
	//fmt.Println("===Set==Exists==")
	s.lock.RLock()
	defer s.lock.RUnlock()
	_, exists := s.items[item]
	return exists
}

// Size returns the size of the set
func (s *Set) Size() int {
	//fmt.Println("===Set==Size==")
	s.lock.RLock()
	defer s.lock.RUnlock()
	return len(s.items)
}

// ToArray returns a slice with items
// at the point in time the method was invoked
func (s *Set) ToArray() []interface{} {
	//fmt.Println("===Set==ToArray==")
	s.lock.RLock()
	defer s.lock.RUnlock()
	a := make([]interface{}, len(s.items))
	i := 0
	for item := range s.items {
		a[i] = item
		i++
	}
	return a
}

// Clear removes all elements from set
func (s *Set) Clear() {
	//fmt.Println("===Set==Clear==")
	s.lock.Lock()
	defer s.lock.Unlock()
	s.items = make(map[interface{}]struct{})
}

// Remove removes a given item from the set
func (s *Set) Remove(item interface{}) {
	//fmt.Println("===Set==Remove==")
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.items, item)
}

// PrintStackTrace prints to stdout
// all goroutines
func PrintStackTrace() {
	//fmt.Println("===PrintStackTrace==")
	buf := make([]byte, 1<<16)
	runtime.Stack(buf, true)
	fmt.Printf("%s", buf)
}

// GetIntOrDefault returns the int value from config if present otherwise default value
func GetIntOrDefault(key string, defVal int) int {
	//fmt.Println("===GetIntOrDefault==")
	viperLock.RLock()
	defer viperLock.RUnlock()

	if val := viper.GetInt(key); val != 0 {
		return val
	}

	return defVal
}

// GetFloat64OrDefault returns the float64 value from config if present otherwise default value
func GetFloat64OrDefault(key string, defVal float64) float64 {
	//fmt.Println("===GetFloat64OrDefault==")
	viperLock.RLock()
	defer viperLock.RUnlock()

	if val := viper.GetFloat64(key); val != 0 {
		return val
	}

	return defVal
}

// GetDurationOrDefault returns the Duration value from config if present otherwise default value
func GetDurationOrDefault(key string, defVal time.Duration) time.Duration {
	//fmt.Println("===GetDurationOrDefault==")
	viperLock.RLock()
	defer viperLock.RUnlock()

	if val := viper.GetDuration(key); val != 0 {
		return val
	}

	return defVal
}

// SetVal stores key value to viper
func SetVal(key string, val interface{}) {
	//fmt.Println("===SetVal==")
	viperLock.Lock()
	defer viperLock.Unlock()
	viper.Set(key, val)
}

// RandomInt returns, as an int, a non-negative pseudo-random integer in [0,n)
// It panics if n <= 0
func RandomInt(n int) int {
	//fmt.Println("===RandomInt==")
	if n <= 0 {
		panic(fmt.Sprintf("Got invalid (non positive) value: %d", n))
	}
	m := int(RandomUInt64()) % n
	if m < 0 {
		return n + m
	}
	return m
}

// RandomUInt64 returns a random uint64
func RandomUInt64() uint64 {
	//fmt.Println("===RandomUInt64==")
	b := make([]byte, 8)
	_, err := io.ReadFull(cryptorand.Reader, b)
	//fmt.Println("===err=====",err)
	if err == nil {
		n := new(big.Int)
		b:= n.SetBytes(b).Uint64()
		//fmt.Println("===========RandomUInt64",b)
		return b
	}
	rand.Seed(rand.Int63())
	return uint64(rand.Int63())
}

func BytesToStrings(bytes [][]byte) []string {
	//fmt.Println("===BytesToStrings==")
	strings := make([]string, len(bytes))
	for i, b := range bytes {
		strings[i] = string(b)
	}
	return strings
}

func StringsToBytes(strings []string) [][]byte {
	//fmt.Println("===StringsToBytes==")
	bytes := make([][]byte, len(strings))
	for i, str := range strings {
		bytes[i] = []byte(str)
	}
	return bytes
}
