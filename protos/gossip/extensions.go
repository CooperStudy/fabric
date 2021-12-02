/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package gossip

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/common/util"
	"github.com/hyperledger/fabric/gossip/api"
	"github.com/hyperledger/fabric/gossip/common"
)

// NewGossipMessageComparator creates a MessageReplacingPolicy given a maximum number of blocks to hold
func NewGossipMessageComparator(dataBlockStorageSize int) common.MessageReplacingPolicy {
	//logger.Info("==========NewGossipMessageComparator==========")
	return (&msgComparator{dataBlockStorageSize: dataBlockStorageSize}).getMsgReplacingPolicy()
}

type msgComparator struct {
	dataBlockStorageSize int
}

func (mc *msgComparator) getMsgReplacingPolicy() common.MessageReplacingPolicy {
//	logger.Info("==========msgComparator===getMsgReplacingPolicy=======")
	return func(this interface{}, that interface{}) common.InvalidationResult {
		return mc.invalidationPolicy(this, that)
	}
}

func (mc *msgComparator) invalidationPolicy(this interface{}, that interface{}) common.InvalidationResult {
	//logger.Info("==========msgComparator===invalidationPolicy=======")
	thisMsg := this.(*SignedGossipMessage)
	thatMsg := that.(*SignedGossipMessage)

	if thisMsg.IsAliveMsg() && thatMsg.IsAliveMsg() {
		return aliveInvalidationPolicy(thisMsg.GetAliveMsg(), thatMsg.GetAliveMsg())
	}

	if thisMsg.IsDataMsg() && thatMsg.IsDataMsg() {
		return mc.dataInvalidationPolicy(thisMsg.GetDataMsg(), thatMsg.GetDataMsg())
	}

	if thisMsg.IsStateInfoMsg() && thatMsg.IsStateInfoMsg() {
		return mc.stateInvalidationPolicy(thisMsg.GetStateInfo(), thatMsg.GetStateInfo())
	}

	if thisMsg.IsIdentityMsg() && thatMsg.IsIdentityMsg() {
		return mc.identityInvalidationPolicy(thisMsg.GetPeerIdentity(), thatMsg.GetPeerIdentity())
	}

	if thisMsg.IsLeadershipMsg() && thatMsg.IsLeadershipMsg() {
		return leaderInvalidationPolicy(thisMsg.GetLeadershipMsg(), thatMsg.GetLeadershipMsg())
	}

	return common.MessageNoAction
}

func (mc *msgComparator) stateInvalidationPolicy(thisStateMsg *StateInfo, thatStateMsg *StateInfo) common.InvalidationResult {
	//logger.Info("==========msgComparator===stateInvalidationPolicy=======")
	if !bytes.Equal(thisStateMsg.PkiId, thatStateMsg.PkiId) {
		return common.MessageNoAction
	}
	return compareTimestamps(thisStateMsg.Timestamp, thatStateMsg.Timestamp)
}

func (mc *msgComparator) identityInvalidationPolicy(thisIdentityMsg *PeerIdentity, thatIdentityMsg *PeerIdentity) common.InvalidationResult {
	//logger.Info("==========msgComparator===identityInvalidationPolicy=======")
	if bytes.Equal(thisIdentityMsg.PkiId, thatIdentityMsg.PkiId) {
		return common.MessageInvalidated
	}

	return common.MessageNoAction
}

func (mc *msgComparator) dataInvalidationPolicy(thisDataMsg *DataMessage, thatDataMsg *DataMessage) common.InvalidationResult {
	//logger.Info("==========msgComparator===dataInvalidationPolicy=======")
	if thisDataMsg.Payload.SeqNum == thatDataMsg.Payload.SeqNum {
		return common.MessageInvalidated
	}

	diff := abs(thisDataMsg.Payload.SeqNum, thatDataMsg.Payload.SeqNum)
	if diff <= uint64(mc.dataBlockStorageSize) {
		return common.MessageNoAction
	}

	if thisDataMsg.Payload.SeqNum > thatDataMsg.Payload.SeqNum {
		return common.MessageInvalidates
	}
	return common.MessageInvalidated
}

func aliveInvalidationPolicy(thisMsg *AliveMessage, thatMsg *AliveMessage) common.InvalidationResult {
	//logger.Info("==========aliveInvalidationPolicy======")
	if !bytes.Equal(thisMsg.Membership.PkiId, thatMsg.Membership.PkiId) {
		return common.MessageNoAction
	}

	return compareTimestamps(thisMsg.Timestamp, thatMsg.Timestamp)
}

func leaderInvalidationPolicy(thisMsg *LeadershipMessage, thatMsg *LeadershipMessage) common.InvalidationResult {
	//logger.Info("==========leaderInvalidationPolicy======")
	if !bytes.Equal(thisMsg.PkiId, thatMsg.PkiId) {
		return common.MessageNoAction
	}

	return compareTimestamps(thisMsg.Timestamp, thatMsg.Timestamp)
}

func compareTimestamps(thisTS *PeerTime, thatTS *PeerTime) common.InvalidationResult {
	//logger.Info("==========compareTimestamps======")
	if thisTS.IncNum == thatTS.IncNum {
		if thisTS.SeqNum > thatTS.SeqNum {
			return common.MessageInvalidates
		}

		return common.MessageInvalidated
	}
	if thisTS.IncNum < thatTS.IncNum {
		return common.MessageInvalidated
	}
	return common.MessageInvalidates
}

// IsAliveMsg returns whether this GossipMessage is an AliveMessage
func (m *GossipMessage) IsAliveMsg() bool {
	//logger.Info("==========GossipMessage==IsAliveMsg====")
	return m.GetAliveMsg() != nil
}

// IsDataMsg returns whether this GossipMessage is a data message
func (m *GossipMessage) IsDataMsg() bool {
//	logger.Info("==========GossipMessage==IsDataMsg====")
	//logger.Info("====m.GetDataMsg()======",m.GetDataMsg())
	return m.GetDataMsg() != nil
}

// IsStateInfoPullRequestMsg returns whether this GossipMessage is a stateInfoPullRequest
func (m *GossipMessage) IsStateInfoPullRequestMsg() bool {
	//logger.Info("==========GossipMessage==IsStateInfoPullRequestMsg====")
	return m.GetStateInfoPullReq() != nil
}

// IsStateInfoSnapshot returns whether this GossipMessage is a stateInfo snapshot
func (m *GossipMessage) IsStateInfoSnapshot() bool {
	//logger.Info("==========GossipMessage==IsStateInfoSnapshot====")
	return m.GetStateSnapshot() != nil
}

// IsStateInfoMsg returns whether this GossipMessage is a stateInfo message
func (m *GossipMessage) IsStateInfoMsg() bool {
	//logger.Info("==========GossipMessage==IsStateInfoMsg====")
	return m.GetStateInfo() != nil
}

// IsPullMsg returns whether this GossipMessage is a message that belongs
// to the pull mechanism
func (m *GossipMessage) IsPullMsg() bool {
	//logger.Info("==========GossipMessage==IsPullMsg====")
	return m.GetDataReq() != nil || m.GetDataUpdate() != nil ||
		m.GetHello() != nil || m.GetDataDig() != nil
}

// IsRemoteStateMessage returns whether this GossipMessage is related to state synchronization
func (m *GossipMessage) IsRemoteStateMessage() bool {
	//logger.Info("==========GossipMessage==IsRemoteStateMessage====")
	return m.GetStateRequest() != nil || m.GetStateResponse() != nil
}

// GetPullMsgType returns the phase of the pull mechanism this GossipMessage belongs to
// for example: Hello, Digest, etc.
// If this isn't a pull message, PullMsgType_UNDEFINED is returned.
func (m *GossipMessage) GetPullMsgType() PullMsgType {
	//logger.Info("==========GossipMessage==GetPullMsgType====")
	if helloMsg := m.GetHello(); helloMsg != nil {
		return helloMsg.MsgType
	}

	if digMsg := m.GetDataDig(); digMsg != nil {
		return digMsg.MsgType
	}

	if reqMsg := m.GetDataReq(); reqMsg != nil {
		return reqMsg.MsgType
	}

	if resMsg := m.GetDataUpdate(); resMsg != nil {
		return resMsg.MsgType
	}

	return PullMsgType_UNDEFINED
}

// IsChannelRestricted returns whether this GossipMessage should be routed
// only in its channel
func (m *GossipMessage) IsChannelRestricted() bool {
	//logger.Info("==========GossipMessage==IsChannelRestricted====")
	return m.Tag == GossipMessage_CHAN_AND_ORG || m.Tag == GossipMessage_CHAN_ONLY || m.Tag == GossipMessage_CHAN_OR_ORG
}

// IsOrgRestricted returns whether this GossipMessage should be routed only
// inside the organization
func (m *GossipMessage) IsOrgRestricted() bool {
	//logger.Info("==========GossipMessage==IsOrgRestricted====")
	return m.Tag == GossipMessage_CHAN_AND_ORG || m.Tag == GossipMessage_ORG_ONLY
}

// IsIdentityMsg returns whether this GossipMessage is an identity message
func (m *GossipMessage) IsIdentityMsg() bool {
	//logger.Info("==========GossipMessage==IsIdentityMsg====")
	return m.GetPeerIdentity() != nil
}

// IsDataReq returns whether this GossipMessage is a data request message
func (m *GossipMessage) IsDataReq() bool {
	//logger.Info("==========GossipMessage==IsDataReq====")
	return m.GetDataReq() != nil
}

// IsPrivateDataMsg returns whether this message is related to private data
func (m *GossipMessage) IsPrivateDataMsg() bool {
	//logger.Info("==========GossipMessage==IsPrivateDataMsg====")
	//logger.Info("============m.GetPrivateReq()============",m.GetPrivateReq())
	//logger.Info("============m.GetPrivateRes()============",m.GetPrivateRes())
	//logger.Info("============m.GetPrivateData()============",m.GetPrivateData())
	return m.GetPrivateReq() != nil || m.GetPrivateRes() != nil || m.GetPrivateData() != nil
}

// IsAck returns whether this GossipMessage is an acknowledgement
func (m *GossipMessage) IsAck() bool {
	//logger.Info("==========GossipMessage==IsAck====")
//	logger.Info("=========m.GetAck() != nil============",m.GetAck() != nil)
	return m.GetAck() != nil
}

// IsDataUpdate returns whether this GossipMessage is a data update message
func (m *GossipMessage) IsDataUpdate() bool {
	//logger.Info("==========GossipMessage==IsDataUpdate====")
	return m.GetDataUpdate() != nil
}

// IsHelloMsg returns whether this GossipMessage is a hello message
func (m *GossipMessage) IsHelloMsg() bool {
	//logger.Info("==========GossipMessage==IsHelloMsg====")
	return m.GetHello() != nil
}

// IsDigestMsg returns whether this GossipMessage is a digest message
func (m *GossipMessage) IsDigestMsg() bool {
	//logger.Info("==========GossipMessage==IsDigestMsg====")
	return m.GetDataDig() != nil
}

// IsLeadershipMsg returns whether this GossipMessage is a leadership (leader election) message
func (m *GossipMessage) IsLeadershipMsg() bool {
	//logger.Info("==========GossipMessage==IsLeadershipMsg====")
	return m.GetLeadershipMsg() != nil
}

// MsgConsumer invokes code given a SignedGossipMessage
type MsgConsumer func(message *SignedGossipMessage)

// IdentifierExtractor extracts from a SignedGossipMessage an identifier
type IdentifierExtractor func(*SignedGossipMessage) string

// IsTagLegal checks the GossipMessage tags and inner type
// and returns an error if the tag doesn't match the type.
func (m *GossipMessage) IsTagLegal() error {
	//logger.Info("==========GossipMessage==IsTagLegal====")
	if m.Tag == GossipMessage_UNDEFINED {
		//logger.Info("========= m.Tag == GossipMessage_UNDEFINED ========")
		return fmt.Errorf("Undefined tag")
	}
	if m.IsDataMsg() {
	//	logger.Info("=========m.IsDataMsg()========",m.IsDataMsg())
		if m.Tag != GossipMessage_CHAN_AND_ORG {
		//	logger.Info("=========  m.Tag != GossipMessage_CHAN_AND_ORG  ========")
			return fmt.Errorf("Tag should be %s", GossipMessage_Tag_name[int32(GossipMessage_CHAN_AND_ORG)])
		}
		return nil
	}

	if m.IsAliveMsg() || m.GetMemReq() != nil || m.GetMemRes() != nil {
		//logger.Info("=========m.IsAliveMsg() || m.GetMemReq() != nil || m.GetMemRes() != nil ========")
		if m.Tag != GossipMessage_EMPTY {
			return fmt.Errorf("Tag should be %s", GossipMessage_Tag_name[int32(GossipMessage_EMPTY)])
		}
		return nil
	}

	if m.IsIdentityMsg() {
		//logger.Info("========m.IsIdentityMsg=========")
		if m.Tag != GossipMessage_ORG_ONLY {
			return fmt.Errorf("Tag should be %s", GossipMessage_Tag_name[int32(GossipMessage_ORG_ONLY)])
		}
		return nil
	}

	if m.IsPullMsg() {
		//logger.Info("====== m.IsPullMsg========")
		switch m.GetPullMsgType() {
		case PullMsgType_BLOCK_MSG:
			if m.Tag != GossipMessage_CHAN_AND_ORG {
				return fmt.Errorf("Tag should be %s", GossipMessage_Tag_name[int32(GossipMessage_CHAN_AND_ORG)])
			}
			return nil
		case PullMsgType_IDENTITY_MSG:
			if m.Tag != GossipMessage_EMPTY {
				return fmt.Errorf("Tag should be %s", GossipMessage_Tag_name[int32(GossipMessage_EMPTY)])
			}
			return nil
		default:
			return fmt.Errorf("Invalid PullMsgType: %s", PullMsgType_name[int32(m.GetPullMsgType())])
		}
	}

	if m.IsStateInfoMsg() || m.IsStateInfoPullRequestMsg() || m.IsStateInfoSnapshot() || m.IsRemoteStateMessage() {
		//logger.Info("====== m.IsStateInfoMsg() || m.IsStateInfoPullRequestMsg() || m.IsStateInfoSnapshot() || m.IsRemoteStateMessage() ========")
		if m.Tag != GossipMessage_CHAN_OR_ORG {
			return fmt.Errorf("Tag should be %s", GossipMessage_Tag_name[int32(GossipMessage_CHAN_OR_ORG)])
		}
		return nil
	}

	if m.IsLeadershipMsg() {
		//logger.Info("====== m.IsLeadershipMsg ========")
		if m.Tag != GossipMessage_CHAN_AND_ORG {
			return fmt.Errorf("Tag should be %s", GossipMessage_Tag_name[int32(GossipMessage_CHAN_AND_ORG)])
		}
		return nil
	}

	return fmt.Errorf("Unknown message type: %v", m)
}

// Verifier receives a peer identity, a signature and a message
// and returns nil if the signature on the message could be verified
// using the given identity.
type Verifier func(peerIdentity []byte, signature, message []byte) error

// Signer signs a message, and returns (signature, nil)
// on success, and nil and an error on failure.
type Signer func(msg []byte) ([]byte, error)

// ReceivedMessage is a GossipMessage wrapper that
// enables the user to send a message to the origin from which
// the ReceivedMessage was sent from.
// It also allows to know the identity of the sender,
// to obtain the raw bytes the GossipMessage was un-marshaled from,
// and the signature over these raw bytes.
type ReceivedMessage interface {

	// Respond sends a GossipMessage to the origin from which this ReceivedMessage was sent from
	Respond(msg *GossipMessage)

	// GetGossipMessage returns the underlying GossipMessage
	GetGossipMessage() *SignedGossipMessage

	// GetSourceMessage Returns the Envelope the ReceivedMessage was
	// constructed with
	GetSourceEnvelope() *Envelope

	// GetConnectionInfo returns information about the remote peer
	// that sent the message
	GetConnectionInfo() *ConnectionInfo

	// Ack returns to the sender an acknowledgement for the message
	// An ack can receive an error that indicates that the operation related
	// to the message has failed
	Ack(err error)
}

// ConnectionInfo represents information about
// the remote peer that sent a certain ReceivedMessage
type ConnectionInfo struct {
	ID       common.PKIidType
	Auth     *AuthInfo
	Identity api.PeerIdentityType
	Endpoint string
}

// String returns a string representation of this ConnectionInfo
func (c *ConnectionInfo) String() string {
	//logger.Info("==========ConnectionInfo==String====")
	return fmt.Sprintf("%s %v", c.Endpoint, c.ID)
}

// AuthInfo represents the authentication
// data that was provided by the remote peer
// at the connection time
type AuthInfo struct {
	SignedData []byte
	Signature  []byte
}

// Sign signs a GossipMessage with given Signer.
// Returns an Envelope on success,
// panics on failure.
func (m *SignedGossipMessage) Sign(signer Signer) (*Envelope, error) {
	//logger.Info("==========SignedGossipMessage==Sign====")
	// If we have a secretEnvelope, don't override it.

	// Back it up, and restore it later
	var secretEnvelope *SecretEnvelope
//	logger.Info("======= m.Envelope==========", m.Envelope)
	if m.Envelope != nil {
	//	logger.Info("=======secretEnvelope==========", secretEnvelope)
		secretEnvelope = m.Envelope.SecretEnvelope
	}
	m.Envelope = nil
	payload, err := proto.Marshal(m.GossipMessage)
	//logger.Info("======payload=======",payload)
	if err != nil {
		return nil, err
	}
	sig, err := signer(payload)
	if err != nil {
		return nil, err
	}
	//logger.Info("=====sig========",sig)

	e := &Envelope{
		Payload:        payload,
		Signature:      sig,
		SecretEnvelope: secretEnvelope,
	}
	//logger.Info("============Payload==============",payload)
	//logger.Info("============Signature==============",sig)
	//logger.Info("============SecretEnvelope==============",secretEnvelope)
	m.Envelope = e
	return e, nil
}

// NoopSign creates a SignedGossipMessage with a nil signature
func (m *GossipMessage) NoopSign() (*SignedGossipMessage, error) {
	//logger.Info("==========GossipMessage==NoopSign====")
	signer := func(msg []byte) ([]byte, error) {
		//logger.Info("========NoopSign======signer=========")
		//logger.Info("======msg=======",msg)
		//logger.Info("======return nil nil==================")
		return nil, nil
	}
	sMsg := &SignedGossipMessage{
		GossipMessage: m,
	}
	_, err := sMsg.Sign(signer)
	return sMsg, err
}

// Verify verifies a signed GossipMessage with a given Verifier.
// Returns nil on success, error on failure.
func (m *SignedGossipMessage) Verify(peerIdentity []byte, verify Verifier) error {
	//logger.Info("==========SignedGossipMessage==Verify====")
	if m.Envelope == nil {
		return errors.New("Missing envelope")
	}
	if len(m.Envelope.Payload) == 0 {
		return errors.New("Empty payload")
	}
	if len(m.Envelope.Signature) == 0 {
		return errors.New("Empty signature")
	}
	payloadSigVerificationErr := verify(peerIdentity, m.Envelope.Signature, m.Envelope.Payload)
	if payloadSigVerificationErr != nil {
		return payloadSigVerificationErr
	}
	if m.Envelope.SecretEnvelope != nil {
		payload := m.Envelope.SecretEnvelope.Payload
		sig := m.Envelope.SecretEnvelope.Signature
		if len(payload) == 0 {
			return errors.New("Empty payload")
		}
		if len(sig) == 0 {
			return errors.New("Empty signature")
		}
		return verify(peerIdentity, sig, payload)
	}
	return nil
}

// IsSigned returns whether the message
// has a signature in the envelope.
func (m *SignedGossipMessage) IsSigned() bool {
	//logger.Info("==========SignedGossipMessage==IsSigned====")
	return m.Envelope != nil && m.Envelope.Payload != nil && m.Envelope.Signature != nil
}

// ToGossipMessage un-marshals a given envelope and creates a
// SignedGossipMessage out of it.
// Returns an error if un-marshaling fails.
func (e *Envelope) ToGossipMessage() (*SignedGossipMessage, error) {
	//logger.Info("==========Envelope==ToGossipMessage====")
	if e == nil {
		return nil, errors.New("nil envelope")
	}
	msg := &GossipMessage{}
	err := proto.Unmarshal(e.Payload, msg)
	if err != nil {
		return nil, fmt.Errorf("Failed unmarshaling GossipMessage from envelope: %v", err)
	}
	return &SignedGossipMessage{
		GossipMessage: msg,
		Envelope:      e,
	}, nil
}

// SignSecret signs the secret payload and creates
// a secret envelope out of it.
func (e *Envelope) SignSecret(signer Signer, secret *Secret) error {
	//logger.Info("==========Envelope==SignSecret====")
	payload, err := proto.Marshal(secret)
	if err != nil {
		return err
	}
	//logger.Info("======payload==============",payload)
	sig, err := signer(payload)
	if err != nil {
		return err
	}
	//logger.Info("======sig==============",sig)
	e.SecretEnvelope = &SecretEnvelope{
		Payload:   payload,
		Signature: sig,
	}
	return nil
}

// InternalEndpoint returns the internal endpoint
// in the secret envelope, or an empty string
// if a failure occurs.
func (s *SecretEnvelope) InternalEndpoint() string {
	//logger.Info("==========SecretEnvelope==InternalEndpoint====")
	secret := &Secret{}
	if err := proto.Unmarshal(s.Payload, secret); err != nil {
		return ""
	}
	return secret.GetInternalEndpoint()
}

// SignedGossipMessage contains a GossipMessage
// and the Envelope from which it came from
type SignedGossipMessage struct {
	*Envelope
	*GossipMessage
}

func (p *Payload) toString() string {
//	logger.Info("==========Payload==toString====")
	return fmt.Sprintf("Block message: {Data: %d bytes, seq: %d}", len(p.Data), p.SeqNum)
}

func (du *DataUpdate) toString() string {
	//logger.Info("==========DataUpdate==toString====")
	mType := PullMsgType_name[int32(du.MsgType)]
	return fmt.Sprintf("Type: %s, items: %d, nonce: %d", mType, len(du.Data), du.Nonce)
}

func (mr *MembershipResponse) toString() string {
	//logger.Info("==========MembershipResponse==toString====")
	return fmt.Sprintf("MembershipResponse with Alive: %d, Dead: %d", len(mr.Alive), len(mr.Dead))
}

func (sis *StateInfoSnapshot) toString() string {
	//logger.Info("==========StateInfoSnapshot==toString====")
	return fmt.Sprintf("StateInfoSnapshot with %d items", len(sis.Elements))
}

// String returns a string representation
// of a SignedGossipMessage
func (m *SignedGossipMessage) String() string {
	//logger.Info("==========SignedGossipMessage==String====")
	env := "No envelope"
	if m.Envelope != nil {
		//logger.Info("========m.Envelope != nil========")
		var secretEnv string
		//logger.Info("=======m.SecretEnvelope ========",m.SecretEnvelope)
		if m.SecretEnvelope != nil {
			pl := len(m.SecretEnvelope.Payload)
			sl := len(m.SecretEnvelope.Signature)
			secretEnv = fmt.Sprintf(" Secret payload: %d bytes, Secret Signature: %d bytes", pl, sl)
			//logger.Info("=================pl",pl)
			//logger.Info("=================sl",sl)
			//logger.Info("=================secretEnv",secretEnv)
		}
		env = fmt.Sprintf("%d bytes, Signature: %d bytes%s", len(m.Envelope.Payload), len(m.Envelope.Signature), secretEnv)
		//logger.Info("============env===============",env)
	}
	gMsg := "No gossipMessage"
	if m.GossipMessage != nil {
		var isSimpleMsg bool
		if m.GetStateResponse() != nil {
			//logger.Info("=========m.GetStateResponse() != nil====================")
			gMsg = fmt.Sprintf("StateResponse with %d items", len(m.GetStateResponse().Payloads))
		} else if m.IsDataMsg() && m.GetDataMsg().Payload != nil {
			gMsg = m.GetDataMsg().Payload.toString()
			//logger.Info("========else if m.IsDataMsg() && m.GetDataMsg().Payload != nil===================")
		} else if m.IsDataUpdate() {
			//logger.Info("========m.IsDataUpdate()===================")
			update := m.GetDataUpdate()
			gMsg = fmt.Sprintf("DataUpdate: %s", update.toString())
		} else if m.GetMemRes() != nil {
			//logger.Info("========m.GetMemRes() != nil==================")
			gMsg = m.GetMemRes().toString()
		} else if m.IsStateInfoSnapshot() {
			//logger.Info("========m.IsStateInfoSnapshot()==================")
			gMsg = m.GetStateSnapshot().toString()
		} else if m.GetPrivateRes() != nil {
			//logger.Info("========m.GetPrivateRes() != nil=================")
			gMsg = m.GetPrivateRes().ToString()
		} else {
			//logger.Info("========else=================")
			gMsg = m.GossipMessage.String()
			isSimpleMsg = true
		}
		if !isSimpleMsg {
			desc := fmt.Sprintf("Channel: %s, nonce: %d, tag: %s", string(m.Channel), m.Nonce, GossipMessage_Tag_name[int32(m.Tag)])
			gMsg = fmt.Sprintf("%s %s", desc, gMsg)
			//logger.Info("===========desc========",desc)
		}
	}
	//logger.Info("=========gMsg============",gMsg)
	//logger.Info("=========env============",env)
	return fmt.Sprintf("GossipMessage: %v, Envelope: %s", gMsg, env)
}

func (dd *DataRequest) FormattedDigests() []string {
	//logger.Info("==========DataRequest==FormattedDigests====")
	if dd.MsgType == PullMsgType_IDENTITY_MSG {
		return digestsToHex(dd.Digests)
	}

	return digestsAsStrings(dd.Digests)
}

func (dd *DataDigest) FormattedDigests() []string {
	//logger.Info("==========DataDigest==FormattedDigests====")
	if dd.MsgType == PullMsgType_IDENTITY_MSG {
		return digestsToHex(dd.Digests)
	}
	return digestsAsStrings(dd.Digests)
}

// Hash returns the SHA256 representation of the PvtDataDigest's bytes
func (dig *PvtDataDigest) Hash() (string, error) {
	logger.Info("==========PvtDataDigest==Hash====")
	b, err := proto.Marshal(dig)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(util.ComputeSHA256(b)), nil
}

// ToString returns a string representation of this RemotePvtDataResponse
func (res *RemotePvtDataResponse) ToString() string {
	//logger.Info("==========RemotePvtDataResponse==ToString====")
	a := make([]string, len(res.Elements))
	for i, el := range res.Elements {
		a[i] = fmt.Sprintf("%s with %d elements", el.Digest.String(), len(el.Payload))
	}
	return fmt.Sprintf("%v", a)
}

func digestsAsStrings(digests [][]byte) []string {
	//logger.Info("==========digestsAsStrings====")
	a := make([]string, len(digests))
	for i, dig := range digests {
		a[i] = string(dig)
	}
	return a
}

func digestsToHex(digests [][]byte) []string {
	//logger.Info("==========digestsToHex====")
	a := make([]string, len(digests))
	for i, dig := range digests {
		a[i] = hex.EncodeToString(dig)
	}
	return a
}

// LedgerHeight returns the ledger height that is specified
// in the StateInfo message
func (msg *StateInfo) LedgerHeight() (uint64, error) {
	//logger.Info("==========StateInfo==LedgerHeight==")
	if msg.Properties != nil {
		return msg.Properties.LedgerHeight, nil
	}
	return 0, errors.New("properties undefined")
}

// Abs returns abs(a-b)
func abs(a, b uint64) uint64 {
	//logger.Info("========abs==")
	if a > b {
		return a - b
	}
	return b - a
}
