/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package grpclogging

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

// Leveler returns a zap level to use when logging from a grpc interceptor.
type Leveler interface {
	Level(ctx context.Context, fullMethod string) zapcore.Level
}

// PayloadLeveler gets the level to use when logging grpc message payloads.
type PayloadLeveler interface {
	PayloadLevel(ctx context.Context, fullMethod string) zapcore.Level
}

//go:generate counterfeiter -o fakes/leveler.go --fake-name Leveler . LevelerFunc

type LevelerFunc func(ctx context.Context, fullMethod string) zapcore.Level

func (l LevelerFunc) Level(ctx context.Context, fullMethod string) zapcore.Level {
	//logger.Info("===LevelerFunc==Level===========")
	return l(ctx, fullMethod)
}

func (l LevelerFunc) PayloadLevel(ctx context.Context, fullMethod string) zapcore.Level {
	//logger.Info("===LevelerFunc==PayloadLevel===========")
	return l(ctx, fullMethod)
}

// DefaultPayloadLevel is default level to use when logging payloads
const DefaultPayloadLevel = zapcore.Level(zapcore.DebugLevel - 1)

type options struct {
	Leveler
	PayloadLeveler
}

type Option func(o *options)

func WithLeveler(l Leveler) Option {
	//logger.Info("===WithLeveler==========")
	return func(o *options) { o.Leveler = l }
}

func WithPayloadLeveler(l PayloadLeveler) Option {
	//logger.Info("===WithPayloadLeveler==========")
	return func(o *options) { o.PayloadLeveler = l }
}

func applyOptions(opts ...Option) *options {
	//logger.Info("===applyOptions=========")
	o := &options{
		Leveler:        LevelerFunc(func(context.Context, string) zapcore.Level { return zapcore.InfoLevel }),
		PayloadLeveler: LevelerFunc(func(context.Context, string) zapcore.Level { return DefaultPayloadLevel }),
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// Levelers will be required and should be provided with the full method info

func UnaryServerInterceptor(logger *zap.Logger, opts ...Option) grpc.UnaryServerInterceptor {
	//logger.Info("===UnaryServerInterceptor=========")
	o := applyOptions(opts...)

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		logger := logger
		startTime := time.Now()

		fields := getFields(ctx, startTime, info.FullMethod)
		logger = logger.With(fields...)
		ctx = WithFields(ctx, fields)

		payloadLogger := logger.Named("payload")
		payloadLevel := o.PayloadLevel(ctx, info.FullMethod)
		if ce := payloadLogger.Check(payloadLevel, "received unary request"); ce != nil {
			ce.Write(ProtoMessage("message", req))
		}

		resp, err := handler(ctx, req)

		if ce := payloadLogger.Check(payloadLevel, "sending unary response"); ce != nil && err == nil {
			ce.Write(ProtoMessage("message", resp))
		}

		if ce := logger.Check(o.Level(ctx, info.FullMethod), "unary call completed"); ce != nil {
			/*
			unary call completed
			{"grpc.start_time": "2021-11-18T06:31:19.239Z",
			"grpc.service": "protos.Endorser",
			"grpc.method": "ProcessProposal",
			"grpc.request_deadline": "2021-11-18T06:34:19.232Z",
			"grpc.peer_address": "172.19.0.1:37296",
			"grpc.code": "OK",
			"grpc.call_duration": "1.786148ms"}

			*/
			ce.Write(
				Error(err),
				zap.Stringer("grpc.code", grpc.Code(err)),
				zap.Duration("grpc.call_duration", time.Since(startTime)),
			)
		}

		return resp, err
	}
}

func StreamServerInterceptor(logger *zap.Logger, opts ...Option) grpc.StreamServerInterceptor {
	//logger.Info("===StreamServerInterceptor=========")
	o := applyOptions(opts...)

	return func(service interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		logger := logger
		ctx := stream.Context()
		startTime := time.Now()

		fields := getFields(ctx, startTime, info.FullMethod)
		logger = logger.With(fields...)
		ctx = WithFields(ctx, fields)

		wrappedStream := &serverStream{
			ServerStream:  stream,
			context:       ctx,
			payloadLogger: logger.Named("payload"),
			payloadLevel:  o.PayloadLevel(ctx, info.FullMethod),
		}

		err := handler(service, wrappedStream)
		if ce := logger.Check(o.Level(ctx, info.FullMethod), "streaming call completed"); ce != nil {
			ce.Write(
				Error(err),
				zap.Stringer("grpc.code", grpc.Code(err)),
				zap.Duration("grpc.call_duration", time.Since(startTime)),
			)
		}
		return err
	}
}

func getFields(ctx context.Context, startTime time.Time, method string) []zapcore.Field {
	//logger.Info("===getFields=========")
	fields := []zap.Field{zap.Time("grpc.start_time", startTime)}
	if parts := strings.Split(method, "/"); len(parts) == 3 {
		fields = append(fields, zap.String("grpc.service", parts[1]), zap.String("grpc.method", parts[2]))
	}
	if deadline, ok := ctx.Deadline(); ok {
		fields = append(fields, zap.Time("grpc.request_deadline", deadline))
	}
	if p, ok := peer.FromContext(ctx); ok {
		fields = append(fields, zap.String("grpc.peer_address", p.Addr.String()))
		if ti, ok := p.AuthInfo.(credentials.TLSInfo); ok {
			if len(ti.State.PeerCertificates) > 0 {
				cert := ti.State.PeerCertificates[0]
				fields = append(fields, zap.String("grpc.peer_subject", cert.Subject.String()))
			}
		}
	}
	return fields
}

type serverStream struct {
	grpc.ServerStream
	context       context.Context
	payloadLogger *zap.Logger
	payloadLevel  zapcore.Level
}

func (ss *serverStream) Context() context.Context {
	//logger.Info("===serverStream===Context======")
	return ss.context
}

func (ss *serverStream) SendMsg(msg interface{}) error {
	//logger.Info("===serverStream===SendMsg======")
	if ce := ss.payloadLogger.Check(ss.payloadLevel, "sending stream message"); ce != nil {
		ce.Write(ProtoMessage("message", msg))
	}
	return ss.ServerStream.SendMsg(msg)
}

func (ss *serverStream) RecvMsg(msg interface{}) error {
	//logger.Info("===serverStream===RecvMsg======")
	err := ss.ServerStream.RecvMsg(msg)
	if ce := ss.payloadLogger.Check(ss.payloadLevel, "received stream message"); ce != nil {
		ce.Write(ProtoMessage("message", msg))
	}
	return err
}
