/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package crypto

import (
	"github.com/golang/protobuf/proto"
	cb "github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric/protos/msp"
)

// LocalSigner is a temporary stub interface which will be implemented by the local MSP
type LocalSigner interface {
	SignatureHeaderMaker
	Signer
}

// Signer signs messages
type Signer interface {
	// Sign a message and return the signature over the digest, or error on failure
	Sign(message []byte) ([]byte, error)
}

// IdentitySerializer serializes identities
type IdentitySerializer interface {
	// Serialize converts an identity to bytes
	Serialize() ([]byte, error)
}

// SignatureHeaderMaker creates a new SignatureHeader
type SignatureHeaderMaker interface {
	// NewSignatureHeader creates a SignatureHeader with the correct signing identity and a valid nonce
	NewSignatureHeader() (*cb.SignatureHeader, error)
}

// SignatureHeaderCreator creates signature headers
type SignatureHeaderCreator struct {
	SignerSupport
}

// SignerSupport implements the needed support for LocalSigner
type SignerSupport interface {
	Signer
	IdentitySerializer
}

// NewSignatureHeaderCreator creates new signature headers
func NewSignatureHeaderCreator(ss SignerSupport) *SignatureHeaderCreator {
	logger.Info("======NewSignatureHeaderCreator===========")
	return &SignatureHeaderCreator{ss}
}

// NewSignatureHeader creates a SignatureHeader with the correct signing identity and a valid nonce
func (bs *SignatureHeaderCreator) NewSignatureHeader() (*cb.SignatureHeader, error) {
	logger.Info("======SignatureHeaderCreator=====NewSignatureHeader======")
	creator, err := bs.Serialize()
	if err != nil {
		return nil, err
	}
	nonce, err := GetRandomNonce()
	if err != nil {
		return nil, err
	}


	sid := &msp.SerializedIdentity{}
	err = proto.Unmarshal(creator, sid)
	logger.Info("==========creator===",sid.Mspid)

	return &cb.SignatureHeader{
		Creator: creator,
		Nonce:   nonce,
	}, nil
}
