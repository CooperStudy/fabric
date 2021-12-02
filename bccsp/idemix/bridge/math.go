/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/
package bridge

import (
	"github.com/hyperledger/fabric-amcl/amcl/FP256BN"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/idemix"
)


var logger = flogging.MustGetLogger("bccsp.idemix.bridge")
// Big encapsulate an amcl big integer
type Big struct {
	E *FP256BN.BIG
}

func (b *Big) Bytes() ([]byte, error) {
	//logger.Info("====Big====Bytes=============")
	return idemix.BigToBytes(b.E), nil
}

// Ecp encapsulate an amcl elliptic curve point
type Ecp struct {
	E *FP256BN.ECP
}

func (o *Ecp) Bytes() ([]byte, error) {
	//logger.Info("====Ecp====Bytes=============")
	var res []byte
	res = append(res, idemix.BigToBytes(o.E.GetX())...)
	res = append(res, idemix.BigToBytes(o.E.GetY())...)

	return res, nil
}
