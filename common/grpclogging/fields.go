/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package grpclogging

import (
	"fmt"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/jsonpb"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type protoMarshaler struct {
	jsonpb.Marshaler
	message proto.Message
}

func (m *protoMarshaler) MarshalJSON() ([]byte, error) {
	fmt.Println("====protoMarshaler====MarshalJSON============")
	out, err := m.Marshaler.MarshalToString(m.message)
	if err != nil {
		return nil, err
	}
	return []byte(out), nil
}

func ProtoMessage(key string, val interface{}) zapcore.Field {
	fmt.Println("====ProtoMessage============")
	if pm, ok := val.(proto.Message); ok {
		return zap.Reflect(key, &protoMarshaler{message: pm})
	}
	return zap.Any(key, val)
}

func Error(err error) zapcore.Field {
	fmt.Println("===Error===========")
	if err == nil {
		return zap.Skip()
	}

	// Wrap the error so it no longer implements fmt.Formatter. This will prevent
	// zap from adding the "verboseError" field to the log record that includes a
	// full stack trace.
	return zap.Error(struct{ error }{err})
}
