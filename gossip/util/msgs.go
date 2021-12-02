/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package util

import (
	"sync"

	"github.com/hyperledger/fabric/gossip/common"
	proto "github.com/hyperledger/fabric/protos/gossip"
)

// MembershipStore struct which encapsulates
// membership message store abstraction
type MembershipStore struct {
	m map[string]*proto.SignedGossipMessage
	sync.RWMutex
}

// NewMembershipStore creates new membership store instance
func NewMembershipStore() *MembershipStore {
	////logger.Info("===NewMembershipStore==")
	return &MembershipStore{m: make(map[string]*proto.SignedGossipMessage)}
}

// MsgByID returns a message stored by a certain ID, or nil
// if such an ID isn't found
func (m *MembershipStore) MsgByID(pkiID common.PKIidType) *proto.SignedGossipMessage {
	////logger.Info("===MembershipStore==MsgByID==")
	m.RLock()
	defer m.RUnlock()
	if msg, exists := m.m[string(pkiID)]; exists {
		return msg
	}
	return nil
}

// Size of the membership store
func (m *MembershipStore) Size() int {
	////logger.Info("===MembershipStore==Size==")
	m.RLock()
	defer m.RUnlock()
	return len(m.m)
}

// Put associates msg with the given pkiID
func (m *MembershipStore) Put(pkiID common.PKIidType, msg *proto.SignedGossipMessage) {
	////logger.Info("===MembershipStore==Put==")
	m.Lock()
	defer m.Unlock()
	m.m[string(pkiID)] = msg
}

// Remove removes a message with a given pkiID
func (m *MembershipStore) Remove(pkiID common.PKIidType) {
	////logger.Info("===MembershipStore==Remove==")
	m.Lock()
	defer m.Unlock()
	delete(m.m, string(pkiID))
}

// ToSlice returns a slice backed by the elements
// of the MembershipStore
func (m *MembershipStore) ToSlice() []*proto.SignedGossipMessage {
	////logger.Info("===MembershipStore==ToSlice==")
	m.RLock()
	defer m.RUnlock()
	members := make([]*proto.SignedGossipMessage, len(m.m))
	i := 0
	for _, member := range m.m {
		////logger.Info("=====member====",member)
		members[i] = member
		i++
	}
	////logger.Info("=====members========",members)
	return members
}
