/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package api

import (
	"time"

	"github.com/hyperledger/fabric/gossip/common"
	"google.golang.org/grpc"
)

// MessageCryptoService is the contract between the gossip component and the
// peer's cryptographic layer and is used by the gossip component to verify,
// and authenticate remote peers and data they send, as well as to verify
// received blocks from the ordering service.
type MessageCryptoService interface {

	// GetPKIidOfCert returns the PKI-ID of a peer's identity
	// If any error occurs, the method return nil
	// This method does not validate peerIdentity.
	// This validation is supposed to be done appropriately during the execution flow.
	GetPKIidOfCert(peerIdentity PeerIdentityType) common.PKIidType

	// VerifyBlock returns nil if the block is properly signed, and the claimed seqNum is the
	// sequence number that the block's header contains.
	// else returns error
	VerifyBlock(chainID common.ChainID, seqNum uint64, signedBlock []byte) error

	// Sign signs msg with this peer's signing key and outputs
	// the signature if no error occurred.
	Sign(msg []byte) ([]byte, error)

	// Verify checks that signature is a valid signature of message under a peer's verification key.
	// If the verification succeeded, Verify returns nil meaning no error occurred.
	// If peerIdentity is nil, then the verification fails.
	Verify(peerIdentity PeerIdentityType, signature, message []byte) error

	// VerifyByChannel checks that signature is a valid signature of message
	// under a peer's verification key, but also in the context of a specific channel.
	// If the verification succeeded, Verify returns nil meaning no error occurred.
	// If peerIdentity is nil, then the verification fails.
	VerifyByChannel(chainID common.ChainID, peerIdentity PeerIdentityType, signature, message []byte) error

	// ValidateIdentity validates the identity of a remote peer.
	// If the identity is invalid, revoked, expired it returns an error.
	// Else, returns nil
	ValidateIdentity(peerIdentity PeerIdentityType) error

	// Expiration returns:
	// - The time when the identity expires, nil
	//   In case it can expire
	// - A zero value time.Time, nil
	//   in case it cannot expire
	// - A zero value, error in case it cannot be
	//   determined if the identity can expire or not
	Expiration(peerIdentity PeerIdentityType) (time.Time, error)
}

// PeerIdentityInfo aggregates a peer's identity,
// and also additional metadata about it
type PeerIdentityInfo struct {
	PKIId        common.PKIidType
	Identity     PeerIdentityType
	Organization OrgIdentityType
}

// PeerIdentitySet aggregates a PeerIdentityInfo slice
type PeerIdentitySet []PeerIdentityInfo

// ByOrg sorts the PeerIdentitySet by organizations of its peers
func (pis PeerIdentitySet) ByOrg() map[string]PeerIdentitySet {
	//logger.Info("======PeerIdentitySet=======ByOrg==============")
	m := make(map[string]PeerIdentitySet)
	for _, id := range pis {
		//logger.Info("=======id",id)
		/*
		 {500f76e570c22e91343ac9c28db55d6948faa44da710db6fa7971641d4ea285b [10 7 79 114 103 49 77 83 80 18 166 6 45 45 45 45 45 66 69 71 73 78 32 67 69 82 84 73 70 73 67 65 84 69 45 45 45 45 45 10 77 73 73 67 74 122 67 67 65 99 54 103 65 119 73 66 65 103 73 81 81 118 65 82 82 87 72 53 86 109 81 69 112 108 89 70 53 98 119 80 102 106 65 75 66 103 103 113 104 107 106 79 80 81 81 68 65 106 66 122 77 81 115 119 10 67 81 89 68 86 81 81 71 69 119 74 86 85 122 69 84 77 66 69 71 65 49 85 69 67 66 77 75 81 50 70 115 97 87 90 118 99 109 53 112 89 84 69 87 77 66 81 71 65 49 85 69 66 120 77 78 85 50 70 117 73 69 90 121 10 89 87 53 106 97 88 78 106 98 122 69 90 77 66 99 71 65 49 85 69 67 104 77 81 98 51 74 110 77 83 53 108 101 71 70 116 99 71 120 108 76 109 78 118 98 84 69 99 77 66 111 71 65 49 85 69 65 120 77 84 89 50 69 117 10 98 51 74 110 77 83 53 108 101 71 70 116 99 71 120 108 76 109 78 118 98 84 65 101 70 119 48 121 77 84 69 120 77 84 85 119 79 84 73 51 77 68 66 97 70 119 48 122 77 84 69 120 77 84 77 119 79 84 73 51 77 68 66 97 10 77 71 111 120 67 122 65 74 66 103 78 86 66 65 89 84 65 108 86 84 77 82 77 119 69 81 89 68 86 81 81 73 69 119 112 68 89 87 120 112 90 109 57 121 98 109 108 104 77 82 89 119 70 65 89 68 86 81 81 72 69 119 49 84 10 89 87 52 103 82 110 74 104 98 109 78 112 99 50 78 118 77 81 48 119 67 119 89 68 86 81 81 76 69 119 82 119 90 87 86 121 77 82 56 119 72 81 89 68 86 81 81 68 69 120 90 119 90 87 86 121 77 83 53 118 99 109 99 120 10 76 109 86 52 89 87 49 119 98 71 85 117 89 50 57 116 77 70 107 119 69 119 89 72 75 111 90 73 122 106 48 67 65 81 89 73 75 111 90 73 122 106 48 68 65 81 99 68 81 103 65 69 74 73 85 48 73 70 108 105 66 71 67 55 10 57 98 77 49 116 74 83 56 69 50 82 112 65 98 107 81 112 114 122 79 113 90 111 84 52 75 120 103 77 68 111 53 68 80 67 81 106 55 72 67 69 108 107 65 47 103 109 78 86 122 69 50 84 73 118 74 122 68 75 115 84 43 86 67 10 87 50 79 84 66 87 85 78 104 75 78 78 77 69 115 119 68 103 89 68 86 82 48 80 65 81 72 47 66 65 81 68 65 103 101 65 77 65 119 71 65 49 85 100 69 119 69 66 47 119 81 67 77 65 65 119 75 119 89 68 86 82 48 106 10 66 67 81 119 73 111 65 103 75 50 65 49 104 114 82 54 113 120 108 50 69 66 57 56 119 55 74 102 50 121 109 47 53 116 103 49 118 103 97 101 88 101 74 51 102 76 98 87 76 50 81 119 67 103 89 73 75 111 90 73 122 106 48 69 10 65 119 73 68 82 119 65 119 82 65 73 103 74 47 85 65 115 55 117 68 103 73 103 111 81 103 84 69 89 77 103 115 54 101 83 118 101 113 80 72 75 76 82 117 89 47 48 99 67 105 104 51 105 69 56 67 73 66 107 119 111 87 120 85 10 89 76 112 88 66 100 113 99 55 109 81 104 77 67 113 57 116 102 113 74 51 53 106 43 54 88 71 89 104 120 112 105 71 71 105 76 10 45 45 45 45 45 69 78 68 32 67 69 82 84 73 70 73 67 65 84 69 45 45 45 45 45 10] [79 114 103 49 77 83 80]}
		*/
		a := string(id.Organization)// Org1MSP
		//logger.Info("=======id.Organization=======",a)
		m[a] = append(m[a], id)
	}
	//logger.Info("====m===",m)
	return m
}

// ByOrg sorts the PeerIdentitySet by PKI-IDs of its peers
func (pis PeerIdentitySet) ByID() map[string]PeerIdentityInfo {
	//logger.Info("======PeerIdentitySet=======ByID==============")
	m := make(map[string]PeerIdentityInfo)
	for _, id := range pis {
		//logger.Info("==id===",id)//{7030 [10 1 65 18 2 112 48] [65]}  //{7031 [10 1 65 18 2 112 49] [65]}
		//logger.Info("===string(id.PKIId)======",string(id.PKIId))//p0 p1  p2  p3 p4  ...  p15
		m[string(id.PKIId)] = id
	}
	//logger.Info("======PeerIdentitySet==================",m)
	// map[p6:{7036 [10 1 68 18 2 112 54] [68]} p11:{703131 [10 1 66 18 3 112 49 49] [66]} p15:{703135 [10 1 68 18 3 112 49 53] [68]} p8:{7038 [10 1 65 18 2 112 56] [65]} p9:{7039 [10 1 65 18 2 112 57] [65]} p1:{7031 [10 1 65 18 2 112 49] [65]} p2:{7032 [10 1 66 18 2 112 50] [66]} p4:{7034 [10 1 67 18 2 112 52] [67]} p5:{7035 [10 1 67 18 2 112 53] [67]} p7:{7037 [10 1 68 18 2 112 55] [68]} p3:{7033 [10 1 66 18 2 112 51] [66]} p0:{7030 [10 1 65 18 2 112 48] [65]} p10:{703130 [10 1 66 18 3 112 49 48] [66]} p12:{703132 [10 1 67 18 3 112 49 50] [67]} p13:{703133 [10 1 67 18 3 112 49 51] [67]} p14:{703134 [10 1 68 18 3 112 49 52] [68]}]
	return m
}

// PeerIdentityType is the peer's certificate
type PeerIdentityType []byte

// PeerSuspector returns whether a peer with a given identity is suspected
// as being revoked, or its CA is revoked
type PeerSuspector func(identity PeerIdentityType) bool

// PeerSecureDialOpts returns the gRPC DialOptions to use for connection level
// security when communicating with remote peer endpoints
type PeerSecureDialOpts func() []grpc.DialOption

// PeerSignature defines a signature of a peer
// on a given message
type PeerSignature struct {
	Signature    []byte
	Message      []byte
	PeerIdentity PeerIdentityType
}
