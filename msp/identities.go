/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package msp

import (
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/bccsp"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/protos/msp"
	"github.com/pkg/errors"
	"go.uber.org/zap/zapcore"
)

var mspIdentityLogger = flogging.MustGetLogger("msp.identity")

type identity struct {
	// id contains the identifier (MSPID and identity identifier) for this instance
	id *IdentityIdentifier

	// cert contains the x.509 certificate that signs the public key of this instance
	cert *x509.Certificate

	// this is the public key of this instance
	pk bccsp.Key

	// reference to the MSP that "owns" this identity
	msp *bccspmsp
}

func newIdentity(cert *x509.Certificate, pk bccsp.Key, msp *bccspmsp) (Identity, error) {
	mspLogger.Info("====func newIdentity(cert *x509.Certificate, pk bccsp.Key, msp *bccspmsp) (Identity, error)==")
	if mspIdentityLogger.IsEnabledFor(zapcore.DebugLevel) {
		mspIdentityLogger.Debugf("Creating identity instance for cert %s", certToPEM(cert))
	}

	// Sanitize first the certificate
	cert, err := msp.sanitizeCert(cert)
	if err != nil {
		return nil, err
	}

	// Compute identity identifier

	// Use the hash of the identity's certificate as id in the IdentityIdentifier
	hashOpt, err := bccsp.GetHashOpt(msp.cryptoConfig.IdentityIdentifierHashFunction)
	if err != nil {
		return nil, errors.WithMessage(err, "failed getting hash function options")
	}

	digest, err := msp.bccsp.Hash(cert.Raw, hashOpt)
	//mspLogger.Info("=====func newIdentity(cert *x509.Certificate, pk bccsp.Key, msp *bccspmsp)==================")
	//mspLogger.Info("====cert.Raw==================",cert.Raw)
	//[48 130 2 42 48 130 1 209 160 3 2 1 2 2 17 0 200 196 133 251 73 248 157 173 212 29 167 39 237 220 133 4 48 10 6 8 42 134 72 206 61 4 3 2 48 115 49 11 48 9 6 3 85 4 6 19 2 85 83 49 19 48 17 6 3 85 4 8 19 10 67 97 108 105 102 111 114 110 105 97 49 22 48 20 6 3 85 4 7 19 13 83 97 110 32 70 114 97 110 99 105 115 99 111 49 25 48 23 6 3 85 4 10 19 16 111 114 103 49 46 101 120 97 109 112 108 101 46 99 111 109 49 28 48 26 6 3 85 4 3 19 19 99 97 46 111 114 103 49 46 101 120 97 109 112 108 101 46 99 111 109 48 30 23 13 50 49 49 49 51 48 48 51 49 53 48 48 90 23 13 51 49 49 49 50 56 48 51 49 53 48 48 90 48 108 49 11 48 9 6 3 85 4 6 19 2 85 83 49 19 48 17 6 3 85 4 8 19 10 67 97 108 105 102 111 114 110 105 97 49 22 48 20 6 3 85 4 7 19 13 83 97 110 32 70 114 97 110 99 105 115 99 111 49 15 48 13 6 3 85 4 11 19 6 99 108 105 101 110 116 49 31 48 29 6 3 85 4 3 12 22 65 100 109 105 110 64 111 114 103 49 46 101 120 97 109 112 108 101 46 99 111 109 48 89 48 19 6 7 42 134 72 206 61 2 1 6 8 42 134 72 206 61 3 1 7 3 66 0 4 27 235 203 251 65 224 159 212 99 146 52 86 232 188 75 190 55 148 171 129 136 115 220 61 75 139 11 157 156 152 249 166 169 34 196 154 11 173 208 156 161 220 251 173 100 58 17 191 72 52 254 91 223 236 49 208 58 13 210 163 152 221 22 207 163 77 48 75 48 14 6 3 85 29 15 1 1 255 4 4 3 2 7 128 48 12 6 3 85 29 19 1 1 255 4 2 48 0 48 43 6 3 85 29 35 4 36 48 34 128 32 0 100 115 25 6 39 57 104 13 47 55 127 155 182 94 3 58 214 247 108 119 220 254 198 88 177 13 79 5 201 73 238 48 10 6 8 42 134 72 206 61 4 3 2 3 71 0 48 68 2 32 6 106 141 180 203 197 22 249 12 245 136 108 172 227 110 29 177 232 68 141 243 194 24 47 44 203 251 97 14 143 0 120 2 32 41 213 217 197 11 14 142 241 141 91 176 220 235 195 112 197 122 185 157 199 63 0 74 214 245 55 173 12 31 90 175 34]
	//mspLogger.Info("=====digest==================",digest)
	//aee1cbab6109e686a7408a38e25b0c654cbf964866a2f1fe50a0222404a1a547
	//mspLogger.Info("=====id:==hex.EncodeToString(digest)==================", hex.EncodeToString(digest))
	if err != nil {
		return nil, errors.WithMessage(err, "failed hashing raw certificate to compute the id of the IdentityIdentifier")
	}

	id := &IdentityIdentifier{
		Mspid: msp.name,
		Id:    hex.EncodeToString(digest)}
	
	mspLogger.Info("==IdentityIdentifier:MspId,Id======")
	mspLogger.Info("===identity{id: id., cert: cert, pk: pk, msp: msp}==")
	mspLogger.Infof("===Mspid: msp.name:%v,Id:string(digest):%v====",id.Mspid,id.Id)
	//Org1MSP  aee1cbab6109e686a7408a38e25b0c654cbf964866a2f1fe50a0222404a1a547
	return &identity{id: id, cert: cert, pk: pk, msp: msp}, nil
}

// ExpiresAt returns the time at which the Identity expires.
func (id *identity) ExpiresAt() time.Time {
	logger.Info("====identity==ExpiresAt=")
	return id.cert.NotAfter
}

// SatisfiesPrincipal returns null if this instance matches the supplied principal or an error otherwise
func (id *identity) SatisfiesPrincipal(principal *msp.MSPPrincipal) error {
	logger.Info("====identity==SatisfiesPrincipal=")
	return id.msp.SatisfiesPrincipal(id, principal)
}

// GetIdentifier returns the identifier (MSPID/IDID) for this instance
func (id *identity) GetIdentifier() *IdentityIdentifier {
	//logger.Info("====identity==GetIdentifier=")
	return id.id
}

// GetMSPIdentifier returns the MSP identifier for this instance
func (id *identity) GetMSPIdentifier() string {
	return id.id.Mspid
}

// Validate returns nil if this instance is a valid identity or an error otherwise
func (id *identity) Validate() error {
	logger.Info("====identity==Validate=")
	return id.msp.Validate(id)
}

// GetOrganizationalUnits returns the OU for this instance
func (id *identity) GetOrganizationalUnits() []*OUIdentifier {
	logger.Info("====identity==GetOrganizationalUnits=")
	if id.cert == nil {
		return nil
	}

	cid, err := id.msp.getCertificationChainIdentifier(id)
	if err != nil {
		mspIdentityLogger.Errorf("Failed getting certification chain identifier for [%v]: [%+v]", id, err)

		return nil
	}

	res := []*OUIdentifier{}
	for _, unit := range id.cert.Subject.OrganizationalUnit {
		res = append(res, &OUIdentifier{
			OrganizationalUnitIdentifier: unit,
			CertifiersIdentifier:         cid,
		})
	}

	return res
}

// Anonymous returns true if this identity provides anonymity
func (id *identity) Anonymous() bool {
	logger.Info("====identity==Anonymous=")
	return false
}

// NewSerializedIdentity returns a serialized identity
// having as content the passed mspID and x509 certificate in PEM format.
// This method does not check the validity of certificate nor
// any consistency of the mspID with it.
func NewSerializedIdentity(mspID string, certPEM []byte) ([]byte, error) {
	logger.Info("====NewSerializedIdentity=")
	// We serialize identities by prepending the MSPID
	// and appending the x509 cert in PEM format
	sId := &msp.SerializedIdentity{Mspid: mspID, IdBytes: certPEM}
	raw, err := proto.Marshal(sId)
	if err != nil {
		return nil, errors.Wrapf(err, "failed serializing identity [%s][%X]", mspID, certPEM)
	}
	return raw, nil
}

// Verify checks against a signature and a message
// to determine whether this identity produced the
// signature; it returns nil if so or an error otherwise
func (id *identity) Verify(msg []byte, sig []byte) error {
	//logger.Info("==identity==Verify=")
	// mspIdentityLogger.Infof("Verifying signature")

	// Compute Hash
	hashOpt, err := id.getHashOpt(id.msp.cryptoConfig.SignatureHashFamily)
	if err != nil {
		return errors.WithMessage(err, "failed getting hash function options")
	}

	digest, err := id.msp.bccsp.Hash(msg, hashOpt)
	if err != nil {
		return errors.WithMessage(err, "failed computing digest")
	}

	if mspIdentityLogger.IsEnabledFor(zapcore.DebugLevel) {
		mspIdentityLogger.Debugf("Verify: digest = %s", hex.Dump(digest))
		mspIdentityLogger.Debugf("Verify: sig = %s", hex.Dump(sig))
	}

	//                                   pk this is the public key of this instance sig digest
	valid, err := id.msp.bccsp.Verify(id.pk, sig, digest, nil)
	if err != nil {
		return errors.WithMessage(err, "could not determine the validity of the signature")
	} else if !valid {
		return errors.New("The signature is invalid")
	}

	return nil
}

// Serialize returns a byte array representation of this identity
func (id *identity) Serialize() ([]byte, error) {
	mspLogger.Info("==func (id *identity) Serialize() ([]byte, error)=")
	// mspIdentityLogger.Infof("Serializing identity %s", id.id)

	pb := &pem.Block{Bytes: id.cert.Raw, Type: "CERTIFICATE"}
	pemBytes := pem.EncodeToMemory(pb)
	if pemBytes == nil {
		return nil, errors.New("encoding of identity failed")
	}

	// We serialize identities by prepending the MSPID and appending the ASN.1 DER content of the cert
	sId := &msp.SerializedIdentity{Mspid: id.id.Mspid, IdBytes: pemBytes}
	//mspLogger.Info("=======sId := &msp.SerializedIdentity{Mspid: id.id.Mspid, IdBytes: pemBytes}====================")
	//mspLogger.Infof("========func(id *identity) Serialize()  Mspid:%v,IdBytes:pemBytes:%v===================",id.id.Mspid,pemBytes)
	//mspLogger.Infof("=======pemBytes:%v===================",pemBytes)
	//[45 45 45 45 45 66 69 71 73 78 32 67 69 82 84 73 70 73 67 65 84 69 45 45 45 45 45 10 77 73 73 67 75 106 67 67 65 100 71 103 65 119 73 66 65 103 73 82 65 77 106 69 104 102 116 74 43 74 50 116 49 66 50 110 74 43 51 99 104 81 81 119 67 103 89 73 75 111 90 73 122 106 48 69 65 119 73 119 99 122 69 76 10 77 65 107 71 65 49 85 69 66 104 77 67 86 86 77 120 69 122 65 82 66 103 78 86 66 65 103 84 67 107 78 104 98 71 108 109 98 51 74 117 97 87 69 120 70 106 65 85 66 103 78 86 66 65 99 84 68 86 78 104 98 105 66 71 10 99 109 70 117 89 50 108 122 89 50 56 120 71 84 65 88 66 103 78 86 66 65 111 84 69 71 57 121 90 122 69 117 90 88 104 104 98 88 66 115 90 83 53 106 98 50 48 120 72 68 65 97 66 103 78 86 66 65 77 84 69 50 78 104 10 76 109 57 121 90 122 69 117 90 88 104 104 98 88 66 115 90 83 53 106 98 50 48 119 72 104 99 78 77 106 69 120 77 84 77 119 77 68 77 120 78 84 65 119 87 104 99 78 77 122 69 120 77 84 73 52 77 68 77 120 78 84 65 119 10 87 106 66 115 77 81 115 119 67 81 89 68 86 81 81 71 69 119 74 86 85 122 69 84 77 66 69 71 65 49 85 69 67 66 77 75 81 50 70 115 97 87 90 118 99 109 53 112 89 84 69 87 77 66 81 71 65 49 85 69 66 120 77 78 10 85 50 70 117 73 69 90 121 89 87 53 106 97 88 78 106 98 122 69 80 77 65 48 71 65 49 85 69 67 120 77 71 89 50 120 112 90 87 53 48 77 82 56 119 72 81 89 68 86 81 81 68 68 66 90 66 90 71 49 112 98 107 66 118 10 99 109 99 120 76 109 86 52 89 87 49 119 98 71 85 117 89 50 57 116 77 70 107 119 69 119 89 72 75 111 90 73 122 106 48 67 65 81 89 73 75 111 90 73 122 106 48 68 65 81 99 68 81 103 65 69 71 43 118 76 43 48 72 103 10 110 57 82 106 107 106 82 87 54 76 120 76 118 106 101 85 113 52 71 73 99 57 119 57 83 52 115 76 110 90 121 89 43 97 97 112 73 115 83 97 67 54 51 81 110 75 72 99 43 54 49 107 79 104 71 47 83 68 84 43 87 57 47 115 10 77 100 65 54 68 100 75 106 109 78 48 87 122 54 78 78 77 69 115 119 68 103 89 68 86 82 48 80 65 81 72 47 66 65 81 68 65 103 101 65 77 65 119 71 65 49 85 100 69 119 69 66 47 119 81 67 77 65 65 119 75 119 89 68 10 86 82 48 106 66 67 81 119 73 111 65 103 65 71 82 122 71 81 89 110 79 87 103 78 76 122 100 47 109 55 90 101 65 122 114 87 57 50 120 51 51 80 55 71 87 76 69 78 84 119 88 74 83 101 52 119 67 103 89 73 75 111 90 73 10 122 106 48 69 65 119 73 68 82 119 65 119 82 65 73 103 66 109 113 78 116 77 118 70 70 118 107 77 57 89 104 115 114 79 78 117 72 98 72 111 82 73 51 122 119 104 103 118 76 77 118 55 89 81 54 80 65 72 103 67 73 67 110 86 10 50 99 85 76 68 111 55 120 106 86 117 119 51 79 118 68 99 77 86 54 117 90 51 72 80 119 66 75 49 118 85 51 114 81 119 102 87 113 56 105 10 45 45 45 45 45 69 78 68 32 67 69 82 84 73 70 73 67 65 84 69 45 45 45 45 45 10]

	idBytes, err := proto.Marshal(sId)
	if err != nil {
		return nil, errors.Wrapf(err, "could not marshal a SerializedIdentity structure for identity %s", id.id)
	}

	return idBytes, nil
}

func (id *identity) getHashOpt(hashFamily string) (bccsp.HashOpts, error) {
	//logger.Info("==identity==getHashOpt=")
	switch hashFamily {
	case bccsp.SHA2:
		//logger.Info("====bccsp.SHA2============")
		return bccsp.GetHashOpt(bccsp.SHA256)
	case bccsp.SHA3:
	//	logger.Info("====bccsp.SHA3============")
		return bccsp.GetHashOpt(bccsp.SHA3_256)
	}
	return nil, errors.Errorf("hash familiy not recognized [%s]", hashFamily)
}

type signingidentity struct {
	// we embed everything from a base identity
	identity

	// signer corresponds to the object that can produce signatures from this identity
	signer crypto.Signer
}

func newSigningIdentity(cert *x509.Certificate, pk bccsp.Key, signer crypto.Signer, msp *bccspmsp) (SigningIdentity, error) {
	//logger.Info("==newSigningIdentity=")
	//mspIdentityLogger.Infof("Creating signing identity instance for ID %s", id)
	mspId, err := newIdentity(cert, pk, msp)
	if err != nil {
		return nil, err
	}
	return &signingidentity{identity: *mspId.(*identity), signer: signer}, nil
}

// Sign produces a signature over msg, signed by this instance
func (id *signingidentity) Sign(msg []byte) ([]byte, error) {
	//logger.Info("==signingidentity=Sign==")
	//mspIdentityLogger.Infof("Signing message")

	// Compute Hash
	hashOpt, err := id.getHashOpt(id.msp.cryptoConfig.SignatureHashFamily)
	if err != nil {
		return nil, errors.WithMessage(err, "failed getting hash function options")
	}

	digest, err := id.msp.bccsp.Hash(msg, hashOpt)
	if err != nil {
		return nil, errors.WithMessage(err, "failed computing digest")
	}

	if len(msg) < 32 {
		mspIdentityLogger.Debugf("Sign: plaintext: %X \n", msg)
	} else {
		mspIdentityLogger.Debugf("Sign: plaintext: %X...%X \n", msg[0:16], msg[len(msg)-16:])
	}
	mspIdentityLogger.Debugf("Sign: digest: %X \n", digest)

	// Sign
	return id.signer.Sign(rand.Reader, digest, nil)
}

// GetPublicVersion returns the public version of this identity,
// namely, the one that is only able to verify messages and not sign them
func (id *signingidentity) GetPublicVersion() Identity {
	//logger.Info("==signingidentity=GetPublicVersion==")
	return &id.identity
}
