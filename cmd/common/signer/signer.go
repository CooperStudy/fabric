/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package signer

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"github.com/hyperledger/fabric/common/flogging"
	"io/ioutil"
	"math/big"

	"github.com/hyperledger/fabric/bccsp/utils"
	"github.com/hyperledger/fabric/common/util"
	"github.com/hyperledger/fabric/protos/msp"
	proto_utils "github.com/hyperledger/fabric/protos/utils"
	"github.com/pkg/errors"
)

// Config holds the configuration for
// creation of a Signer
type Config struct {
	MSPID        string
	IdentityPath string
	KeyPath      string
}
var logger = flogging.MustGetLogger("cmd.common.signer")
// Signer signs messages.
// TODO: Ideally we'd use an MSP to be agnostic, but since it's impossible to
// initialize an MSP without a CA cert that signs the signing identity,
// this will do for now.
type Signer struct {
	key     *ecdsa.PrivateKey
	Creator []byte
}

// NewSigner creates a new Signer out of the given configuration
func NewSigner(conf Config) (*Signer, error) {
	logger.Info("=NewSigner===")
	sId, err := serializeIdentity(conf.IdentityPath, conf.MSPID)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	key, err := loadPrivateKey(conf.KeyPath)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &Signer{
		Creator: sId,
		key:     key,
	}, nil
}

func serializeIdentity(clientCert string, mspID string) ([]byte, error) {
	logger.Info("=serializeIdentity===")
	b, err := ioutil.ReadFile(clientCert)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	sId := &msp.SerializedIdentity{
		Mspid:   mspID,
		IdBytes: b,
	}
	return proto_utils.MarshalOrPanic(sId), nil
}

func (si *Signer) Sign(msg []byte) ([]byte, error) {
	logger.Info("===Signer===Sign===")
	digest := util.ComputeSHA256(msg)
	return signECDSA(si.key, digest)
}

func loadPrivateKey(file string) (*ecdsa.PrivateKey, error) {
	logger.Info("===loadPrivateKey======")
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	bl, _ := pem.Decode(b)
	if bl == nil {
		return nil, errors.Errorf("failed to decode PEM block from %s", file)
	}
	key, err := x509.ParsePKCS8PrivateKey(bl.Bytes)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse private key from %s", file)
	}
	return key.(*ecdsa.PrivateKey), nil
}

func signECDSA(k *ecdsa.PrivateKey, digest []byte) (signature []byte, err error) {
	logger.Info("===signECDSA======")
	r, s, err := ecdsa.Sign(rand.Reader, k, digest)
	logger.Info("===============r,s=====")
	if err != nil {
		return nil, err
	}

	s, _, err = utils.ToLowS(&k.PublicKey, s)
	logger.Info("=================")
	if err != nil {
		return nil, err
	}

	return marshalECDSASignature(r, s)
}

func marshalECDSASignature(r, s *big.Int) ([]byte, error) {
	logger.Info("=====marshalECDSASignature======")
	return asn1.Marshal(ECDSASignature{r, s})
}

type ECDSASignature struct {
	R, S *big.Int
}
