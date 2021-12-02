/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package grpcmetrics

import (
	"context"
	"github.com/hyperledger/fabric/common/flogging"
	"strings"
	"time"

	"github.com/hyperledger/fabric/common/metrics"
	"google.golang.org/grpc"
)

var grpcmetricsLogger = flogging.MustGetLogger("common.grpcmetrics")
type UnaryMetrics struct {
	RequestDuration   metrics.Histogram
	RequestsReceived  metrics.Counter
	RequestsCompleted metrics.Counter
}

func UnaryServerInterceptor(um *UnaryMetrics) grpc.UnaryServerInterceptor {
	grpcmetricsLogger.Info("===UnaryServerInterceptor======")
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		service, method := serviceMethod(info.FullMethod)
		um.RequestsReceived.With("service", service, "method", method).Add(1)

		startTime := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(startTime)

		um.RequestDuration.With(
			"service", service, "method", method, "code", grpc.Code(err).String(),
		).Observe(duration.Seconds())
		um.RequestsCompleted.With("service", service, "method", method, "code", grpc.Code(err).String()).Add(1)

		return resp, err
	}
}

type StreamMetrics struct {
	RequestDuration   metrics.Histogram
	RequestsReceived  metrics.Counter
	RequestsCompleted metrics.Counter
	MessagesSent      metrics.Counter
	MessagesReceived  metrics.Counter
}

func StreamServerInterceptor(sm *StreamMetrics) grpc.StreamServerInterceptor {
	grpcmetricsLogger.Info("===StreamServerInterceptor======")
	return func(svc interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		sm := sm
		service, method := serviceMethod(info.FullMethod)
		sm.RequestsReceived.With("service", service, "method", method).Add(1)

		wrappedStream := &serverStream{
			ServerStream:     stream,
			messagesSent:     sm.MessagesSent.With("service", service, "method", method),
			messagesReceived: sm.MessagesReceived.With("service", service, "method", method),
		}

		startTime := time.Now()
		err := handler(svc, wrappedStream)
		duration := time.Since(startTime)

		sm.RequestDuration.With(
			"service", service, "method", method, "code", grpc.Code(err).String(),
		).Observe(duration.Seconds())
		sm.RequestsCompleted.With("service", service, "method", method, "code", grpc.Code(err).String()).Add(1)

		return err
	}
}

func serviceMethod(fullMethod string) (service, method string) {
	grpcmetricsLogger.Info("===serviceMethod======")
	//logger.Info("============fullMethod===========",fullMethod)//protos.Endorser/ProcessProposal
	normalizedMethod := strings.Replace(fullMethod, ".", "_", -1)
	parts := strings.SplitN(normalizedMethod, "/", -1)
	if len(parts) != 3 {
		return "unknown", "unknown"
	}
	grpcmetricsLogger.Info("==========parts[1], parts[2]====================",parts[1], parts[2])//protos_Endorser ProcessProposal
	return parts[1], parts[2]
}

type serverStream struct {
	grpc.ServerStream
	messagesSent     metrics.Counter
	messagesReceived metrics.Counter
}

func (ss *serverStream) SendMsg(msg interface{}) error {
//	logger.Info("===serverStream===SendMsg===")
	ss.messagesSent.Add(1)
	return ss.ServerStream.SendMsg(msg)
}

func (ss *serverStream) RecvMsg(msg interface{}) error {
	//logger.Info("===serverStream===RecvMsg===")
	err := ss.ServerStream.RecvMsg(msg)
	if err == nil {
		ss.messagesReceived.Add(1)
	}
	return err
}
