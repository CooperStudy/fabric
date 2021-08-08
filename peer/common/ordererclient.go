package common

import (
	"fmt"
	"strings"
	"time"

	cb "github.com/hyperledger/fabric/protos/common"
	ab "github.com/hyperledger/fabric/protos/orderer"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type BroadcastClient interface {
	//Send data to orderer
	Send(env *cb.Envelope) error
	Close() error
}

type broadcastClient struct {
	conn   *grpc.ClientConn
	client ab.AtomicBroadcast_BroadcastClient
}

// GetBroadcastClient creates a simple instance of the BroadcastClient interface
//GetBroadcastClientFnc的值
func GetBroadcastClient(orderingEndpoint string, tlsEnabled bool, caFile string) (BroadcastClient, error) {

	if len(strings.Split(orderingEndpoint, ":")) != 2 {
		return nil, fmt.Errorf("Ordering service endpoint %s is not valid or missing", orderingEndpoint)
	}

	var opts []grpc.DialOption
	// check for TLS
	if tlsEnabled {
		if caFile != "" {
			creds, err := credentials.NewClientTLSFromFile(caFile, "")
			if err != nil {
				return nil, fmt.Errorf("Error connecting to %s due to %s", orderingEndpoint, err)
			}
			opts = append(opts, grpc.WithTransportCredentials(creds))
		}
	} else {
		opts = append(opts, grpc.WithInsecure())
	}

	opts = append(opts, grpc.WithTimeout(3*time.Second))
	opts = append(opts, grpc.WithBlock())

	conn, err := grpc.Dial(orderingEndpoint, opts...)
	if err != nil {
		return nil, fmt.Errorf("Error connecting to %s due to %s", orderingEndpoint, err)
	}
	client, err := ab.NewAtomicBroadcastClient(conn).Broadcast(context.TODO())
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("Error connecting to %s due to %s", orderingEndpoint, err)
	}

	return &broadcastClient{conn: conn, client: client}, nil
}

func (s *broadcastClient) getAck() error {
	msg, err := s.client.Recv()
	if err != nil {
		return err
	}
	if msg.Status != cb.Status_SUCCESS {
		return fmt.Errorf("Got unexpected status: %v", msg.Status)
	}
	return nil
}

//Send data to orderer
func (s *broadcastClient) Send(env *cb.Envelope) error {
	if err := s.client.Send(env); err != nil {
		return fmt.Errorf("Could not send :%s)", err)
	}

	err := s.getAck()

	return err
}

func (s *broadcastClient) Close() error {
	return s.conn.Close()
}
