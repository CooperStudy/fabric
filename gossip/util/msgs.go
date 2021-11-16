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
	//fmt.Println("===NewMembershipStore==")
	return &MembershipStore{m: make(map[string]*proto.SignedGossipMessage)}
}

// MsgByID returns a message stored by a certain ID, or nil
// if such an ID isn't found
func (m *MembershipStore) MsgByID(pkiID common.PKIidType) *proto.SignedGossipMessage {
	//fmt.Println("===MembershipStore==MsgByID==")
	m.RLock()
	defer m.RUnlock()
	if msg, exists := m.m[string(pkiID)]; exists {
		return msg
	}
	return nil
}

// Size of the membership store
func (m *MembershipStore) Size() int {
	//fmt.Println("===MembershipStore==Size==")
	m.RLock()
	defer m.RUnlock()
	return len(m.m)
}

// Put associates msg with the given pkiID
func (m *MembershipStore) Put(pkiID common.PKIidType, msg *proto.SignedGossipMessage) {
	//fmt.Println("===MembershipStore==Put==")
	m.Lock()
	defer m.Unlock()
	m.m[string(pkiID)] = msg
}

// Remove removes a message with a given pkiID
func (m *MembershipStore) Remove(pkiID common.PKIidType) {
	//fmt.Println("===MembershipStore==Remove==")
	m.Lock()
	defer m.Unlock()
	delete(m.m, string(pkiID))
}

// ToSlice returns a slice backed by the elements
// of the MembershipStore
func (m *MembershipStore) ToSlice() []*proto.SignedGossipMessage {
	//fmt.Println("===MembershipStore==ToSlice==")
	m.RLock()
	defer m.RUnlock()
	members := make([]*proto.SignedGossipMessage, len(m.m))
	i := 0
	for _, member := range m.m {
		//fmt.Println("=====member====",member)
		members[i] = member
		i++
	}
	//fmt.Println("=====members========",members)
	return members
}
