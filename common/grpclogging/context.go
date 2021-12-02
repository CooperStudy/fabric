/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package grpclogging

import (
	"context"
	"github.com/hyperledger/fabric/common/flogging"

	"go.uber.org/zap/zapcore"
)

type fieldKeyType struct{}

var fieldKey = &fieldKeyType{}
var grpcloggingLogger = flogging.MustGetLogger("common.grpclogging")
func ZapFields(ctx context.Context) []zapcore.Field {
	grpcloggingLogger.Info("====ZapFields================")
	fields, ok := ctx.Value(fieldKey).([]zapcore.Field)
	if ok {
		return fields
	}
	return nil
}

func Fields(ctx context.Context) []interface{} {
	grpcloggingLogger.Info("====Fields================")
	fields, ok := ctx.Value(fieldKey).([]zapcore.Field)
	if !ok {
		return nil
	}
	genericFields := make([]interface{}, len(fields))
	for i := range fields {
		genericFields[i] = fields[i]
	}
	return genericFields
}

func WithFields(ctx context.Context, fields []zapcore.Field) context.Context {
	grpcloggingLogger.Info("====WithFields================")
	grpcloggingLogger.Infof("===context.WithValue(%v, %v, %v)===============",ctx,fieldKey,fields)

	return context.WithValue(ctx, fieldKey, fields)
}
