/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package statebased

import (
	"fmt"
	"sync"

	commonerrors "github.com/hyperledger/fabric/common/errors"
	"github.com/hyperledger/fabric/core/handlers/validation/api/policies"
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/ledger/kvledger/txmgmt/rwsetutil"
	"github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric/protos/peer"
	"github.com/hyperledger/fabric/protos/utils"
	"github.com/pkg/errors"
)

/**********************************************************************************************************/
/**********************************************************************************************************/

type policyChecker struct {
	someEPChecked bool
	ccEPChecked   bool
	vpmgr         KeyLevelValidationParameterManager
	policySupport validation.PolicyEvaluator
	ccEP          []byte
	signatureSet  []*common.SignedData
}

func (p *policyChecker) checkCCEPIfCondition(cc string, blockNum, txNum uint64, condition bool) commonerrors.TxValidationError {
	logger.Info("=============func (p *policyChecker) checkCCEPIfCondition(cc string, blockNum, txNum uint64, condition bool) commonerrors.TxValidationError=====================")

	logger.Info("======condition=======", condition) //false
	if condition {
		return nil
	}

	// validate against cc ep
	logger.Info("============err := p.policySupport.Evaluate(p.ccEP, p.signatureSet)======================")
	err := p.policySupport.Evaluate(p.ccEP, p.signatureSet)
	if err != nil {
		return policyErr(errors.Wrapf(err, "validation of endorsement policy for chaincode %s in tx %d:%d failed", cc, blockNum, txNum))
	}

	p.ccEPChecked = true
	p.someEPChecked = true
	return nil
}

func (p *policyChecker) checkCCEPIfNotChecked(cc string, blockNum, txNum uint64) commonerrors.TxValidationError {
	logger.Info("==============func (p *policyChecker) checkCCEPIfNotChecked(cc string, blockNum, txNum uint64) commonerrors.TxValidationError=====================")
	return p.checkCCEPIfCondition(cc, blockNum, txNum, p.ccEPChecked)
}

func (p *policyChecker) checkCCEPIfNoEPChecked(cc string, blockNum, txNum uint64) commonerrors.TxValidationError {
	return p.checkCCEPIfCondition(cc, blockNum, txNum, p.someEPChecked)
}

func (p *policyChecker) checkSBAndCCEP(cc, coll, key string, blockNum, txNum uint64) commonerrors.TxValidationError {
	logger.Info("========func (p *policyChecker) checkSBAndCCEP(cc, coll, key string, blockNum, txNum uint64) commonerrors.TxValidationError {======================================")
	// see if there is a key-level validation parameter for this key
	vp, err := p.vpmgr.GetValidationParameterForKey(cc, coll, key, blockNum, txNum)
	logger.Info("===vp===", vp)   //[]
	logger.Info("===err===", err) // nil
	if err != nil {
		// error handling for GetValidationParameterForKey follows this rationale:
		switch err := errors.Cause(err).(type) {
		// 1) if there is a conflict because validation params have been updated
		//    by another transaction in this block, we will get ValidationParameterUpdatedError.
		//    This should lead to invalidating the transaction by calling policyErr
		case *ValidationParameterUpdatedError:
			return policyErr(err)
		// 2) if the ledger returns "determinstic" errors, that is, errors that
		//    every peer in the channel will also return (such as errors linked to
		//    an attempt to retrieve metadata from a non-defined collection) should be
		//    logged and ignored. The ledger will take the most appropriate action
		//    when performing its side of the validation.
		case *ledger.CollConfigNotDefinedError, *ledger.InvalidCollNameError:
			logger.Warningf(errors.WithMessage(err, "skipping key-level validation").Error())
			err = nil
		// 3) any other type of error should return an execution failure which will
		//    lead to halting the processing on this channel. Note that any non-categorized
		//    deterministic error would be caught by the default and would lead to
		//    a processing halt. This would certainly be a bug, but - in the absence of a
		//    single, well-defined deterministic error returned by the ledger, it is
		//    best to err on the side of caution and rather halt processing (because a
		//    deterministic error is treated like an I/O one) rather than risking a fork
		//    (in case an I/O error is treated as a deterministic one).
		default:
			return &commonerrors.VSCCExecutionFailureError{
				Err: err,
			}
		}
	}

	// if no key-level validation parameter has been specified, the regular cc endorsement policy needs to hold
	if len(vp) == 0 {
		//invoke: go here
		logger.Info("====================if len(vp) == 0==================")
		return p.checkCCEPIfNotChecked(cc, blockNum, txNum)
	}

	// validate against key-level vp
	err = p.policySupport.Evaluate(vp, p.signatureSet)
	if err != nil {
		return policyErr(errors.Wrapf(err, "validation of key %s (coll'%s':ns'%s') in tx %d:%d failed", key, coll, cc, blockNum, txNum))
	}

	p.someEPChecked = true

	return nil
}

/**********************************************************************************************************/
/**********************************************************************************************************/

type blockDependency struct {
	mutex     sync.Mutex
	blockNum  uint64
	txDepOnce []sync.Once
}

// KeyLevelValidator implements per-key level ep validation
type KeyLevelValidator struct {
	vpmgr         KeyLevelValidationParameterManager
	policySupport validation.PolicyEvaluator
	blockDep      blockDependency
}

func NewKeyLevelValidator(policySupport validation.PolicyEvaluator, vpmgr KeyLevelValidationParameterManager) *KeyLevelValidator {
	return &KeyLevelValidator{
		vpmgr:         vpmgr,
		policySupport: policySupport,
		blockDep:      blockDependency{},
	}
}

func (klv *KeyLevelValidator) invokeOnce(block *common.Block, txnum uint64) *sync.Once {
	klv.blockDep.mutex.Lock()
	defer klv.blockDep.mutex.Unlock()

	if klv.blockDep.blockNum != block.Header.Number {
		klv.blockDep.blockNum = block.Header.Number
		klv.blockDep.txDepOnce = make([]sync.Once, len(block.Data.Data))
	}

	return &klv.blockDep.txDepOnce[txnum]
}

func (klv *KeyLevelValidator) extractDependenciesForTx(blockNum, txNum uint64, envelopeBytes []byte) {
	env, err := utils.GetEnvelopeFromBlock(envelopeBytes)
	if err != nil {
		logger.Warningf("while executing GetEnvelopeFromBlock got error '%s', skipping tx at height (%d,%d)", err, blockNum, txNum)
		return
	}

	payl, err := utils.GetPayload(env)
	if err != nil {
		logger.Warningf("while executing GetPayload got error '%s', skipping tx at height (%d,%d)", err, blockNum, txNum)
		return
	}

	tx, err := utils.GetTransaction(payl.Data)
	if err != nil {
		logger.Warningf("while executing GetTransaction got error '%s', skipping tx at height (%d,%d)", err, blockNum, txNum)
		return
	}

	cap, err := utils.GetChaincodeActionPayload(tx.Actions[0].Payload)
	if err != nil {
		logger.Warningf("while executing GetChaincodeActionPayload got error '%s', skipping tx at height (%d,%d)", err, blockNum, txNum)
		return
	}

	pRespPayload, err := utils.GetProposalResponsePayload(cap.Action.ProposalResponsePayload)
	if err != nil {
		logger.Warningf("while executing GetProposalResponsePayload got error '%s', skipping tx at height (%d,%d)", err, blockNum, txNum)
		return
	}

	respPayload, err := utils.GetChaincodeAction(pRespPayload.Extension)
	if err != nil {
		logger.Warningf("while executing GetChaincodeAction got error '%s', skipping tx at height (%d,%d)", err, blockNum, txNum)
		return
	}

	klv.vpmgr.ExtractValidationParameterDependency(blockNum, txNum, respPayload.Results)
}

// PreValidate implements the function of the StateBasedValidator interface
func (klv *KeyLevelValidator) PreValidate(txNum uint64, block *common.Block) {
	logger.Info("==============================func (klv *KeyLevelValidator) PreValidate(txNum uint64, block *common.Block)===============================")
	for i := int64(txNum); i >= 0; i-- {
		txPosition := uint64(i)

		klv.invokeOnce(block, txPosition).Do(
			func() {
				klv.extractDependenciesForTx(block.Header.Number, txPosition, block.Data.Data[txPosition])
			})
	}
}

// Validate implements the function of the StateBasedValidator interface
//   vscc.stateBasedValidator.Validate(namespace:mycc,      block.Header.Number::4, uint64(txPosition):0, va.rwset:[18 20 10 4 108 115 99 99 18 12 10 10 10 4 109 121 99 99 18 2 8 3 18 45 10 4 109 121 99 99 18 37 10 7 10 1 97 18 2 8 3 10 7 10 1 98 18 2 8 3 26 7 10 1 97 26 2 57 48 26 8 10 1 98 26 3 50 49 48], va.prp:[10 32 215 204 106 12 163 204 65 52 104 91 235 45 104 154 135 59 194 42 197 232 84 252 208 140 213 219 139 81 154 157 160 204 18 89 10 69 18 20 10 4 108 115 99 99 18 12 10 10 10 4 109 121 99 99 18 2 8 3 18 45 10 4 109 121 99 99 18 37 10 7 10 1 97 18 2 8 3 10 7 10 1 98 18 2 8 3 26 7 10 1 97 26 2 57 48 26 8 10 1 98 26 3 50 49 48 26 3 8 200 1 34 11 18 4 109 121 99 99 26 3 49 46 48], policyBytes:[18 12 18 10 8 2 18 2 8 0 18 2 8 1 26 13 18 11 10 7 79 114 103 49 77 83 80 16 3 26 13 18 11 10 7 79 114 103 50 77 83 80 16 3], va.endorsements:[endorser:"\n\007Org1MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKTCCAc+gAwIBAgIRAMxPmHXrWH4pFNx33A3BFuswCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzEuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzEuZXhhbXBsZS5jb20wHhcNMjExMTI1MDExOTAwWhcNMzExMTIzMDExOTAw\nWjBqMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzENMAsGA1UECxMEcGVlcjEfMB0GA1UEAxMWcGVlcjAub3Jn\nMS5leGFtcGxlLmNvbTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABA7R7EvbUEak\nIQ8Q0Yn3fNSJlG0xi0tOU8XRXoV5ZczB+fY3WbTpzRG/hhNE45zaUInP2eh+bGwv\ntcY1EGFKw9WjTTBLMA4GA1UdDwEB/wQEAwIHgDAMBgNVHRMBAf8EAjAAMCsGA1Ud\nIwQkMCKAIFIICVt7cT9WOvNL9XNebeF7K77B3VbepIOSMnQDyiKHMAoGCCqGSM49\nBAMCA0gAMEUCIQD7Epgj12JuKu7M4NQ6D3GyDrKBfaSWOrgM9bbY6iJUDgIgN2d2\nnZCaFvu6QDG/Kk3FBg6aF2M2ZZ/2X7MOg/k4YbA=\n-----END CERTIFICATE-----\n" signature:"0E\002!\000\340v\321ET\225\330\220f\007]\250m:\366\3167\245{\326\035\354\273J=\273\313\3514Xu:\002 \002\017\366y_^\330\013^*\221\253)9\305\006\333$\317\275\316\0228\003\031C\344\307Z\324\315\237"  endorser:"\n\007Org2MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKTCCAc+gAwIBAgIRALVUgZt5qdpmSUDNh5EABbMwCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzIuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzIuZXhhbXBsZS5jb20wHhcNMjExMTI1MDExOTAwWhcNMzExMTIzMDExOTAw\nWjBqMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzENMAsGA1UECxMEcGVlcjEfMB0GA1UEAxMWcGVlcjAub3Jn\nMi5leGFtcGxlLmNvbTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABOCWoh/kXAKe\ns3y4Y+4ZB6tq4mYwsZhePHdHqXbsthnQ6ra87H3bJp1w/QL0BCjh/pPUFFKDZSqk\nWg4t2BB9eyOjTTBLMA4GA1UdDwEB/wQEAwIHgDAMBgNVHRMBAf8EAjAAMCsGA1Ud\nIwQkMCKAIJOWSkw4YbR7/TtpSzr4yr2KvplbbBcF5S9POxOX0lUXMAoGCCqGSM49\nBAMCA0gAMEUCIQDxxvQqf8w7svfUyGy4R/frJycb7rXngTooMef3DM8aYQIgA4Im\nzktJeVxb+1oTmgW5VdlkiPmNGTt0ECoIPbrWTOE=\n-----END CERTIFICATE-----\n" signature:"0D\002 \tT*vbQ\347\251X\300\032\257$\032\024x\210\217\000 +-9\331'{D\222P\031\246\204\002 ,\344\0317\261)\365`\000\020\366\274\344\010\274*A6\261\256\236:\266!)J\037\247\242G\346\266" ])==========
func (klv *KeyLevelValidator) Validate(         cc string,        blockNum,               txNum uint64,          rwsetBytes,                                                                                                                                                                                           prp,                                                                                                                                                                                                                                                                                                                                                                                                  ccEP []byte,                                                                                                                              endorsements []*peer.Endorsement) commonerrors.TxValidationError {
	logger.Info("=======func (klv *KeyLevelValidator) Validate(cc string, blockNum, txNum uint64, rwsetBytes, prp, ccEP []byte, endorsements []*peer.Endorsement) commonerrors.TxValidationError ========================")
	// construct signature set
	signatureSet := []*common.SignedData{}
	logger.Infof("======================endorsements:%v============", endorsements)
	/*
	======================endorsements:[endorser:"\n\007Org1MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKTCCAc+gAwIBAgIRAMxPmHXrWH4pFNx33A3BFuswCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzEuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzEuZXhhbXBsZS5jb20wHhcNMjExMTI1MDExOTAwWhcNMzExMTIzMDExOTAw\nWjBqMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzENMAsGA1UECxMEcGVlcjEfMB0GA1UEAxMWcGVlcjAub3Jn\nMS5leGFtcGxlLmNvbTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABA7R7EvbUEak\nIQ8Q0Yn3fNSJlG0xi0tOU8XRXoV5ZczB+fY3WbTpzRG/hhNE45zaUInP2eh+bGwv\ntcY1EGFKw9WjTTBLMA4GA1UdDwEB/wQEAwIHgDAMBgNVHRMBAf8EAjAAMCsGA1Ud\nIwQkMCKAIFIICVt7cT9WOvNL9XNebeF7K77B3VbepIOSMnQDyiKHMAoGCCqGSM49\nBAMCA0gAMEUCIQD7Epgj12JuKu7M4NQ6D3GyDrKBfaSWOrgM9bbY6iJUDgIgN2d2\nnZCaFvu6QDG/Kk3FBg6aF2M2ZZ/2X7MOg/k4YbA=\n-----END CERTIFICATE-----\n" signature:"0E\002!\000\340v\321ET\225\330\220f\007]\250m:\366\3167\245{\326\035\354\273J=\273\313\3514Xu:\002 \002\017\366y_^\330\013^*\221\253)9\305\006\333$\317\275\316\0228\003\031C\344\307Z\324\315\237"  endorser:"\n\007Org2MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKTCCAc+gAwIBAgIRALVUgZt5qdpmSUDNh5EABbMwCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzIuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzIuZXhhbXBsZS5jb20wHhcNMjExMTI1MDExOTAwWhcNMzExMTIzMDExOTAw\nWjBqMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzENMAsGA1UECxMEcGVlcjEfMB0GA1UEAxMWcGVlcjAub3Jn\nMi5leGFtcGxlLmNvbTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABOCWoh/kXAKe\ns3y4Y+4ZB6tq4mYwsZhePHdHqXbsthnQ6ra87H3bJp1w/QL0BCjh/pPUFFKDZSqk\nWg4t2BB9eyOjTTBLMA4GA1UdDwEB/wQEAwIHgDAMBgNVHRMBAf8EAjAAMCsGA1Ud\nIwQkMCKAIJOWSkw4YbR7/TtpSzr4yr2KvplbbBcF5S9POxOX0lUXMAoGCCqGSM49\nBAMCA0gAMEUCIQDxxvQqf8w7svfUyGy4R/frJycb7rXngTooMef3DM8aYQIgA4Im\nzktJeVxb+1oTmgW5VdlkiPmNGTt0ECoIPbrWTOE=\n-----END CERTIFICATE-----\n" signature:"0D\002 \tT*vbQ\347\251X\300\032\257$\032\024x\210\217\000 +-9\331'{D\222P\031\246\204\002 ,\344\0317\261)\365`\000\020\366\274\344\010\274*A6\261\256\236:\266!)J\037\247\242G\346\266" ]============

			invoke: fail Org1MSP
		======================endorsements:[endorser:"\n\007Org1MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKTCCAc+gAwIBAgIRAPT9GCsi6R6KydezMCP5w7owCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzEuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzEuZXhhbXBsZS5jb20wHhcNMjExMTI0MDYzNTAwWhcNMzExMTIyMDYzNTAw\nWjBqMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzENMAsGA1UECxMEcGVlcjEfMB0GA1UEAxMWcGVlcjAub3Jn\nMS5leGFtcGxlLmNvbTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABBBuCto/n39v\nyIdt6qszIz29542VcqYr6zUbSHw3qWj7+mzBgjItINazQNmElDz7mboivqtBty0U\nnxDUzYf6YJ+jTTBLMA4GA1UdDwEB/wQEAwIHgDAMBgNVHRMBAf8EAjAAMCsGA1Ud\nIwQkMCKAIKCzA8E4BkVt4gyxDR1Gskm535N4GqC7hVG2oYV2dPkBMAoGCCqGSM49\nBAMCA0gAMEUCIQCWHhPgYMakCBiBKu7k15NvmTNz+ngqXUG+UfeJJIBkewIgT4sJ\n49uUvv6x5XSzHhfzvlUW9neWIsuIEbnwyg0Bhz0=\n-----END CERTIFICATE-----\n" signature:"0D\002 a\333Oa!o\367\205\020am\237\312\250\331S\372\232U\310\221d*\222\t\301\002\001\0321I\034\002 U\254\345+\320c}\3347\221\354\217\202\244\"$t\252_\272q\336lo\t\326\036\225\237.\232\277" ]============

		invoke: success Org1MSP Org2MSP
		======================endorsements:[endorser:"\n\007Org1MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKTCCAc+gAwIBAgIRAPT9GCsi6R6KydezMCP5w7owCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzEuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzEuZXhhbXBsZS5jb20wHhcNMjExMTI0MDYzNTAwWhcNMzExMTIyMDYzNTAw\nWjBqMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzENMAsGA1UECxMEcGVlcjEfMB0GA1UEAxMWcGVlcjAub3Jn\nMS5leGFtcGxlLmNvbTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABBBuCto/n39v\nyIdt6qszIz29542VcqYr6zUbSHw3qWj7+mzBgjItINazQNmElDz7mboivqtBty0U\nnxDUzYf6YJ+jTTBLMA4GA1UdDwEB/wQEAwIHgDAMBgNVHRMBAf8EAjAAMCsGA1Ud\nIwQkMCKAIKCzA8E4BkVt4gyxDR1Gskm535N4GqC7hVG2oYV2dPkBMAoGCCqGSM49\nBAMCA0gAMEUCIQCWHhPgYMakCBiBKu7k15NvmTNz+ngqXUG+UfeJJIBkewIgT4sJ\n49uUvv6x5XSzHhfzvlUW9neWIsuIEbnwyg0Bhz0=\n-----END CERTIFICATE-----\n" signature:"0D\002 Q\312=\344\330\343n\275g\236\271\245\006\357\034K\324\272\335\207\234}\264\273\214!,8\320@x\310\002 g\272E\311\272\232\272L\201f\342\273\366\375\277\323\025\2448QVW\002\377+\370\272\005\221\211\272x"  endorser:"\n\007Org2MSP\022\246\006-----BEGIN CERTIFICATE-----\nMIICJzCCAc6gAwIBAgIQAYM1JvH0ECeaytVHR5tpFzAKBggqhkjOPQQDAjBzMQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEZMBcGA1UEChMQb3JnMi5leGFtcGxlLmNvbTEcMBoGA1UEAxMTY2Eu\nb3JnMi5leGFtcGxlLmNvbTAeFw0yMTExMjQwNjM1MDBaFw0zMTExMjIwNjM1MDBa\nMGoxCzAJBgNVBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1T\nYW4gRnJhbmNpc2NvMQ0wCwYDVQQLEwRwZWVyMR8wHQYDVQQDExZwZWVyMC5vcmcy\nLmV4YW1wbGUuY29tMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEQLu76lXADvRE\nvEYFDERvHqgRUqCEeZkWK/8RmgpWC85UIPwzkfdSkAbWIv4vPASD8x7yu9zPAzdW\nRBDLBdSScaNNMEswDgYDVR0PAQH/BAQDAgeAMAwGA1UdEwEB/wQCMAAwKwYDVR0j\nBCQwIoAg5Q79T8eiXIM+VFPkZgpoP8t4a39+qumQZNwriNSuKb0wCgYIKoZIzj0E\nAwIDRwAwRAIgPwTvbxE1bC8XbkZJJnAV97zsYGqrNLEb4GkaLp+adWgCIEhD27RU\nPjCddupl0EXwmTDotlljAQzcSzGF04hUp+mg\n-----END CERTIFICATE-----\n" signature:"0E\002!\000\266\264\227m\272\000\224\303\341\264\260\376\240\343Y\370\021\230Eq\240\314!\021\211\2304\265\217<T\025\002 8\205\310\331\353\0052\331l\255n%\352Z'@#\215(^\006\3431\347\230\305\3161\336\223\247E" ]============



	*/
	for k, endorsement := range endorsements {
		logger.Infof("=============endorsement[%v]:%v===============", k, endorsement)
		/*
			=============endorsement[0]:endorser:"\n\007Org1MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKTCCAc+gAwIBAgIRAPT9GCsi6R6KydezMCP5w7owCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzEuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzEuZXhhbXBsZS5jb20wHhcNMjExMTI0MDYzNTAwWhcNMzExMTIyMDYzNTAw\nWjBqMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzENMAsGA1UECxMEcGVlcjEfMB0GA1UEAxMWcGVlcjAub3Jn\nMS5leGFtcGxlLmNvbTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABBBuCto/n39v\nyIdt6qszIz29542VcqYr6zUbSHw3qWj7+mzBgjItINazQNmElDz7mboivqtBty0U\nnxDUzYf6YJ+jTTBLMA4GA1UdDwEB/wQEAwIHgDAMBgNVHRMBAf8EAjAAMCsGA1Ud\nIwQkMCKAIKCzA8E4BkVt4gyxDR1Gskm535N4GqC7hVG2oYV2dPkBMAoGCCqGSM49\nBAMCA0gAMEUCIQCWHhPgYMakCBiBKu7k15NvmTNz+ngqXUG+UfeJJIBkewIgT4sJ\n49uUvv6x5XSzHhfzvlUW9neWIsuIEbnwyg0Bhz0=\n-----END CERTIFICATE-----\n" signature:"0D\002 a\333Oa!o\367\205\020am\237\312\250\331S\372\232U\310\221d*\222\t\301\002\001\0321I\034\002 U\254\345+\320c}\3347\221\354\217\202\244\"$t\252_\272q\336lo\t\326\036\225\237.\232\277" ===============

			=============endorsement[0]:endorser:"\n\007Org1MSP\022\252\006-----BEGIN CERTIFICATE-----\nMIICKTCCAc+gAwIBAgIRAPT9GCsi6R6KydezMCP5w7owCgYIKoZIzj0EAwIwczEL\nMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG\ncmFuY2lzY28xGTAXBgNVBAoTEG9yZzEuZXhhbXBsZS5jb20xHDAaBgNVBAMTE2Nh\nLm9yZzEuZXhhbXBsZS5jb20wHhcNMjExMTI0MDYzNTAwWhcNMzExMTIyMDYzNTAw\nWjBqMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMN\nU2FuIEZyYW5jaXNjbzENMAsGA1UECxMEcGVlcjEfMB0GA1UEAxMWcGVlcjAub3Jn\nMS5leGFtcGxlLmNvbTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABBBuCto/n39v\nyIdt6qszIz29542VcqYr6zUbSHw3qWj7+mzBgjItINazQNmElDz7mboivqtBty0U\nnxDUzYf6YJ+jTTBLMA4GA1UdDwEB/wQEAwIHgDAMBgNVHRMBAf8EAjAAMCsGA1Ud\nIwQkMCKAIKCzA8E4BkVt4gyxDR1Gskm535N4GqC7hVG2oYV2dPkBMAoGCCqGSM49\nBAMCA0gAMEUCIQCWHhPgYMakCBiBKu7k15NvmTNz+ngqXUG+UfeJJIBkewIgT4sJ\n49uUvv6x5XSzHhfzvlUW9neWIsuIEbnwyg0Bhz0=\n-----END CERTIFICATE-----\n" signature:"0D\002 Q\312=\344\330\343n\275g\236\271\245\006\357\034K\324\272\335\207\234}\264\273\214!,8\320@x\310\002 g\272E\311\272\232\272L\201f\342\273\366\375\277\323\025\2448QVW\002\377+\370\272\005\221\211\272x" ===============
			=============endorsement[1]:endorser:"\n\007Org2MSP\022\246\006-----BEGIN CERTIFICATE-----\nMIICJzCCAc6gAwIBAgIQAYM1JvH0ECeaytVHR5tpFzAKBggqhkjOPQQDAjBzMQsw\nCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZy\nYW5jaXNjbzEZMBcGA1UEChMQb3JnMi5leGFtcGxlLmNvbTEcMBoGA1UEAxMTY2Eu\nb3JnMi5leGFtcGxlLmNvbTAeFw0yMTExMjQwNjM1MDBaFw0zMTExMjIwNjM1MDBa\nMGoxCzAJBgNVBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1T\nYW4gRnJhbmNpc2NvMQ0wCwYDVQQLEwRwZWVyMR8wHQYDVQQDExZwZWVyMC5vcmcy\nLmV4YW1wbGUuY29tMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEQLu76lXADvRE\nvEYFDERvHqgRUqCEeZkWK/8RmgpWC85UIPwzkfdSkAbWIv4vPASD8x7yu9zPAzdW\nRBDLBdSScaNNMEswDgYDVR0PAQH/BAQDAgeAMAwGA1UdEwEB/wQCMAAwKwYDVR0j\nBCQwIoAg5Q79T8eiXIM+VFPkZgpoP8t4a39+qumQZNwriNSuKb0wCgYIKoZIzj0E\nAwIDRwAwRAIgPwTvbxE1bC8XbkZJJnAV97zsYGqrNLEb4GkaLp+adWgCIEhD27RU\nPjCddupl0EXwmTDotlljAQzcSzGF04hUp+mg\n-----END CERTIFICATE-----\n" signature:"0E\002!\000\266\264\227m\272\000\224\303\341\264\260\376\240\343Y\370\021\230Eq\240\314!\021\211\2304\265\217<T\025\002 8\205\310\331\353\0052\331l\255n%\352Z'@#\215(^\006\3431\347\230\305\3161\336\223\247E" ===============

		*/
		data := make([]byte, len(prp)+len(endorsement.Endorser))
		copy(data, prp)
		copy(data[len(prp):], endorsement.Endorser)

		logger.Infof("==========Data:%v=====================", data)
		//==========Data:
		//prp [10 32 195 150 249 213 20 227 41 188 246 163 6 208 151 186 201 20 222 184 2 28 44 151 43 85 56 194 165 49 56 217 238 108 18 89 10 69 18 20 10 4 108 115 99 99 18 12 10 10 10 4 109 121 99 99 18 2 8 3 18 45 10 4 109 121 99 99 18 37 10 7 10 1 97 18 2 8 4 10 7 10 1 98 18 2 8 4 26 7 10 1 97 26 2 56 48 26 8 10 1 98 26 3 50 50 48 26 3 8 200 1 34 11 18 4 109 121 99 99 26 3 49 46 48
		//identity 10 7 79 114 103 49 77 83 80 18 170 6 45 45 45 45 45 66 69 71 73 78 32 67 69 82 84 73 70 73 67 65 84 69 45 45 45 45 45 10 77 73 73 67 75 84 67 67 65 99 43 103 65 119 73 66 65 103 73 82 65 80 84 57 71 67 115 105 54 82 54 75 121 100 101 122 77 67 80 53 119 55 111 119 67 103 89 73 75 111 90 73 122 106 48 69 65 119 73 119 99 122 69 76 10 77 65 107 71 65 49 85 69 66 104 77 67 86 86 77 120 69 122 65 82 66 103 78 86 66 65 103 84 67 107 78 104 98 71 108 109 98 51 74 117 97 87 69 120 70 106 65 85 66 103 78 86 66 65 99 84 68 86 78 104 98 105 66 71 10 99 109 70 117 89 50 108 122 89 50 56 120 71 84 65 88 66 103 78 86 66 65 111 84 69 71 57 121 90 122 69 117 90 88 104 104 98 88 66 115 90 83 53 106 98 50 48 120 72 68 65 97 66 103 78 86 66 65 77 84 69 50 78 104 10 76 109 57 121 90 122 69 117 90 88 104 104 98 88 66 115 90 83 53 106 98 50 48 119 72 104 99 78 77 106 69 120 77 84 73 48 77 68 89 122 78 84 65 119 87 104 99 78 77 122 69 120 77 84 73 121 77 68 89 122 78 84 65 119 10 87 106 66 113 77 81 115 119 67 81 89 68 86 81 81 71 69 119 74 86 85 122 69 84 77 66 69 71 65 49 85 69 67 66 77 75 81 50 70 115 97 87 90 118 99 109 53 112 89 84 69 87 77 66 81 71 65 49 85 69 66 120 77 78 10 85 50 70 117 73 69 90 121 89 87 53 106 97 88 78 106 98 122 69 78 77 65 115 71 65 49 85 69 67 120 77 69 99 71 86 108 99 106 69 102 77 66 48 71 65 49 85 69 65 120 77 87 99 71 86 108 99 106 65 117 98 51 74 110 10 77 83 53 108 101 71 70 116 99 71 120 108 76 109 78 118 98 84 66 90 77 66 77 71 66 121 113 71 83 77 52 57 65 103 69 71 67 67 113 71 83 77 52 57 65 119 69 72 65 48 73 65 66 66 66 117 67 116 111 47 110 51 57 118 10 121 73 100 116 54 113 115 122 73 122 50 57 53 52 50 86 99 113 89 114 54 122 85 98 83 72 119 51 113 87 106 55 43 109 122 66 103 106 73 116 73 78 97 122 81 78 109 69 108 68 122 55 109 98 111 105 118 113 116 66 116 121 48 85 10 110 120 68 85 122 89 102 54 89 74 43 106 84 84 66 76 77 65 52 71 65 49 85 100 68 119 69 66 47 119 81 69 65 119 73 72 103 68 65 77 66 103 78 86 72 82 77 66 65 102 56 69 65 106 65 65 77 67 115 71 65 49 85 100 10 73 119 81 107 77 67 75 65 73 75 67 122 65 56 69 52 66 107 86 116 52 103 121 120 68 82 49 71 115 107 109 53 51 53 78 52 71 113 67 55 104 86 71 50 111 89 86 50 100 80 107 66 77 65 111 71 67 67 113 71 83 77 52 57 10 66 65 77 67 65 48 103 65 77 69 85 67 73 81 67 87 72 104 80 103 89 77 97 107 67 66 105 66 75 117 55 107 49 53 78 118 109 84 78 122 43 110 103 113 88 85 71 43 85 102 101 74 74 73 66 107 101 119 73 103 84 52 115 74 10 52 57 117 85 118 118 54 120 53 88 83 122 72 104 102 122 118 108 85 87 57 110 101 87 73 115 117 73 69 98 110 119 121 103 48 66 104 122 48 61 10 45 45 45 45 45 69 78 68 32 67 69 82 84 73 70 73 67 65 84 69 45 45 45 45 45 10]=====================
		logger.Infof("==========Identity:%v=====================", endorsement.Endorser)
		//==========Identity:[10 7 79 114 103 49 77 83 80 18 170 6 45 45 45 45 45 66 69 71 73 78 32 67 69 82 84 73 70 73 67 65 84 69 45 45 45 45 45 10 77 73 73 67 75 84 67 67 65 99 43 103 65 119 73 66 65 103 73 82 65 80 84 57 71 67 115 105 54 82 54 75 121 100 101 122 77 67 80 53 119 55 111 119 67 103 89 73 75 111 90 73 122 106 48 69 65 119 73 119 99 122 69 76 10 77 65 107 71 65 49 85 69 66 104 77 67 86 86 77 120 69 122 65 82 66 103 78 86 66 65 103 84 67 107 78 104 98 71 108 109 98 51 74 117 97 87 69 120 70 106 65 85 66 103 78 86 66 65 99 84 68 86 78 104 98 105 66 71 10 99 109 70 117 89 50 108 122 89 50 56 120 71 84 65 88 66 103 78 86 66 65 111 84 69 71 57 121 90 122 69 117 90 88 104 104 98 88 66 115 90 83 53 106 98 50 48 120 72 68 65 97 66 103 78 86 66 65 77 84 69 50 78 104 10 76 109 57 121 90 122 69 117 90 88 104 104 98 88 66 115 90 83 53 106 98 50 48 119 72 104 99 78 77 106 69 120 77 84 73 48 77 68 89 122 78 84 65 119 87 104 99 78 77 122 69 120 77 84 73 121 77 68 89 122 78 84 65 119 10 87 106 66 113 77 81 115 119 67 81 89 68 86 81 81 71 69 119 74 86 85 122 69 84 77 66 69 71 65 49 85 69 67 66 77 75 81 50 70 115 97 87 90 118 99 109 53 112 89 84 69 87 77 66 81 71 65 49 85 69 66 120 77 78 10 85 50 70 117 73 69 90 121 89 87 53 106 97 88 78 106 98 122 69 78 77 65 115 71 65 49 85 69 67 120 77 69 99 71 86 108 99 106 69 102 77 66 48 71 65 49 85 69 65 120 77 87 99 71 86 108 99 106 65 117 98 51 74 110 10 77 83 53 108 101 71 70 116 99 71 120 108 76 109 78 118 98 84 66 90 77 66 77 71 66 121 113 71 83 77 52 57 65 103 69 71 67 67 113 71 83 77 52 57 65 119 69 72 65 48 73 65 66 66 66 117 67 116 111 47 110 51 57 118 10 121 73 100 116 54 113 115 122 73 122 50 57 53 52 50 86 99 113 89 114 54 122 85 98 83 72 119 51 113 87 106 55 43 109 122 66 103 106 73 116 73 78 97 122 81 78 109 69 108 68 122 55 109 98 111 105 118 113 116 66 116 121 48 85 10 110 120 68 85 122 89 102 54 89 74 43 106 84 84 66 76 77 65 52 71 65 49 85 100 68 119 69 66 47 119 81 69 65 119 73 72 103 68 65 77 66 103 78 86 72 82 77 66 65 102 56 69 65 106 65 65 77 67 115 71 65 49 85 100 10 73 119 81 107 77 67 75 65 73 75 67 122 65 56 69 52 66 107 86 116 52 103 121 120 68 82 49 71 115 107 109 53 51 53 78 52 71 113 67 55 104 86 71 50 111 89 86 50 100 80 107 66 77 65 111 71 67 67 113 71 83 77 52 57 10 66 65 77 67 65 48 103 65 77 69 85 67 73 81 67 87 72 104 80 103 89 77 97 107 67 66 105 66 75 117 55 107 49 53 78 118 109 84 78 122 43 110 103 113 88 85 71 43 85 102 101 74 74 73 66 107 101 119 73 103 84 52 115 74 10 52 57 117 85 118 118 54 120 53 88 83 122 72 104 102 122 118 108 85 87 57 110 101 87 73 115 117 73 69 98 110 119 121 103 48 66 104 122 48 61 10 45 45 45 45 45 69 78 68 32 67 69 82 84 73 70 73 67 65 84 69 45 45 45 45 45 10]=====================
		logger.Infof("==========Signature:%v=====================", endorsement.Signature)
		//==========Signature:[48 68 2 32 97 219 79 97 33 111 247 133 16 97 109 159 202 168 217 83 250 154 85 200 145 100 42 146 9 193 2 1 26 49 73 28 2 32 85 172 229 43 208 99 125 220 55 145 236 143 130 164 34 36 116 170 95 186 113 222 108 111 9 214 30 149 159 46 154 191]=====================
		signatureSet = append(signatureSet, &common.SignedData{
			// set the data that is signed; concatenation of proposal response bytes and endorser ID
			Data: data,
			// set the identity that signs the message: it's the endorser
			Identity: endorsement.Endorser,
			// set the signature
			Signature: endorsement.Signature})
	}

	// construct the policy checker object
	policyChecker := policyChecker{
		ccEP:          ccEP,
		policySupport: klv.policySupport,
		signatureSet:  signatureSet,
		vpmgr:         klv.vpmgr,
	}

	// unpack the rwset
	rwset := &rwsetutil.TxRwSet{}
	err := rwset.FromProtoBytes(rwsetBytes)
	logger.Info("====================err===", err)
	//nil
	if err != nil {
		return policyErr(errors.WithMessage(err, fmt.Sprintf("txRWSet.FromProtoBytes failed on tx (%d,%d)", blockNum, txNum)))
	}

	// iterate over all writes in the rwset
	for k, nsRWSet := range rwset.NsRwSets {
		logger.Infof("==========k:%v,nsRWSet:%v===================", k, nsRWSet)
		//==========k:0,nsRWSet:&{lscc reads:<key:"mycc" version:<block_num:3 > >  []}===================
		// ==========k:1,nsRWSet:&{mycc reads:<key:"a" version:<block_num:4 > > reads:<key:"b" version:<block_num:4 > > writes:<key:"a" value:"80" > writes:<key:"b" value:"220" >  []}===================
		//need to write to block
		//skip other namespaces
		logger.Info("====rwset.NsRwSets======nsRWSet.NameSpace ===================", nsRWSet.NameSpace)
		//0:lscc
		//1:mycc
		logger.Info("==========cc ===================", cc)
        //0:mycc'
        //1:mycc
		if nsRWSet.NameSpace != cc {
			//go this
			logger.Info("=======if nsRWSet.NameSpace != cc ========")
			continue
		}

		// public writes
		// we validate writes against key-level validation parameters
		// if any are present or the chaincode-wide endorsement policy
		for k, pubWrite := range nsRWSet.KvRwSet.Writes {
			logger.Infof("========k:%v,pubWrite:%v=============", k, pubWrite)
			//k:0,pubWrite:key:"a" value:"90" =============

			err := policyChecker.checkSBAndCCEP(cc, "", pubWrite.Key, blockNum, txNum)
			if err != nil {
				return err
			}

			logger.Infof("================%v=policyChecker.checkSBAndCCEP(%v, \"\", %v, %v, %v)===========================================",err,cc,pubWrite.Key,blockNum,txNum)
		}
		// public metadata writes
		// we validate writes against key-level validation parameters
		// if any are present or the chaincode-wide endorsement policy
		for k, pubMdWrite := range nsRWSet.KvRwSet.MetadataWrites {
			logger.Infof("==========nsRWSet.KvRwSet.MetadataWrites=====k:%v,v:%========================",k,pubMdWrite)
			err := policyChecker.checkSBAndCCEP(cc, "", pubMdWrite.Key, blockNum, txNum)
			if err != nil {
				return err
			}
			logger.Infof("================%v=policyChecker.checkSBAndCCEP(%v, \"\", %v, %v, %v)===========================================",err,cc, pubMdWrite.Key,blockNum,txNum)

		}
		// writes in collections
		// we validate writes against key-level validation parameters
		// if any are present or the chaincode-wide endorsement policy
		for k, collRWSet := range nsRWSet.CollHashedRwSets {
			logger.Infof("=======================nsRWSet.CollHashedRwSets===============k:%v,collRWSet:%v====================",k,collRWSet)

			coll := collRWSet.CollectionName
			for _, hashedWrite := range collRWSet.HashedRwSet.HashedWrites {
				key := string(hashedWrite.KeyHash)
				err := policyChecker.checkSBAndCCEP(cc, coll, key, blockNum, txNum)
				if err != nil {
					return err
				}
			}
		}
		// metadata writes in collections
		// we validate writes against key-level validation parameters
		// if any are present or the chaincode-wide endorsement policy
		for k, collRWSet := range nsRWSet.CollHashedRwSets {
			logger.Infof("==============================nsRWSet.CollHashedRwSets:========k:%v,collRWSet:%v====================",k,collRWSet)
			coll := collRWSet.CollectionName
			for _, hashedMdWrite := range collRWSet.HashedRwSet.MetadataWrites {
				key := string(hashedMdWrite.KeyHash)
				err := policyChecker.checkSBAndCCEP(cc, coll, key, blockNum, txNum)
				if err != nil {
					return err
				}
			}
		}
	}

	// we make sure that we check at least the CCEP to honour FAB-9473
	a := policyChecker.checkCCEPIfNoEPChecked(cc, blockNum, txNum)
	logger.Infof("============%v := policyChecker.checkCCEPIfNoEPChecked(%v, %v, %v)==========================",a,cc,blockNum,txNum)
	return a
}

// PostValidate implements the function of the StateBasedValidator interface
func (klv *KeyLevelValidator) PostValidate(cc string, blockNum, txNum uint64, err error) {
	logger.Info("=====================func (klv *KeyLevelValidator) PostValidate(cc string, blockNum, txNum uint64, err error)===============================================")
	klv.vpmgr.SetTxValidationResult(cc, blockNum, txNum, err)
}

func policyErr(err error) *commonerrors.VSCCEndorsementPolicyError {
	return &commonerrors.VSCCEndorsementPolicyError{
		Err: err,
	}
}
