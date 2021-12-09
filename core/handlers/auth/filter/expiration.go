/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package filter

import (
	"context"
	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/protos/msp"
	"time"

	"github.com/hyperledger/fabric/common/crypto"
	"github.com/hyperledger/fabric/core/handlers/auth"
	"github.com/hyperledger/fabric/protos/peer"
	"github.com/hyperledger/fabric/protos/utils"
	"github.com/pkg/errors"
)

var CreatorName string

// NewExpirationCheckFilter creates a new Filter that checks identity expiration
func NewExpirationCheckFilter() auth.Filter {
	filterLogger.Info("==NewExpirationCheckFilter==")
	return &expirationCheckFilter{}
}

type expirationCheckFilter struct {
	next peer.EndorserServer
}

// Init initializes the Filter with the next EndorserServer
func (f *expirationCheckFilter) Init(next peer.EndorserServer) {
	filterLogger.Info("==expirationCheckFilter==Init==")
	f.next = next
}

func validateProposal(signedProp *peer.SignedProposal) error {
	filterLogger.Info("== validateProposal(signedProp *peer.SignedProposal) error ===")
	//filterLogger.Infof("========signedProp.Signature:%v==============",signedProp.Signature)
	/*
	[48 69 2 33 0 160 16 97 105 183 233 21 149 150 200 140 87 160 93 139 140 181 141 100 195 173 86 95 35 185 255 176 39 48 171 13 3 2 32 99 53 43 51 166 174 45 69 167 118 14 204 177 73 24 187 169 226 147 198 7 42 150 40 87 214 106 203 72 208 226 230]
	*/
	//filterLogger.Infof("========signedProp.ProposalBytes:%v==============",signedProp.ProposalBytes)
	/*
	[10 180 7 10 92 8 3 26 12 8 149 154 220 140 6 16 251 157 152 187 2 42 64 50 98 50 99 55 52 101 54 48 100 48 55 98 97 98 49 53 54 97 102 48 101 48 49 97 50 54 102 56 102 56 54 52 51 56 56 100 56 50 56 99 55 53 55 52 102 97 55 48 99 52 54 49 49 51 102 101 56 101 50 99 98 50 54 58 8 18 6 18 4 108 115 99 99 18 211 6 10 182 6 10 7 79 114 103 49 77 83 80 18 170 6 45 45 45 45 45 66 69 71 73 78 32 67 69 82 84 73 70 73 67 65 84 69 45 45 45 45 45 10 77 73 73 67 75 106 67 67 65 100 71 103 65 119 73 66 65 103 73 82 65 76 118 56 78 100 104 116 73 121 74 71 72 107 49 68 79 43 99 90 102 67 56 119 67 103 89 73 75 111 90 73 122 106 48 69 65 119 73 119 99 122 69 76 10 77 65 107 71 65 49 85 69 66 104 77 67 86 86 77 120 69 122 65 82 66 103 78 86 66 65 103 84 67 107 78 104 98 71 108 109 98 51 74 117 97 87 69 120 70 106 65 85 66 103 78 86 66 65 99 84 68 86 78 104 98 105 66 71 10 99 109 70 117 89 50 108 122 89 50 56 120 71 84 65 88 66 103 78 86 66 65 111 84 69 71 57 121 90 122 69 117 90 88 104 104 98 88 66 115 90 83 53 106 98 50 48 120 72 68 65 97 66 103 78 86 66 65 77 84 69 50 78 104 10 76 109 57 121 90 122 69 117 90 88 104 104 98 88 66 115 90 83 53 106 98 50 48 119 72 104 99 78 77 106 69 120 77 84 69 49 77 68 107 121 78 122 65 119 87 104 99 78 77 122 69 120 77 84 69 122 77 68 107 121 78 122 65 119 10 87 106 66 115 77 81 115 119 67 81 89 68 86 81 81 71 69 119 74 86 85 122 69 84 77 66 69 71 65 49 85 69 67 66 77 75 81 50 70 115 97 87 90 118 99 109 53 112 89 84 69 87 77 66 81 71 65 49 85 69 66 120 77 78 10 85 50 70 117 73 69 90 121 89 87 53 106 97 88 78 106 98 122 69 80 77 65 48 71 65 49 85 69 67 120 77 71 89 50 120 112 90 87 53 48 77 82 56 119 72 81 89 68 86 81 81 68 68 66 90 66 90 71 49 112 98 107 66 118 10 99 109 99 120 76 109 86 52 89 87 49 119 98 71 85 117 89 50 57 116 77 70 107 119 69 119 89 72 75 111 90 73 122 106 48 67 65 81 89 73 75 111 90 73 122 106 48 68 65 81 99 68 81 103 65 69 47 67 67 106 77 72 99 120 10 118 102 113 76 66 51 122 53 86 107 50 114 101 79 97 73 49 80 82 97 97 89 111 114 112 119 47 77 112 55 67 108 110 121 69 83 104 50 122 78 81 90 115 53 112 103 72 74 57 89 118 79 99 108 100 69 109 66 112 104 115 70 112 120 10 119 78 77 90 122 76 115 122 86 108 43 114 72 54 78 78 77 69 115 119 68 103 89 68 86 82 48 80 65 81 72 47 66 65 81 68 65 103 101 65 77 65 119 71 65 49 85 100 69 119 69 66 47 119 81 67 77 65 65 119 75 119 89 68 10 86 82 48 106 66 67 81 119 73 111 65 103 75 50 65 49 104 114 82 54 113 120 108 50 69 66 57 56 119 55 74 102 50 121 109 47 53 116 103 49 118 103 97 101 88 101 74 51 102 76 98 87 76 50 81 119 67 103 89 73 75 111 90 73 10 122 106 48 69 65 119 73 68 82 119 65 119 82 65 73 103 97 100 54 116 65 100 43 111 89 109 105 84 80 66 66 50 106 83 76 77 84 88 115 85 76 121 108 102 71 121 68 69 73 97 108 55 81 87 103 97 102 48 103 67 73 68 67 76 10 106 114 89 71 104 79 113 110 49 100 98 116 47 89 82 76 67 118 68 101 79 100 101 120 85 74 70 106 47 48 114 70 110 74 57 116 87 75 114 108 10 45 45 45 45 45 69 78 68 32 67 69 82 84 73 70 73 67 65 84 69 45 45 45 45 45 10 18 24 95 152 142 239 219 33 68 3 237 142 39 171 157 254 184 8 206 167 196 209 85 136 213 250 18 40 10 38 10 36 8 1 18 6 18 4 108 115 99 99 26 24 10 22 103 101 116 105 110 115 116 97 108 108 101 100 99 104 97 105 110 99 111 100 101 115]
	 */

	prop, err := utils.GetProposal(signedProp.ProposalBytes)

	if err != nil {
		return errors.Wrap(err, "failed parsing proposal")
	}

	hdr, err := utils.GetHeader(prop.Header)
	if err != nil {
		return errors.Wrap(err, "failed parsing header")
	}

	sh, err := utils.GetSignatureHeader(hdr.SignatureHeader)
	if err != nil {
		return errors.Wrap(err, "failed parsing signature header")
	}
	expirationTime := crypto.ExpiresAt(sh.Creator)

	cc := &msp.SerializedIdentity{}
	err = proto.Unmarshal(sh.Creator,cc)
	filterLogger.Infof("===========instantiate获取创建者.Name:%v==========\n",cc.Mspid)//Org1MSP
	CreatorName = cc.Mspid

	filterLogger.Info("====================判断证书是否过期===================")
	if !expirationTime.IsZero() && time.Now().After(expirationTime) {
		return errors.New("identity expired")
	}
	return nil
}

// ProcessProposal processes a signed proposal
func (f *expirationCheckFilter) ProcessProposal(ctx context.Context, signedProp *peer.SignedProposal) (*peer.ProposalResponse, error) {
	filterLogger.Info("==expirationCheckFilter==ProcessProposal===")
	if err := validateProposal(signedProp); err != nil {
		return nil, err
	}
	filterLogger.Info("================f.next:%T===================\n",f.next)//*endorser.Endorser
	//*endorser.Endorser
	return f.next.ProcessProposal(ctx, signedProp)
}
