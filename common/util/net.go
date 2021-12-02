/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package util

import (
	"context"
	"github.com/hyperledger/fabric/common/flogging"

	"google.golang.org/grpc/peer"
)
var utilLogger = flogging.MustGetLogger("common.util")
func ExtractRemoteAddress(ctx context.Context) string {
	utilLogger.Info("===ExtractRemoteAddress================")
	var remoteAddress string
	p, ok := peer.FromContext(ctx)
	if !ok {
		return ""
	}
	if address := p.Addr; address != nil {
		remoteAddress = address.String()
		utilLogger.Infof("=====remoteAddress:%v================",remoteAddress)
		//====remoteAddress================ 172.19.0.1:37296

	}
	return remoteAddress
}
