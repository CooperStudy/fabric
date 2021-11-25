/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package v13

import (
	"fmt"
	"regexp"

	commonerrors "github.com/hyperledger/fabric/common/errors"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/core/chaincode/platforms/ccmetadata"
	. "github.com/hyperledger/fabric/core/common/validation/statebased"
	. "github.com/hyperledger/fabric/core/handlers/validation/api/capabilities"
	. "github.com/hyperledger/fabric/core/handlers/validation/api/identities"
	. "github.com/hyperledger/fabric/core/handlers/validation/api/policies"
	. "github.com/hyperledger/fabric/core/handlers/validation/api/state"
	"github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric/protos/peer"
	"github.com/hyperledger/fabric/protos/utils"
)

var logger = flogging.MustGetLogger("vscc")

var validCollectionNameRegex = regexp.MustCompile(ccmetadata.AllowedCharsCollectionName)

//go:generate mockery -dir ../../api/capabilities/ -name Capabilities -case underscore -output mocks/
//go:generate mockery -dir ../../api/state/ -name StateFetcher -case underscore -output mocks/
//go:generate mockery -dir ../../api/identities/ -name IdentityDeserializer -case underscore -output mocks/
//go:generate mockery -dir ../../api/policies/ -name PolicyEvaluator -case underscore -output mocks/
//go:generate mockery -dir . -name StateBasedValidator -case underscore -output mocks/

// New creates a new instance of the default VSCC
// Typically this will only be invoked once per peer
func New(c Capabilities, s StateFetcher, d IdentityDeserializer, pe PolicyEvaluator) *Validator {
	vpmgr := &KeyLevelValidationParameterManagerImpl{StateFetcher: s}
	sbv := NewKeyLevelValidator(pe, vpmgr)

	return &Validator{
		capabilities:        c,
		stateFetcher:        s,
		deserializer:        d,
		policyEvaluator:     pe,
		stateBasedValidator: sbv,
	}
}

// Validator implements the default transaction validation policy,
// which is to check the correctness of the read-write set and the endorsement
// signatures against an endorsement policy that is supplied as argument to
// every invoke
type Validator struct {
	deserializer        IdentityDeserializer
	capabilities        Capabilities
	stateFetcher        StateFetcher
	policyEvaluator     PolicyEvaluator
	stateBasedValidator StateBasedValidator
}

type validationArtifacts struct {
	rwset        []byte
	prp          []byte
	endorsements []*peer.Endorsement
	chdr         *common.ChannelHeader
	env          *common.Envelope
	payl         *common.Payload
	cap          *peer.ChaincodeActionPayload
}
//                                                                           0                0
func (vscc *Validator) extractValidationArtifacts(block *common.Block, txPosition int, actionPosition int, ) (*validationArtifacts, error) {
	logger.Info("=======================func (vscc *Validator) extractValidationArtifacts(block *common.Block, txPosition int, actionPosition int, ) (*validationArtifacts, error)====================================")
	// get the envelope...
	env, err := utils.GetEnvelopeFromBlock(block.Data.Data[txPosition])
	logger.Infof("=========env, err := utils.GetEnvelopeFromBlock(block.Data.Data(%v)[%v])============================",block.Data.Data,txPosition)
	if err != nil {
		logger.Errorf("VSCC error: GetEnvelope failed, err %s", err)
		return nil, err
	}

	// ...and the payload...
	logger.Infof("========payload, err := utils.GetPayload(%v)============================",env)
	/*
	========payload, err := utils.GetPayload(payload:"\n\303\007\ng\010\003\032\014\010\372\273\374\214\006\020\326\226\345\272\003\"\tmychannel*@ae72834653d48c044266ba6513ba51cd38b1b0a1cd8a05f158f7a4d5d9e715c8:\010\022\006\022\004mycc\022\327\006\n\272\006\n\007Org1MSP\022\256\006-----BEGIN CERTIFICATE-----\nMIICKzCCAdGgAwIBAgIRAJWw9CgNRhttvjsyMrAeODowCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzEuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzEuZXhhbXBsZS5jb20wHhcNMjExMTI1MDUxODAwWhcNMzExMTIzMDUxODAw\nWjBsMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzEPMA0GA1UECxMGY2xpZW50MR8wHQYDVQQDDBZBZG1pbkBv\ncmcxLmV4YW1wbGUuY29tMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEFmj7lz82\noJdEdS7OWkoTI8l9wK5iRyiHPm3OnO2gQI0bW5QQokxLEtsm2Cl/euLHklkoNPxK\nsqIspJgl1JolSqNNMEswDgYDVR0PAQH/BAQDAgeAMAwGA1UdEwEB/wQCMAAwKwYD\nVR0jBCQwIoAgwiSsIUWHtRNbtDX7nrleiPJ9xY/0foPgl7sHBx5pD/wwCgYIKoZI\nzj0EAwIDSAAwRQIhAI45b4pY9x6obrU88qoHnPP8y079H4GFDuFWh47jCi2PAiBW\nTREeVRb8580B8hJpFcse4KdPuFrlDV9AzkYmB/OTHg==\n-----END CERTIFICATE-----\n\022\030T\322|_\275'&\327\004r\352\313\303\337\257\243\375\237\360\274\001\210b\271\022\217\026\n\214\026\n\327\006\n\272\006\n\007Org1MSP\022\256\006-----BEGIN CERTIFICATE-----\nMIICKzCCAdGgAwIBAgIRAJWw9CgNRhttvjsyMrAeODowCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzEuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzEuZXhhbXBsZS5jb20wHhcNMjExMTI1MDUxODAwWhcNMzExMTIzMDUxODAw\nWjBsMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzEPMA0GA1UECxMGY2xpZW50MR8wHQYDVQQDDBZBZG1pbkBv\ncmcxLmV4YW1wbGUuY29tMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEFmj7lz82\noJdEdS7OWkoTI8l9wK5iRyiHPm3OnO2gQI0bW5QQokxLEtsm2Cl/euLHklkoNPxK\nsqIspJgl1JolSqNNMEswDgYDVR0PAQH/BAQDAgeAMAwGA1UdEwEB/wQCMAAwKwYD\nVR0jBCQwIoAgwiSsIUWHtRNbtDX7nrleiPJ9xY/0foPgl7sHBx5pD/wwCgYIKoZI\nzj0EAwIDSAAwRQIhAI45b4pY9x6obrU88qoHnPP8y079H4GFDuFWh47jCi2PAiBW\nTREeVRb8580B8hJpFcse4KdPuFrlDV9AzkYmB/OTHg==\n-----END CERTIFICATE-----\n\022\030T\322|_\275'&\327\004r\352\313\303\337\257\243\375\237\360\274\001\210b\271\022\257\017\n\"\n \n\036\010\001\022\006\022\004mycc\032\022\n\006invoke\n\001a\n\001b\n\00210\022\210\017\n}\n \235\\\366\002/4zJw\r\313\301h\020/\331z*\030\022\225\327\\|\244\204]\347\031\323/\034\022Y\nE\022\024\n\004lscc\022\014\n\n\n\004mycc\022\002\010\003\022-\n\004mycc\022%\n\007\n\001a\022\002\010\003\n\007\n\001b\022\002\010\003\032\007\n\001a\032\00290\032\010\n\001b\032\003210\032\003\010\310\001\"\013\022\004mycc\032\0031.0\022\202\007\n\266\006\n\007Org1MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKTCCAc+gAwIBAgIRALDGTP2aarni8NWiitvYqvEwCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzEuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzEuZXhhbXBsZS5jb20wHhcNMjExMTI1MDUxODAwWhcNMzExMTIzMDUxODAw\nWjBqMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzENMAsGA1UECxMEcGVlcjEfMB0GA1UEAxMWcGVlcjAub3Jn\nMS5leGFtcGxlLmNvbTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABPdkUsaWX7Nc\ndqycTZ1kzfSif3FtbFP6I6dk88vdhysyFiSm0UCs/0wpLw+aAz28+UcOfa2nNuTb\nDldYG3EgBsCjTTBLMA4GA1UdDwEB/wQEAwIHgDAMBgNVHRMBAf8EAjAAMCsGA1Ud\nIwQkMCKAIMIkrCFFh7UTW7Q1+565XojyfcWP9H6D4Je7BwceaQ/8MAoGCCqGSM49\nBAMCA0gAMEUCIQC2Xma1FQKwtkxML8rOQz2+IwAehSqRZ3oHnbBn7nUvCAIgMPfH\nstgskchTzlezePM3OXlg8N6C5yZiH9aeP1N7NqA=\n-----END CERTIFICATE-----\n\022G0E\002!\000\270\361\232\247\235\3012bQ\365\237\013\240$:i\212S\"\361w8JLv\357V\030\245\241;\256\002 %\013.$\000aT\"3\330\371~\013q!\262ln{\252L\356\255\330\235:4\246*\001\213\362\022\201\007\n\266\006\n\007Org2MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKDCCAc6gAwIBAgIQBKkaj2D2kpm1NbUrOk+YczAKBggqhkjOPQQDAjBzMQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEZMBcGA1UEChMQb3JnMi5leGFtcGxlLmNvbTEcMBoGA1UEAxMTY2Eu\nb3JnMi5leGFtcGxlLmNvbTAeFw0yMTExMjUwNTE4MDBaFw0zMTExMjMwNTE4MDBa\nMGoxCzAJBgNVBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1T\nYW4gRnJhbmNpc2NvMQ0wCwYDVQQLEwRwZWVyMR8wHQYDVQQDExZwZWVyMC5vcmcy\nLmV4YW1wbGUuY29tMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEJ9IRUy2KNqME\nv07T564D3AI9oDcmIUD5R9KbAeNiPHNUCY/L12p0lPQ8nfUPI8Cn9/sOLg5eeAdP\nxLlu+C+SB6NNMEswDgYDVR0PAQH/BAQDAgeAMAwGA1UdEwEB/wQCMAAwKwYDVR0j\nBCQwIoAg/7WRDxBkf8mYroOHTeR5Ff5ZlYmK1a9jG1b5/jZARR0wCgYIKoZIzj0E\nAwIDSAAwRQIhAPxjH62TcFtnZ70enhkDcYmhVMStc8nNsad35DooAbvJAiBXrmr/\n+DMzEfh9Dl2FAimuBtjp1HHuEwi80b5x/FOrzw==\n-----END CERTIFICATE-----\n\022F0D\002 3\362.\222m\232\254\375\206-\021\3062:\346\257\271&\257a\304\340\355\003*\327\275\016\277\271\021]\002 7<\263F\225\316]\020c\367\001}\231\254\370KY6]\000\232\275\326F\344+AA\007\037\242\260" signature:"0D\002 \014\342N\211\200\322\220\037\26233p>\346<\377\241X\302\215\026\241\335\240U:\363\235\\\256\262L\002 j\316\371\332xk\355AP\331\006\t\237\273e{\324\305\347\273\216\235\366\303{\023a\002%f\220\344" )============================
	*/

	payl, err := utils.GetPayload(env)
	if err != nil {
		logger.Errorf("VSCC error: GetPayload failed, err %s", err)
		return nil, err
	}

	logger.Infof("========chdr, err := utils.UnmarshalChannelHeader(%v)============================",payl.Header.ChannelHeader)
	//========chdr, err := utils.UnmarshalChannelHeader([8 3 26 12 8 250 187 252 140 6 16 214 150 229 186 3 34 9 109 121 99 104 97 110 110 101 108 42 64 97 101 55 50 56 51 52 54 53 51 100 52 56 99 48 52 52 50 54 54 98 97 54 53 49 51 98 97 53 49 99 100 51 56 98 49 98 48 97 49 99 100 56 97 48 53 102 49 53 56 102 55 97 52 100 53 100 57 101 55 49 53 99 56 58 8 18 6 18 4 109 121 99 99])
	chdr, err := utils.UnmarshalChannelHeader(payl.Header.ChannelHeader)
	if err != nil {
		return nil, err
	}

	logger.Infof("==================common.HeaderType(%v) ===================",common.HeaderType(chdr.Type) )
	//==================common.HeaderType(ENDORSER_TRANSACTION) ===================
	logger.Info("==================common.HeaderType_ENDORSER_TRANSACTION===================",common.HeaderType_ENDORSER_TRANSACTION )
	// validate the payload type
	if common.HeaderType(chdr.Type) != common.HeaderType_ENDORSER_TRANSACTION {
		logger.Errorf("Only Endorser Transactions are supported, provided type %d", chdr.Type)
		err = fmt.Errorf("Only Endorser Transactions are supported, provided type %d", chdr.Type)
		return nil, err
	}

	// ...and the transaction...
	logger.Infof("================tx, err := utils.GetTransaction(%v)====================",payl.Data)
	tx, err := utils.GetTransaction(payl.Data)
	if err != nil {
		logger.Errorf("VSCC error: GetTransaction failed, err %s", err)
		return nil, err
	}

	logger.Infof("================tx:%v====================",tx)
	logger.Infof("===============\tcap, err := utils.GetChaincodeActionPayload(%v)===================",tx.Actions[actionPosition].Payload)
	cap, err := utils.GetChaincodeActionPayload(tx.Actions[actionPosition].Payload)
	if err != nil {
		logger.Errorf("VSCC error: GetChaincodeActionPayload failed, err %s", err)
		return nil, err
	}

	logger.Info("==============cap=============",cap)
	logger.Infof("============= utils.GetProposalResponsePayload(%v)====================",cap.Action.ProposalResponsePayload)

	pRespPayload, err := utils.GetProposalResponsePayload(cap.Action.ProposalResponsePayload)
	if err != nil {
		err = fmt.Errorf("GetProposalResponsePayload error %s", err)
		return nil, err
	}

	logger.Info("==============pRespPayload=============",pRespPayload)
	if pRespPayload.Extension == nil {
		err = fmt.Errorf("nil pRespPayload.Extension")
		return nil, err
	}

	logger.Infof("==============utils.GetChaincodeAction(%v)=============",pRespPayload.Extension)
	respPayload, err := utils.GetChaincodeAction(pRespPayload.Extension)
	if err != nil {
		err = fmt.Errorf("GetChaincodeAction error %s", err)
		return nil, err
	}
	logger.Infof("==============%v, %v := utils.GetChaincodeAction(%v)=============",respPayload,err,pRespPayload.Extension)
	return &validationArtifacts{
		rwset:        respPayload.Results,
		prp:          cap.Action.ProposalResponsePayload,
		endorsements: cap.Action.Endorsements,
		chdr:         chdr,
		env:          env,
		payl:         payl,
		cap:          cap,
	}, nil
}

// Validate validates the given envelope corresponding to a transaction with an endorsement
// policy as given in its serialized form.
// Note that in the case of dependencies in a block, such as tx_n modifying the endorsement policy
// for key a and tx_n+1 modifying the value of key a, Validate(tx_n+1) will block until Validate(tx_n)
// has been resolved. If working with a limited number of goroutines for parallel validation, ensure
// that they are allocated to transactions in ascending order.
func (vscc *Validator) Validate(block *common.Block, namespace string, txPosition int, actionPosition int, policyBytes []byte, ) commonerrors.TxValidationError {
	logger.Infof("==1 fabric/core/handlers/builtin/v13/validation_logic.go  func (vscc *Validator) Validate(block *common.Block, namespace string, txPosition int, actionPosition int, policyBytes []byte, ) commonerrors.TxValidationError ===")

	logger.Infof("===================vscc.stateBasedValidator.PreValidate(%v), %v)===========================",txPosition,block)
	/*
	===================vscc.stateBasedValidator.PreValidate(0), header:<number:4 previous_hash:"F\353\207\233\274\206/\023\005\337~\026\177\206=]\213\305\210\343>\266 \334\217c3\216\277\373\311r" data_hash:"g\220\357\031w\001D\000\245\363\313\361U\026~D\200\242\325\301\002,\312\223jeJ\311l\303\353d" > data:<data:"\n\320\035\n\277\007\ng\010\003\032\014\010\310\313\373\214\006\020\362\231\234\243\003\"\tmychannel*@f112c573b6b8026deab5288ce780b2dac7c989d1a5b22264889ac4ac93343c91:\010\022\006\022\004mycc\022\323\006\n\266\006\n\007Org1MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKjCCAdGgAwIBAgIRAOYXjB1TyGmXvN+MaQcaK/0wCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzEuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzEuZXhhbXBsZS5jb20wHhcNMjExMTI1MDExOTAwWhcNMzExMTIzMDExOTAw\nWjBsMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzEPMA0GA1UECxMGY2xpZW50MR8wHQYDVQQDDBZBZG1pbkBv\ncmcxLmV4YW1wbGUuY29tMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEVYrjCoTa\nxEWE5zRUVVAldDdeGOMccoxjNyK3Tcd6SpkMB28xgJsnRhYrj5eEeEFxq5DFhPRQ\nw6bdZOhCYz9mf6NNMEswDgYDVR0PAQH/BAQDAgeAMAwGA1UdEwEB/wQCMAAwKwYD\nVR0jBCQwIoAgUggJW3txP1Y680v1c15t4XsrvsHdVt6kg5IydAPKIocwCgYIKoZI\nzj0EAwIDRwAwRAIgSyqASkM0FQLiMzOigqfABmxwaPoAFJGsbO57X+cIyWYCID5r\nJHGBlxjY1XNu7+jr4taRw/rQuK204WBVu6YCP2Be\n-----END CERTIFICATE-----\n\022\030*\271\216\273\002\\\022\tre\326\362v/\246\371z\356\274\035\0306\3036\022\213\026\n\210\026\n\323\006\n\266\006\n\007Org1MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKjCCAdGgAwIBAgIRAOYXjB1TyGmXvN+MaQcaK/0wCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzEuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzEuZXhhbXBsZS5jb20wHhcNMjExMTI1MDExOTAwWhcNMzExMTIzMDExOTAw\nWjBsMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzEPMA0GA1UECxMGY2xpZW50MR8wHQYDVQQDDBZBZG1pbkBv\ncmcxLmV4YW1wbGUuY29tMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEVYrjCoTa\nxEWE5zRUVVAldDdeGOMccoxjNyK3Tcd6SpkMB28xgJsnRhYrj5eEeEFxq5DFhPRQ\nw6bdZOhCYz9mf6NNMEswDgYDVR0PAQH/BAQDAgeAMAwGA1UdEwEB/wQCMAAwKwYD\nVR0jBCQwIoAgUggJW3txP1Y680v1c15t4XsrvsHdVt6kg5IydAPKIocwCgYIKoZI\nzj0EAwIDRwAwRAIgSyqASkM0FQLiMzOigqfABmxwaPoAFJGsbO57X+cIyWYCID5r\nJHGBlxjY1XNu7+jr4taRw/rQuK204WBVu6YCP2Be\n-----END CERTIFICATE-----\n\022\030*\271\216\273\002\\\022\tre\326\362v/\246\371z\356\274\035\0306\3036\022\257\017\n\"\n \n\036\010\001\022\006\022\004mycc\032\022\n\006invoke\n\001a\n\001b\n\00210\022\210\017\n}\n \327\314j\014\243\314A4h[\353-h\232\207;\302*\305\350T\374\320\214\325\333\213Q\232\235\240\314\022Y\nE\022\024\n\004lscc\022\014\n\n\n\004mycc\022\002\010\003\022-\n\004mycc\022%\n\007\n\001a\022\002\010\003\n\007\n\001b\022\002\010\003\032\007\n\001a\032\00290\032\010\n\001b\032\003210\032\003\010\310\001\"\013\022\004mycc\032\0031.0\022\202\007\n\266\006\n\007Org1MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKTCCAc+gAwIBAgIRAMxPmHXrWH4pFNx33A3BFuswCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzEuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzEuZXhhbXBsZS5jb20wHhcNMjExMTI1MDExOTAwWhcNMzExMTIzMDExOTAw\nWjBqMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzENMAsGA1UECxMEcGVlcjEfMB0GA1UEAxMWcGVlcjAub3Jn\nMS5leGFtcGxlLmNvbTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABA7R7EvbUEak\nIQ8Q0Yn3fNSJlG0xi0tOU8XRXoV5ZczB+fY3WbTpzRG/hhNE45zaUInP2eh+bGwv\ntcY1EGFKw9WjTTBLMA4GA1UdDwEB/wQEAwIHgDAMBgNVHRMBAf8EAjAAMCsGA1Ud\nIwQkMCKAIFIICVt7cT9WOvNL9XNebeF7K77B3VbepIOSMnQDyiKHMAoGCCqGSM49\nBAMCA0gAMEUCIQD7Epgj12JuKu7M4NQ6D3GyDrKBfaSWOrgM9bbY6iJUDgIgN2d2\nnZCaFvu6QDG/Kk3FBg6aF2M2ZZ/2X7MOg/k4YbA=\n-----END CERTIFICATE-----\n\022G0E\002!\000\340v\321ET\225\330\220f\007]\250m:\366\3167\245{\326\035\354\273J=\273\313\3514Xu:\002 \002\017\366y_^\330\013^*\221\253)9\305\006\333$\317\275\316\0228\003\031C\344\307Z\324\315\237\022\201\007\n\266\006\n\007Org2MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKTCCAc+gAwIBAgIRALVUgZt5qdpmSUDNh5EABbMwCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzIuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzIuZXhhbXBsZS5jb20wHhcNMjExMTI1MDExOTAwWhcNMzExMTIzMDExOTAw\nWjBqMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzENMAsGA1UECxMEcGVlcjEfMB0GA1UEAxMWcGVlcjAub3Jn\nMi5leGFtcGxlLmNvbTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABOCWoh/kXAKe\ns3y4Y+4ZB6tq4mYwsZhePHdHqXbsthnQ6ra87H3bJp1w/QL0BCjh/pPUFFKDZSqk\nWg4t2BB9eyOjTTBLMA4GA1UdDwEB/wQEAwIHgDAMBgNVHRMBAf8EAjAAMCsGA1Ud\nIwQkMCKAIJOWSkw4YbR7/TtpSzr4yr2KvplbbBcF5S9POxOX0lUXMAoGCCqGSM49\nBAMCA0gAMEUCIQDxxvQqf8w7svfUyGy4R/frJycb7rXngTooMef3DM8aYQIgA4Im\nzktJeVxb+1oTmgW5VdlkiPmNGTt0ECoIPbrWTOE=\n-----END CERTIFICATE-----\n\022F0D\002 \tT*vbQ\347\251X\300\032\257$\032\024x\210\217\000 +-9\331'{D\222P\031\246\204\002 ,\344\0317\261)\365`\000\020\366\274\344\010\274*A6\261\256\236:\266!)J\037\247\242G\346\266\022F0D\002 S#5\231\022\255\n\013\366w\337\273;J\344\010r\220\370*\330-\030\3725\204\021 H&\r\362\002 \004\267e\361\020t\204\032\"\236\275\372\236\331_\230l%$\365\262&\245\367~\220Q\006\3500FE" > metadata:<metadata:"\022\371\006\n\255\006\n\220\006\n\nOrdererMSP\022\201\006-----BEGIN CERTIFICATE-----\nMIICCzCCAbKgAwIBAgIQTACGv8V29V/UnWBgVBwKODAKBggqhkjOPQQDAjBpMQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEUMBIGA1UEChMLZXhhbXBsZS5jb20xFzAVBgNVBAMTDmNhLmV4YW1w\nbGUuY29tMB4XDTIxMTEyNTAxMTkwMFoXDTMxMTEyMzAxMTkwMFowWDELMAkGA1UE\nBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBGcmFuY2lz\nY28xHDAaBgNVBAMTE29yZGVyZXIuZXhhbXBsZS5jb20wWTATBgcqhkjOPQIBBggq\nhkjOPQMBBwNCAAQM0B5ZrFkAfMUdPcAbiGFkib+3aPYZfyZW7cC2Nbe3iHGKhJGE\nEbFwsrHLxi3zOcTV2ff+HZnY3fNUGxtVrDQqo00wSzAOBgNVHQ8BAf8EBAMCB4Aw\nDAYDVR0TAQH/BAIwADArBgNVHSMEJDAigCBggxFFk0YKcPjVD0djn57iHGqgti71\nuMcUvMT2zYJMAjAKBggqhkjOPQQDAgNHADBEAiAe9CjkOh3gJcRRWqmfu+DH1odZ\neDTiCK4AelnE2VIOFwIgNJxll2PbxGk54UFjcAwGJ7lmqmUnnFMHzfPxbV017Q4=\n-----END CERTIFICATE-----\n\022\030\013\030\336\214\362\224\245\235\234\302\202\005!\251\321\361\300\351i\306\272H\206U\022G0E\002!\000\324\256w\262\306\375A;\222H\r\241\357Ur\2334\017L\274\205\331B=\271/\034\023\346\224x\252\002 \0308q\217rN\220\375z\335\240\214\231d\2127r2\357\231\337%\245\247\312\331\3100\3363\022\242" metadata:"\n\002\010\002\022\371\006\n\255\006\n\220\006\n\nOrdererMSP\022\201\006-----BEGIN CERTIFICATE-----\nMIICCzCCAbKgAwIBAgIQTACGv8V29V/UnWBgVBwKODAKBggqhkjOPQQDAjBpMQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEUMBIGA1UEChMLZXhhbXBsZS5jb20xFzAVBgNVBAMTDmNhLmV4YW1w\nbGUuY29tMB4XDTIxMTEyNTAxMTkwMFoXDTMxMTEyMzAxMTkwMFowWDELMAkGA1UE\nBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBGcmFuY2lz\nY28xHDAaBgNVBAMTE29yZGVyZXIuZXhhbXBsZS5jb20wWTATBgcqhkjOPQIBBggq\nhkjOPQMBBwNCAAQM0B5ZrFkAfMUdPcAbiGFkib+3aPYZfyZW7cC2Nbe3iHGKhJGE\nEbFwsrHLxi3zOcTV2ff+HZnY3fNUGxtVrDQqo00wSzAOBgNVHQ8BAf8EBAMCB4Aw\nDAYDVR0TAQH/BAIwADArBgNVHSMEJDAigCBggxFFk0YKcPjVD0djn57iHGqgti71\nuMcUvMT2zYJMAjAKBggqhkjOPQQDAgNHADBEAiAe9CjkOh3gJcRRWqmfu+DH1odZ\neDTiCK4AelnE2VIOFwIgNJxll2PbxGk54UFjcAwGJ7lmqmUnnFMHzfPxbV017Q4=\n-----END CERTIFICATE-----\n\022\030\336\201\377\001\207\3675*\205\247\234j!\247a\357V\3664*a\353Do\022G0E\002!\000\360 q\237Xbx\272\ns>\005\277\004\np.(\365\004M9\330\310\306E\0377r\357,_\002 WTD\034,\204\277\240Q\344\344H\325\267\217\202\366+\203N\337\243i\335T\211\014\000\221\254$\376" metadata:"" metadata:"" > )===========================
	 */
	vscc.stateBasedValidator.PreValidate(uint64(txPosition), block)

	logger.Infof("=================== vscc.extractValidationArtifacts(%v, %v, %v)===========================",block,txPosition,actionPosition)
	/*
	 =================== vscc.extractValidationArtifacts(header:<number:4 previous_hash:"F\353\207\233\274\206/\023\005\337~\026\177\206=]\213\305\210\343>\266 \334\217c3\216\277\373\311r" data_hash:"g\220\357\031w\001D\000\245\363\313\361U\026~D\200\242\325\301\002,\312\223jeJ\311l\303\353d" > data:<data:"\n\320\035\n\277\007\ng\010\003\032\014\010\310\313\373\214\006\020\362\231\234\243\003\"\tmychannel*@f112c573b6b8026deab5288ce780b2dac7c989d1a5b22264889ac4ac93343c91:\010\022\006\022\004mycc\022\323\006\n\266\006\n\007Org1MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKjCCAdGgAwIBAgIRAOYXjB1TyGmXvN+MaQcaK/0wCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzEuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzEuZXhhbXBsZS5jb20wHhcNMjExMTI1MDExOTAwWhcNMzExMTIzMDExOTAw\nWjBsMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzEPMA0GA1UECxMGY2xpZW50MR8wHQYDVQQDDBZBZG1pbkBv\ncmcxLmV4YW1wbGUuY29tMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEVYrjCoTa\nxEWE5zRUVVAldDdeGOMccoxjNyK3Tcd6SpkMB28xgJsnRhYrj5eEeEFxq5DFhPRQ\nw6bdZOhCYz9mf6NNMEswDgYDVR0PAQH/BAQDAgeAMAwGA1UdEwEB/wQCMAAwKwYD\nVR0jBCQwIoAgUggJW3txP1Y680v1c15t4XsrvsHdVt6kg5IydAPKIocwCgYIKoZI\nzj0EAwIDRwAwRAIgSyqASkM0FQLiMzOigqfABmxwaPoAFJGsbO57X+cIyWYCID5r\nJHGBlxjY1XNu7+jr4taRw/rQuK204WBVu6YCP2Be\n-----END CERTIFICATE-----\n\022\030*\271\216\273\002\\\022\tre\326\362v/\246\371z\356\274\035\0306\3036\022\213\026\n\210\026\n\323\006\n\266\006\n\007Org1MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKjCCAdGgAwIBAgIRAOYXjB1TyGmXvN+MaQcaK/0wCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzEuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzEuZXhhbXBsZS5jb20wHhcNMjExMTI1MDExOTAwWhcNMzExMTIzMDExOTAw\nWjBsMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzEPMA0GA1UECxMGY2xpZW50MR8wHQYDVQQDDBZBZG1pbkBv\ncmcxLmV4YW1wbGUuY29tMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEVYrjCoTa\nxEWE5zRUVVAldDdeGOMccoxjNyK3Tcd6SpkMB28xgJsnRhYrj5eEeEFxq5DFhPRQ\nw6bdZOhCYz9mf6NNMEswDgYDVR0PAQH/BAQDAgeAMAwGA1UdEwEB/wQCMAAwKwYD\nVR0jBCQwIoAgUggJW3txP1Y680v1c15t4XsrvsHdVt6kg5IydAPKIocwCgYIKoZI\nzj0EAwIDRwAwRAIgSyqASkM0FQLiMzOigqfABmxwaPoAFJGsbO57X+cIyWYCID5r\nJHGBlxjY1XNu7+jr4taRw/rQuK204WBVu6YCP2Be\n-----END CERTIFICATE-----\n\022\030*\271\216\273\002\\\022\tre\326\362v/\246\371z\356\274\035\0306\3036\022\257\017\n\"\n \n\036\010\001\022\006\022\004mycc\032\022\n\006invoke\n\001a\n\001b\n\00210\022\210\017\n}\n \327\314j\014\243\314A4h[\353-h\232\207;\302*\305\350T\374\320\214\325\333\213Q\232\235\240\314\022Y\nE\022\024\n\004lscc\022\014\n\n\n\004mycc\022\002\010\003\022-\n\004mycc\022%\n\007\n\001a\022\002\010\003\n\007\n\001b\022\002\010\003\032\007\n\001a\032\00290\032\010\n\001b\032\003210\032\003\010\310\001\"\013\022\004mycc\032\0031.0\022\202\007\n\266\006\n\007Org1MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKTCCAc+gAwIBAgIRAMxPmHXrWH4pFNx33A3BFuswCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzEuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzEuZXhhbXBsZS5jb20wHhcNMjExMTI1MDExOTAwWhcNMzExMTIzMDExOTAw\nWjBqMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzENMAsGA1UECxMEcGVlcjEfMB0GA1UEAxMWcGVlcjAub3Jn\nMS5leGFtcGxlLmNvbTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABA7R7EvbUEak\nIQ8Q0Yn3fNSJlG0xi0tOU8XRXoV5ZczB+fY3WbTpzRG/hhNE45zaUInP2eh+bGwv\ntcY1EGFKw9WjTTBLMA4GA1UdDwEB/wQEAwIHgDAMBgNVHRMBAf8EAjAAMCsGA1Ud\nIwQkMCKAIFIICVt7cT9WOvNL9XNebeF7K77B3VbepIOSMnQDyiKHMAoGCCqGSM49\nBAMCA0gAMEUCIQD7Epgj12JuKu7M4NQ6D3GyDrKBfaSWOrgM9bbY6iJUDgIgN2d2\nnZCaFvu6QDG/Kk3FBg6aF2M2ZZ/2X7MOg/k4YbA=\n-----END CERTIFICATE-----\n\022G0E\002!\000\340v\321ET\225\330\220f\007]\250m:\366\3167\245{\326\035\354\273J=\273\313\3514Xu:\002 \002\017\366y_^\330\013^*\221\253)9\305\006\333$\317\275\316\0228\003\031C\344\307Z\324\315\237\022\201\007\n\266\006\n\007Org2MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKTCCAc+gAwIBAgIRALVUgZt5qdpmSUDNh5EABbMwCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzIuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzIuZXhhbXBsZS5jb20wHhcNMjExMTI1MDExOTAwWhcNMzExMTIzMDExOTAw\nWjBqMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzENMAsGA1UECxMEcGVlcjEfMB0GA1UEAxMWcGVlcjAub3Jn\nMi5leGFtcGxlLmNvbTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABOCWoh/kXAKe\ns3y4Y+4ZB6tq4mYwsZhePHdHqXbsthnQ6ra87H3bJp1w/QL0BCjh/pPUFFKDZSqk\nWg4t2BB9eyOjTTBLMA4GA1UdDwEB/wQEAwIHgDAMBgNVHRMBAf8EAjAAMCsGA1Ud\nIwQkMCKAIJOWSkw4YbR7/TtpSzr4yr2KvplbbBcF5S9POxOX0lUXMAoGCCqGSM49\nBAMCA0gAMEUCIQDxxvQqf8w7svfUyGy4R/frJycb7rXngTooMef3DM8aYQIgA4Im\nzktJeVxb+1oTmgW5VdlkiPmNGTt0ECoIPbrWTOE=\n-----END CERTIFICATE-----\n\022F0D\002 \tT*vbQ\347\251X\300\032\257$\032\024x\210\217\000 +-9\331'{D\222P\031\246\204\002 ,\344\0317\261)\365`\000\020\366\274\344\010\274*A6\261\256\236:\266!)J\037\247\242G\346\266\022F0D\002 S#5\231\022\255\n\013\366w\337\273;J\344\010r\220\370*\330-\030\3725\204\021 H&\r\362\002 \004\267e\361\020t\204\032\"\236\275\372\236\331_\230l%$\365\262&\245\367~\220Q\006\3500FE" > metadata:<metadata:"\022\371\006\n\255\006\n\220\006\n\nOrdererMSP\022\201\006-----BEGIN CERTIFICATE-----\nMIICCzCCAbKgAwIBAgIQTACGv8V29V/UnWBgVBwKODAKBggqhkjOPQQDAjBpMQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEUMBIGA1UEChMLZXhhbXBsZS5jb20xFzAVBgNVBAMTDmNhLmV4YW1w\nbGUuY29tMB4XDTIxMTEyNTAxMTkwMFoXDTMxMTEyMzAxMTkwMFowWDELMAkGA1UE\nBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBGcmFuY2lz\nY28xHDAaBgNVBAMTE29yZGVyZXIuZXhhbXBsZS5jb20wWTATBgcqhkjOPQIBBggq\nhkjOPQMBBwNCAAQM0B5ZrFkAfMUdPcAbiGFkib+3aPYZfyZW7cC2Nbe3iHGKhJGE\nEbFwsrHLxi3zOcTV2ff+HZnY3fNUGxtVrDQqo00wSzAOBgNVHQ8BAf8EBAMCB4Aw\nDAYDVR0TAQH/BAIwADArBgNVHSMEJDAigCBggxFFk0YKcPjVD0djn57iHGqgti71\nuMcUvMT2zYJMAjAKBggqhkjOPQQDAgNHADBEAiAe9CjkOh3gJcRRWqmfu+DH1odZ\neDTiCK4AelnE2VIOFwIgNJxll2PbxGk54UFjcAwGJ7lmqmUnnFMHzfPxbV017Q4=\n-----END CERTIFICATE-----\n\022\030\013\030\336\214\362\224\245\235\234\302\202\005!\251\321\361\300\351i\306\272H\206U\022G0E\002!\000\324\256w\262\306\375A;\222H\r\241\357Ur\2334\017L\274\205\331B=\271/\034\023\346\224x\252\002 \0308q\217rN\220\375z\335\240\214\231d\2127r2\357\231\337%\245\247\312\331\3100\3363\022\242" metadata:"\n\002\010\002\022\371\006\n\255\006\n\220\006\n\nOrdererMSP\022\201\006-----BEGIN CERTIFICATE-----\nMIICCzCCAbKgAwIBAgIQTACGv8V29V/UnWBgVBwKODAKBggqhkjOPQQDAjBpMQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEUMBIGA1UEChMLZXhhbXBsZS5jb20xFzAVBgNVBAMTDmNhLmV4YW1w\nbGUuY29tMB4XDTIxMTEyNTAxMTkwMFoXDTMxMTEyMzAxMTkwMFowWDELMAkGA1UE\nBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBGcmFuY2lz\nY28xHDAaBgNVBAMTE29yZGVyZXIuZXhhbXBsZS5jb20wWTATBgcqhkjOPQIBBggq\nhkjOPQMBBwNCAAQM0B5ZrFkAfMUdPcAbiGFkib+3aPYZfyZW7cC2Nbe3iHGKhJGE\nEbFwsrHLxi3zOcTV2ff+HZnY3fNUGxtVrDQqo00wSzAOBgNVHQ8BAf8EBAMCB4Aw\nDAYDVR0TAQH/BAIwADArBgNVHSMEJDAigCBggxFFk0YKcPjVD0djn57iHGqgti71\nuMcUvMT2zYJMAjAKBggqhkjOPQQDAgNHADBEAiAe9CjkOh3gJcRRWqmfu+DH1odZ\neDTiCK4AelnE2VIOFwIgNJxll2PbxGk54UFjcAwGJ7lmqmUnnFMHzfPxbV017Q4=\n-----END CERTIFICATE-----\n\022\030\336\201\377\001\207\3675*\205\247\234j!\247a\357V\3664*a\353Do\022G0E\002!\000\360 q\237Xbx\272\ns>\005\277\004\np.(\365\004M9\330\310\306E\0377r\357,_\002 WTD\034,\204\277\240Q\344\344H\325\267\217\202\366+\203N\337\243i\335T\211\014\000\221\254$\376" metadata:"" metadata:"" > , 0, 0)===========================
	 */
	va, err := vscc.extractValidationArtifacts(block, txPosition, actionPosition)
	logger.Info("=============va===========",va)
	// &{[18 20 10 4 108 115 99 99 18 12 10 10 10 4 109 121 99 99 18 2 8 3 18 45 10 4 109 121 99 99 18 37 10 7 10 1 97 18 2 8 3 10 7 10 1 98 18 2 8 3 26 7 10 1 97 26 2 57 48 26 8 10 1 98 26 3 50 49 48] [10 32 215 204 106 12 163 204 65 52 104 91 235 45 104 154 135 59 194 42 197 232 84 252 208 140 213 219 139 81 154 157 160 204 18 89 10 69 18 20 10 4 108 115 99 99 18 12 10 10 10 4 109 121 99 99 18 2 8 3 18 45 10 4 109 121 99 99 18 37 10 7 10 1 97 18 2 8 3 10 7 10 1 98 18 2 8 3 26 7 10 1 97 26 2 57 48 26 8 10 1 98 26 3 50 49 48 26 3 8 200 1 34 11 18 4 109 121 99 99 26 3 49 46 48] [0xc00324b540 0xc00324b590] 0xc00300a630 0xc00324b3b0 0xc003691440 0xc0036914c0}
	logger.Info("=============err===========",err)//nil
	if err != nil {
		vscc.stateBasedValidator.PostValidate(namespace, block.Header.Number, uint64(txPosition), err)
		return policyErr(err)
	}
	logger.Infof("=============vscc.stateBasedValidator.Validate(namespace:%v, block.Header.Number::%v, uint64(txPosition):%v, va.rwset:%v, va.prp:%v, policyBytes:%v, va.endorsements:%v)==========",
		namespace,block.Header.Number,uint64(txPosition),va.rwset,va.prp,policyBytes,va.endorsements)
	/*
	=============vscc.stateBasedValidator.Validate(namespace:mycc, block.Header.Number::4, uint64(txPosition):0, va.rwset:[18 20 10 4 108 115 99 99 18 12 10 10 10 4 109 121 99 99 18 2 8 3 18 45 10 4 109 121 99 99 18 37 10 7 10 1 97 18 2 8 3 10 7 10 1 98 18 2 8 3 26 7 10 1 97 26 2 57 48 26 8 10 1 98 26 3 50 49 48], va.prp:[10 32 215 204 106 12 163 204 65 52 104 91 235 45 104 154 135 59 194 42 197 232 84 252 208 140 213 219 139 81 154 157 160 204 18 89 10 69 18 20 10 4 108 115 99 99 18 12 10 10 10 4 109 121 99 99 18 2 8 3 18 45 10 4 109 121 99 99 18 37 10 7 10 1 97 18 2 8 3 10 7 10 1 98 18 2 8 3 26 7 10 1 97 26 2 57 48 26 8 10 1 98 26 3 50 49 48 26 3 8 200 1 34 11 18 4 109 121 99 99 26 3 49 46 48], policyBytes:[18 12 18 10 8 2 18 2 8 0 18 2 8 1 26 13 18 11 10 7 79 114 103 49 77 83 80 16 3 26 13 18 11 10 7 79 114 103 50 77 83 80 16 3], va.endorsements:[endorser:"\n\007Org1MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKTCCAc+gAwIBAgIRAMxPmHXrWH4pFNx33A3BFuswCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzEuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzEuZXhhbXBsZS5jb20wHhcNMjExMTI1MDExOTAwWhcNMzExMTIzMDExOTAw\nWjBqMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzENMAsGA1UECxMEcGVlcjEfMB0GA1UEAxMWcGVlcjAub3Jn\nMS5leGFtcGxlLmNvbTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABA7R7EvbUEak\nIQ8Q0Yn3fNSJlG0xi0tOU8XRXoV5ZczB+fY3WbTpzRG/hhNE45zaUInP2eh+bGwv\ntcY1EGFKw9WjTTBLMA4GA1UdDwEB/wQEAwIHgDAMBgNVHRMBAf8EAjAAMCsGA1Ud\nIwQkMCKAIFIICVt7cT9WOvNL9XNebeF7K77B3VbepIOSMnQDyiKHMAoGCCqGSM49\nBAMCA0gAMEUCIQD7Epgj12JuKu7M4NQ6D3GyDrKBfaSWOrgM9bbY6iJUDgIgN2d2\nnZCaFvu6QDG/Kk3FBg6aF2M2ZZ/2X7MOg/k4YbA=\n-----END CERTIFICATE-----\n" signature:"0E\002!\000\340v\321ET\225\330\220f\007]\250m:\366\3167\245{\326\035\354\273J=\273\313\3514Xu:\002 \002\017\366y_^\330\013^*\221\253)9\305\006\333$\317\275\316\0228\003\031C\344\307Z\324\315\237"  endorser:"\n\007Org2MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKTCCAc+gAwIBAgIRALVUgZt5qdpmSUDNh5EABbMwCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzIuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzIuZXhhbXBsZS5jb20wHhcNMjExMTI1MDExOTAwWhcNMzExMTIzMDExOTAw\nWjBqMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzENMAsGA1UECxMEcGVlcjEfMB0GA1UEAxMWcGVlcjAub3Jn\nMi5leGFtcGxlLmNvbTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABOCWoh/kXAKe\ns3y4Y+4ZB6tq4mYwsZhePHdHqXbsthnQ6ra87H3bJp1w/QL0BCjh/pPUFFKDZSqk\nWg4t2BB9eyOjTTBLMA4GA1UdDwEB/wQEAwIHgDAMBgNVHRMBAf8EAjAAMCsGA1Ud\nIwQkMCKAIJOWSkw4YbR7/TtpSzr4yr2KvplbbBcF5S9POxOX0lUXMAoGCCqGSM49\nBAMCA0gAMEUCIQDxxvQqf8w7svfUyGy4R/frJycb7rXngTooMef3DM8aYQIgA4Im\nzktJeVxb+1oTmgW5VdlkiPmNGTt0ECoIPbrWTOE=\n-----END CERTIFICATE-----\n" signature:"0D\002 \tT*vbQ\347\251X\300\032\257$\032\024x\210\217\000 +-9\331'{D\222P\031\246\204\002 ,\344\0317\261)\365`\000\020\366\274\344\010\274*A6\261\256\236:\266!)J\037\247\242G\346\266" ])==========
	 */
	txverr := vscc.stateBasedValidator.Validate(namespace, block.Header.Number, uint64(txPosition), va.rwset, va.prp, policyBytes, va.endorsements)
	logger.Infof("=========================================================err:%v=======================================================",err)
	if txverr != nil {
		//VSCC error: stateBasedValidator.Validate failed, err validation of endorsement policy for chaincode mycc in tx 5:0 failed: signature set did not satisfy policy
		logger.Errorf("VSCC error: stateBasedValidator.Validate failed, err %s", txverr)
		vscc.stateBasedValidator.PostValidate(namespace, block.Header.Number, uint64(txPosition), txverr)
		return txverr
	}


	logger.Info("====================namespace==================",namespace)
	logger.Info("====================lscc==================","lscc")
	// do some extra validation that is specific to lscc
	if namespace == "lscc" {
		logger.Info("VSCC info: doing special validation for LSCC")
		err := vscc.ValidateLSCCInvocation(va.chdr.ChannelId, va.env, va.cap, va.payl, vscc.capabilities)
		if err != nil {
			logger.Errorf("VSCC error: ValidateLSCCInvocation failed, err %s", err)
			vscc.stateBasedValidator.PostValidate(namespace, block.Header.Number, uint64(txPosition), err)
			return err
		}
	}

	vscc.stateBasedValidator.PostValidate(namespace, block.Header.Number, uint64(txPosition), nil)
	return nil
}

func policyErr(err error) *commonerrors.VSCCEndorsementPolicyError {
	return &commonerrors.VSCCEndorsementPolicyError{
		Err: err,
	}
}
