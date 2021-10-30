/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package grpclogging

import (
	"context"
	"fmt"

	"go.uber.org/zap/zapcore"
)

type fieldKeyType struct{}

var fieldKey = &fieldKeyType{}

func ZapFields(ctx context.Context) []zapcore.Field {
	fmt.Println("====ZapFields================")
	fields, ok := ctx.Value(fieldKey).([]zapcore.Field)
	if ok {
		return fields
	}
	return nil
}

func Fields(ctx context.Context) []interface{} {
	fmt.Println("====Fields================")
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
	fmt.Println("====WithFields================")
	return context.WithValue(ctx, fieldKey, fields)
}
