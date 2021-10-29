/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package comm

import (
	"fmt"
	"time"

	"github.com/hyperledger/fabric/common/crypto/tlsgen"
	"github.com/hyperledger/fabric/common/util"
	"github.com/hyperledger/fabric/core/comm"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

const defaultTimeout = time.Second * 5

// Client deals with TLS connections
// to the discovery server
type Client struct {
	TLSCertHash []byte
	*comm.GRPCClient
}

// NewClient creates a new comm client out of the given configuration
func NewClient(conf Config) (*Client, error) {
	fmt.Println("========NewClient===========")
	if conf.Timeout == time.Duration(0) {
		conf.Timeout = defaultTimeout
	}
	sop, err := conf.ToSecureOptions(newSelfSignedTLSCert)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	cl, err := comm.NewGRPCClient(comm.ClientConfig{
		SecOpts: sop,
		Timeout: conf.Timeout,
	})
	if err != nil {
		return nil, err
	}
	return &Client{GRPCClient: cl, TLSCertHash: util.ComputeSHA256(sop.Certificate)}, nil
}

// NewDialer creates a new dialer from the given endpoint
func (c *Client) NewDialer(endpoint string) func() (*grpc.ClientConn, error) {
	fmt.Println("==Client=====NewDialer=======")
	return func() (*grpc.ClientConn, error) {
		conn, err := c.NewConnection(endpoint, "")
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return conn, nil
	}
}

func newSelfSignedTLSCert() (*tlsgen.CertKeyPair, error) {
	fmt.Println("==newSelfSignedTLSCert=======")
	ca, err := tlsgen.NewCA()
	if err != nil {
		return nil, err
	}
	return ca.NewClientCertKeyPair()
}
