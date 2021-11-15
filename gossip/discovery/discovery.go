/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package discovery

import (
	"fmt"

	"github.com/hyperledger/fabric/gossip/common"
	proto "github.com/hyperledger/fabric/protos/gossip"
)

// CryptoService is an interface that the discovery expects to be implemented and passed on creation
type CryptoService interface {
	// ValidateAliveMsg validates that an Alive message is authentic
	ValidateAliveMsg(message *proto.SignedGossipMessage) bool

	// SignMessage signs a message
	SignMessage(m *proto.GossipMessage, internalEndpoint string) *proto.Envelope
}

// EnvelopeFilter may or may not remove part of the Envelope
// that the given SignedGossipMessage originates from.
type EnvelopeFilter func(message *proto.SignedGossipMessage) *proto.Envelope

// Sieve defines the messages that are allowed to be sent to some remote peer,
// based on some criteria.
// Returns whether the sieve permits sending a given message.
type Sieve func(message *proto.SignedGossipMessage) bool

// DisclosurePolicy defines which messages a given remote peer
// is eligible of knowing about, and also what is it eligible
// to know about out of a given SignedGossipMessage.
// Returns:
// 1) A Sieve for a given remote peer.
//    The Sieve is applied for each peer in question and outputs
//    whether the message should be disclosed to the remote peer.
// 2) A EnvelopeFilter for a given SignedGossipMessage, which may remove
//    part of the Envelope the SignedGossipMessage originates from
type DisclosurePolicy func(remotePeer *NetworkMember) (Sieve, EnvelopeFilter)

// CommService is an interface that the discovery expects to be implemented and passed on creation
type CommService interface {
	// Gossip gossips a message
	Gossip(msg *proto.SignedGossipMessage)

	// SendToPeer sends to a given peer a message.
	// The nonce can be anything since the communication module handles the nonce itself
	SendToPeer(peer *NetworkMember, msg *proto.SignedGossipMessage)

	// Ping probes a remote peer and returns if it's responsive or not
	Ping(peer *NetworkMember) bool

	// Accept returns a read-only channel for membership messages sent from remote peers
	Accept() <-chan proto.ReceivedMessage

	// PresumedDead returns a read-only channel for peers that are presumed to be dead
	PresumedDead() <-chan common.PKIidType

	// CloseConn orders to close the connection with a certain peer
	CloseConn(peer *NetworkMember)

	// Forward sends message to the next hop, excluding the hop
	// from which message was initially received
	Forward(msg proto.ReceivedMessage)
}

// NetworkMember is a peer's representation 表示
type NetworkMember struct {
	Endpoint         string
	Metadata         []byte
	PKIid            common.PKIidType
	InternalEndpoint string
	Properties       *proto.Properties
	*proto.Envelope
}

// String returns a string representation of the NetworkMember
func (n NetworkMember) String() string {
	fmt.Println("=====NetworkMember========String====")
	return fmt.Sprintf("Endpoint: %s, InternalEndpoint: %s, PKI-ID: %s, Metadata: %x", n.Endpoint, n.InternalEndpoint, n.PKIid, n.Metadata)
}

// PreferredEndpoint computes the endpoint to connect to,
// while preferring internal endpoint over the standard
// endpoint
func (n NetworkMember) PreferredEndpoint() string {
	fmt.Println("=======NetworkMember========PreferredEndpoint==")
	fmt.Println("===n====",n)
	fmt.Println("===n.InternalEndpoint====",n.InternalEndpoint)
	if n.InternalEndpoint != "" {
		return n.InternalEndpoint
	}
	fmt.Println("===n.Endpoint====",n.Endpoint)
	return n.Endpoint
}

// PeerIdentification encompasses a remote peer's
// PKI-ID and whether its in the same org as the current
// peer or not
type PeerIdentification struct {
	ID      common.PKIidType
	SelfOrg bool
}

type identifier func() (*PeerIdentification, error)

// Discovery is the interface that represents a discovery module
type Discovery interface {
	// Lookup returns a network member, or nil if not found
	Lookup(PKIID common.PKIidType) *NetworkMember

	// Self returns this instance's membership information
	Self() NetworkMember

	// UpdateMetadata updates this instance's metadata
	UpdateMetadata([]byte)

	// UpdateEndpoint updates this instance's endpoint
	UpdateEndpoint(string)

	// Stops this instance
	Stop()

	// GetMembership returns the alive members in the view
	GetMembership() []NetworkMember

	// InitiateSync makes the instance ask a given number of peers
	// for their membership information
	InitiateSync(peerNum int)

	// Connect makes this instance to connect to a remote instance
	// The identifier param is a function that can be used to identify
	// the peer, and to assert its PKI-ID, whether its in the peer's org or not,
	// and whether the action was successful or not
	Connect(member NetworkMember, id identifier)
}

// Members represents an aggregation of NetworkMembers
type Members []NetworkMember

// ByID returns a mapping from the PKI-IDs (in string form)
// to NetworkMember
func (members Members) ByID() map[string]NetworkMember {
	fmt.Println("=====Members===ByID===========")
	res := make(map[string]NetworkMember, len(members))
	for _, peer := range members {
		fmt.Println("====peer",peer)
		 a:= string(peer.PKIid)
		 fmt.Println("====peer.PKIid=======",peer.PKIid)
		res[a] = peer
	}
	fmt.Println("=====res====",res)
	return res
}

// Intersect returns the intersection of 2 Members
func (members Members) Intersect(otherMembers Members) Members {
	fmt.Println("=====Members===Intersect===========")
	var res Members
	m := otherMembers.ByID()
	fmt.Println("========m===========",m)
	for _, member := range members {
		fmt.Println("========member=====",member)
		fmt.Println("========string(member.PKIid)=============",string(member.PKIid))
		fmt.Println("========m[string(member.PKIid)]============",m[string(member.PKIid)])
		if _, exists := m[string(member.PKIid)]; exists {
			fmt.Println("=========exists========",exists)
			res = append(res, member)
		}
	}
	fmt.Println("=========res========",res)
	return res
}

// Filter returns only members that satisfy the given filter
func (members Members) Filter(filter func(member NetworkMember) bool) Members {
	fmt.Println("=====Members===Filter===========")
	var res Members
	for _, member := range members {
		if filter(member) {
			res = append(res, member)
		}
	}
	return res
}

// Map invokes the given function to every NetworkMember among the Members
func (members Members) Map(f func(member NetworkMember) NetworkMember) Members {
	fmt.Println("=====Members===Map===========")
	var res Members
	for _, m := range members {
		res = append(res, f(m))
	}
	return res
}

// HaveExternalEndpoints selects network members that have external endpoints
func HasExternalEndpoint(member NetworkMember) bool {
	fmt.Println("====HasExternalEndpoint==========")
	return member.Endpoint != ""
}
