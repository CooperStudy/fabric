/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package discovery

import (
	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/common/flogging"
)

// QueryType defines the types of service discovery requests
type QueryType uint8

const (
	InvalidQueryType QueryType = iota
	ConfigQueryType
	PeerMembershipQueryType
	ChaincodeQueryType
	LocalMembershipQueryType
)
var discoveryLogger = flogging.MustGetLogger("protos.discovery")
// GetType returns the type of the request
func (q *Query) GetType() QueryType {
	discoveryLogger.Info("===========func (q *Query) GetType() QueryType===========")
	if q.GetCcQuery() != nil {
		return ChaincodeQueryType
	}
	if q.GetConfigQuery() != nil {
		return ConfigQueryType
	}
	if q.GetPeerQuery() != nil {
		return PeerMembershipQueryType
	}
	if q.GetLocalPeers() != nil {
		return LocalMembershipQueryType
	}
	return InvalidQueryType
}

// ToRequest deserializes this SignedRequest's payload
// and returns the serialized Request in its object form.
// Returns an error in case the operation fails.
func (sr *SignedRequest) ToRequest() (*Request, error) {
	discoveryLogger.Info("==========SignedRequest===ToRequest===========")
	req := &Request{}
	return req, proto.Unmarshal(sr.Payload, req)
}

// ConfigAt returns the ConfigResult at a given index in the Response,
// or an Error if present.
func (m *Response) ConfigAt(i int) (*ConfigResult, *Error) {
	discoveryLogger.Info("==========Response===ConfigAt===========")
	r := m.Results[i]
	return r.GetConfigResult(), r.GetError()
}

// MembershipAt returns the PeerMembershipResult at a given index in the Response,
// or an Error if present.
func (m *Response) MembershipAt(i int) (*PeerMembershipResult, *Error) {
	discoveryLogger.Info("==========Response===MembershipAt===========")
	r := m.Results[i]
	return r.GetMembers(), r.GetError()
}

// EndorsersAt returns the PeerMembershipResult at a given index in the Response,
// or an Error if present.
func (m *Response) EndorsersAt(i int) (*ChaincodeQueryResult, *Error) {
	discoveryLogger.Info("==========Response===EndorsersAt===========")
	r := m.Results[i]
	return r.GetCcQueryRes(), r.GetError()
}
