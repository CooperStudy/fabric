/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package discovery

import (
	"encoding/asn1"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/hyperledger/fabric/common/util"
	"github.com/hyperledger/fabric/protos/common"
	"github.com/pkg/errors"
)

const (
	defaultMaxCacheSize   = 1000
	defaultRetentionRatio = 0.75
)

var (
	// asBytes is the function that is used to marshal common.SignedData to bytes
	asBytes = asn1.Marshal
)

type acSupport interface {
	// Eligible returns whether the given peer is eligible for receiving
	// service from the discovery service for a given channel
	EligibleForService(channel string, data common.SignedData) error

	// ConfigSequence returns the configuration sequence of the given channel
	ConfigSequence(channel string) uint64
}

type authCacheConfig struct {
	enabled bool
	// maxCacheSize is the maximum size of the cache, after which
	// a purge takes place
	maxCacheSize int
	// purgeRetentionRatio is the % of entries that remain in the cache
	// after the cache is purged due to overpopulation
	purgeRetentionRatio float64
}

// authCache defines an interface that authenticates a request in a channel context,
// and does memoization of invocations
type authCache struct {
	credentialCache map[string]*accessCache
	acSupport
	sync.RWMutex
	conf authCacheConfig
}

func newAuthCache(s acSupport, conf authCacheConfig) *authCache {
	logger.Info("=======newAuthCache====")
	return &authCache{
		acSupport:       s,
		credentialCache: make(map[string]*accessCache),
		conf:            conf,
	}
}

// Eligible returns whether the given peer is eligible for receiving
// service from the discovery service for a given channel
func (ac *authCache) EligibleForService(channel string, data common.SignedData) error {
	logger.Info("=======authCache==EligibleForService==")
	//logger.Info("=======!ac.conf.enabled================",!ac.conf.enabled)//true
	if !ac.conf.enabled {
		return ac.acSupport.EligibleForService(channel, data)
	}
	// Check whether we already have a cache for this channel
	ac.RLock()
	//logger.Info("==========channel=========",channel)//mychannel
	cache := ac.credentialCache[channel]
	//logger.Info("========cache========",cache)
	/*
	&{{{0 0} 0 0 0 0} mychannel 0xc000335180 1 map[cd210851145934f0311641bb6f8f6286ea2a7069207a43a0642422ce96e40a19:<nil> bdaae382031c205c9de37d8dc86c83fa75bb9f8d4604072069d7cc1266601cc1:<nil> 4b5454557963d4f6e3fd6e0366946ab9d47c5c1c502d7b6a0cbe42a04656bb8e:<nil> 46a2f9184fa776a505f28b5c46305d4c0dd7d61274be9acdffc2be917c4d0d31:<nil> f8e9104492ab640b6218f45b215e8176cdc251d47292820257ab730ac4e053c8:<nil> af3e5f0e6e39fdcd9aa892daf52ea7f14a0b30942d1d8e31d22bd7c8b5365708:<nil> 7e555d6c26b399b4a00b3d8837c50d731deb3858a3415fba131e11c0e422439b:<nil> 63bf3671f8c4e0f2ffe0e07d88d91c83c170691ab9501a4fa32483cc5f79e677:<nil> 689b6e2b3b9dc6f8d0820994df2e5f9cf9677385bd554f7f1018ee8d1a2a390e:<nil>]}
	 */
	ac.RUnlock()
	if cache == nil {
		// Cache for given channel wasn't found, so create a new one
		ac.Lock()
		cache = ac.newAccessCache(channel)
		// And store the cache instance.
		ac.credentialCache[channel] = cache
		ac.Unlock()
	}
	return cache.EligibleForService(data)
}

type accessCache struct {
	sync.RWMutex
	channel      string
	ac           *authCache
	lastSequence uint64
	entries      map[string]error
}

func (ac *authCache) newAccessCache(channel string) *accessCache {
	logger.Info("=======authCache==newAccessCache==")
	c := &accessCache{
		channel: channel,
		ac:      ac,
		entries: make(map[string]error),
	}
	logger.Info("==================c",c)
	return c
}

func (cache *accessCache) EligibleForService(data common.SignedData) error {
	logger.Info("=======authCache==EligibleForService==")
	key, err := signedDataToKey(data)
	if err != nil {
		logger.Warningf("Failed computing key of signed data: +%v", err)
		return errors.Wrap(err, "failed computing key of signed data")
	}
	currSeq := cache.ac.acSupport.ConfigSequence(cache.channel)
	if cache.isValid(currSeq) {
		foundInCache, isEligibleErr := cache.lookup(key)
		if foundInCache {
			return isEligibleErr
		}
	} else {
		cache.configChange(currSeq)
	}

	// Make sure the cache doesn't overpopulate.
	// It might happen that it overgrows the maximum size due to concurrent
	// goroutines waiting on the lock above, but that's acceptable.
	cache.purgeEntriesIfNeeded()

	// Compute the eligibility of the client for the service
	err = cache.ac.acSupport.EligibleForService(cache.channel, data)
	cache.Lock()
	defer cache.Unlock()
	// Check if the sequence hasn't changed since last time
	if currSeq != cache.ac.acSupport.ConfigSequence(cache.channel) {
		// The sequence at which we computed the eligibility might have changed,
		// so we can't put it into the cache because a more fresh computation result
		// might already be present in the cache by now, and we don't want to override it
		// with a stale computation result, so just return the result.
		return err
	}
	// Else, the eligibility of the client has been computed under the latest sequence,
	// so store the computation result in the cache
	cache.entries[key] = err
	return err
}

func (cache *accessCache) isPurgeNeeded() bool {
	logger.Info("=======accessCache==isPurgeNeeded==")
	cache.RLock()
	defer cache.RUnlock()
	return len(cache.entries)+1 > cache.ac.conf.maxCacheSize
}

func (cache *accessCache) purgeEntriesIfNeeded() {
	logger.Info("=======accessCache==purgeEntriesIfNeeded==")
	if !cache.isPurgeNeeded() {
		return
	}

	cache.Lock()
	defer cache.Unlock()

	maxCacheSize := cache.ac.conf.maxCacheSize
	purgeRatio := cache.ac.conf.purgeRetentionRatio
	entries2evict := maxCacheSize - int(purgeRatio*float64(maxCacheSize))

	for key := range cache.entries {
		if entries2evict == 0 {
			return
		}
		entries2evict--
		delete(cache.entries, key)
	}
}

func (cache *accessCache) isValid(currSeq uint64) bool {
	logger.Info("=======accessCache==isValid==")
	cache.RLock()
	defer cache.RUnlock()
	return currSeq == cache.lastSequence
}

func (cache *accessCache) configChange(currSeq uint64) {
	logger.Info("=======accessCache==configChange==")
	cache.Lock()
	defer cache.Unlock()
	cache.lastSequence = currSeq
	// Invalidate entries
	cache.entries = make(map[string]error)
}

func (cache *accessCache) lookup(key string) (cacheHit bool, lookupResult error) {
	logger.Info("=======accessCache==lookup==")
	cache.RLock()
	defer cache.RUnlock()

	lookupResult, cacheHit = cache.entries[key]
	return
}

func signedDataToKey(data common.SignedData) (string, error) {
	logger.Info("=======signedDataToKey==")
	b, err := asBytes(data)
	if err != nil {
		return "", errors.Wrap(err, "failed marshaling signed data")
	}
	return hex.EncodeToString(util.ComputeSHA256(b)), nil
}
