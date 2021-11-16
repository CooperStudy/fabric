/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package flogging

import (
	"fmt"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zapgrpc"
)

// NewZapLogger creates a zap logger around a new zap.Core. The core will use
// the provided encoder and sinks and a level enabler that is associated with
// the provided logger name. The logger that is returned will be named the same
// as the logger.
func NewZapLogger(core zapcore.Core, options ...zap.Option) *zap.Logger {
	fmt.Println("===NewZapLogger=========")
	return zap.New(
		core,
		append([]zap.Option{
			zap.AddCaller(),
			zap.AddStacktrace(zapcore.ErrorLevel),
		}, options...)...,
	)
}

// NewGRPCLogger creates a grpc.Logger that delegates to a zap.Logger.
func NewGRPCLogger(l *zap.Logger) *zapgrpc.Logger {
	fmt.Println("===NewGRPCLogger=========")
	l = l.WithOptions(
		zap.AddCaller(),
		zap.AddCallerSkip(3),
	)
	return zapgrpc.NewLogger(l, zapgrpc.WithDebug())
}

// NewFabricLogger creates a logger that delegates to the zap.SugaredLogger.
func NewFabricLogger(l *zap.Logger, options ...zap.Option) *FabricLogger {
	fmt.Println("===NewFabricLogger=========")
	return &FabricLogger{
		s: l.WithOptions(append(options, zap.AddCallerSkip(1))...).Sugar(),
	}
}

// A FabricLogger is an adapter around a zap.SugaredLogger that provides
// structured logging capabilities while preserving much of the legacy logging
// behavior.
//
// The most significant difference between the FabricLogger and the
// zap.SugaredLogger is that methods without a formatting suffix (f or w) build
// the log entry message with fmt.Sprintln instead of fmt.Sprint. Without this
// change, arguments are not separated by spaces.
type FabricLogger struct{ s *zap.SugaredLogger }

func (f *FabricLogger) DPanic(args ...interface{})                    { f.s.DPanicf(formatArgs(args)) }
func (f *FabricLogger) DPanicf(template string, args ...interface{})  { f.s.DPanicf(template, args...) }
func (f *FabricLogger) DPanicw(msg string, kvPairs ...interface{})    { f.s.DPanicw(msg, kvPairs...) }
func (f *FabricLogger) Debug(args ...interface{})                     { f.s.Debugf(formatArgs(args)) }
func (f *FabricLogger) Debugf(template string, args ...interface{})   { f.s.Debugf(template, args...) }
func (f *FabricLogger) Debugw(msg string, kvPairs ...interface{})     { f.s.Debugw(msg, kvPairs...) }
func (f *FabricLogger) Error(args ...interface{})                     { f.s.Errorf(formatArgs(args)) }
func (f *FabricLogger) Errorf(template string, args ...interface{})   { f.s.Errorf(template, args...) }
func (f *FabricLogger) Errorw(msg string, kvPairs ...interface{})     { f.s.Errorw(msg, kvPairs...) }
func (f *FabricLogger) Fatal(args ...interface{})                     { f.s.Fatalf(formatArgs(args)) }
func (f *FabricLogger) Fatalf(template string, args ...interface{})   { f.s.Fatalf(template, args...) }
func (f *FabricLogger) Fatalw(msg string, kvPairs ...interface{})     { f.s.Fatalw(msg, kvPairs...) }
func (f *FabricLogger) Info(args ...interface{})                      { f.s.Infof(formatArgs(args)) }
func (f *FabricLogger) Infof(template string, args ...interface{})    { f.s.Infof(template, args...) }
func (f *FabricLogger) Infow(msg string, kvPairs ...interface{})      { f.s.Infow(msg, kvPairs...) }
func (f *FabricLogger) Panic(args ...interface{})                     { f.s.Panicf(formatArgs(args)) }
func (f *FabricLogger) Panicf(template string, args ...interface{})   { f.s.Panicf(template, args...) }
func (f *FabricLogger) Panicw(msg string, kvPairs ...interface{})     { f.s.Panicw(msg, kvPairs...) }
func (f *FabricLogger) Warn(args ...interface{})                      { f.s.Warnf(formatArgs(args)) }
func (f *FabricLogger) Warnf(template string, args ...interface{})    { f.s.Warnf(template, args...) }
func (f *FabricLogger) Warnw(msg string, kvPairs ...interface{})      { f.s.Warnw(msg, kvPairs...) }
func (f *FabricLogger) Warning(args ...interface{})                   { f.s.Warnf(formatArgs(args)) }
func (f *FabricLogger) Warningf(template string, args ...interface{}) { f.s.Warnf(template, args...) }

// for backwards compatibility
func (f *FabricLogger) Critical(args ...interface{})                   { f.s.Errorf(formatArgs(args)) }
func (f *FabricLogger) Criticalf(template string, args ...interface{}) { f.s.Errorf(template, args...) }
func (f *FabricLogger) Notice(args ...interface{})                     { f.s.Infof(formatArgs(args)) }
func (f *FabricLogger) Noticef(template string, args ...interface{})   { f.s.Infof(template, args...) }

func (f *FabricLogger) Named(name string) *FabricLogger { return &FabricLogger{s: f.s.Named(name)} }
func (f *FabricLogger) Sync() error                     { return f.s.Sync() }
func (f *FabricLogger) Zap() *zap.Logger                { return f.s.Desugar() }

func (f *FabricLogger) IsEnabledFor(level zapcore.Level) bool {
	fmt.Println("===FabricLogger====IsEnabledFor=====")
	return f.s.Desugar().Core().Enabled(level)
}

func (f *FabricLogger) With(args ...interface{}) *FabricLogger {
	fmt.Println("===FabricLogger====With=====")
	return &FabricLogger{s: f.s.With(args...)}
}

func (f *FabricLogger) WithOptions(opts ...zap.Option) *FabricLogger {
	fmt.Println("===FabricLogger====WithOptions=====")
	l := f.s.Desugar().WithOptions(opts...)
	return &FabricLogger{s: l.Sugar()}
}

func formatArgs(args []interface{}) string {
	fmt.Println("===formatArgs====")
	//fmt.Println("=========args=========",args)//[=======GetDefault=================defaultBCCSP 0xc000014b40]
	//[Sending IDENTITY_MSG hello to peer1.org1.example.com:7051]
	s:= strings.TrimSuffix(fmt.Sprintln(args...), "\n")
	//fmt.Println("========s=========",s)//Sleeping 25s //Sending IDENTITY_MSG hello to peer1.org1.example.com:7051
	// [Entering, Sending to peer1.org1.example.com:7051 , msg: GossipMessage: tag:EMPTY hello:<nonce:276265988097776331 msg_type:IDENTITY_MSG > , Envelope: 16 bytes, Signature: 0 bytes]
	//Created with config TLS: true, auth cache disabled
	//========s========= =======GetDefault=================defaultBCCSP &{0x17db2d8 map[*bccsp.ECDSAP256KeyGenOpts:0xc0002dd6a0 *bccsp.ECDSAP384KeyGenOpts:0xc0002dd6b0 *bccsp.AES192KeyGenOpts:0xc000458f98 *bccsp.RSAKeyGenOpts:0xc000458fb8 *bccsp.RSA2048KeyGenOpts:0xc000458fd8 *bccsp.RSA4096KeyGenOpts:0xc000458ff8 *bccsp.ECDSAKeyGenOpts:0xc0002dd690 *bccsp.AESKeyGenOpts:0xc000458f78 *bccsp.AES256KeyGenOpts:0xc000458f88 *bccsp.AES128KeyGenOpts:0xc000458fa8 *bccsp.RSA1024KeyGenOpts:0xc000458fc8 *bccsp.RSA3072KeyGenOpts:0xc000458fe8] map[*sw.ecdsaPrivateKey:0x17db2d8 *sw.ecdsaPublicKey:0x17db2d8 *sw.aesPrivateKey:0xc0002e4460] map[*bccsp.ECDSAPrivateKeyImportOpts:0x17db2d8 *bccsp.ECDSAGoPublicKeyImportOpts:0x17db2d8 *bccsp.RSAGoPublicKeyImportOpts:0x17db2d8 *bccsp.X509PublicKeyImportOpts:0xc0002e4468 *bccsp.AES256ImportKeyOpts:0x17db2d8 *bccsp.HMACImportKeyOpts:0x17db2d8 *bccsp.ECDSAPKIXPublicKeyImportOpts:0x17db2d8] map[*sw.aesPrivateKey:0x17db2d8] map[*sw.aesPrivateKey:0x17db2d8] map[*sw.ecdsaPrivateKey:0x17db2d8 *sw.rsaPrivateKey:0x17db2d8] map[*sw.ecdsaPrivateKey:0x17db2d8 *sw.ecdsaPublicKey:0x17db2d8 *sw.rsaPrivateKey:0x17db2d8 *sw.rsaPublicKey:0x17db2d8] map[*bccsp.SHA384Opts:0xc0002e4438 *bccsp.SHA3_256Opts:0xc0002e4440 *bccsp.SHA3_384Opts:0xc0002e4448 *bccsp.SHAOpts:0xc0002e4428 *bccsp.SHA256Opts:0xc0002e4430]}
	/*
	[Got message: GossipMessage: tag:EMPTY data_dig:<nonce:276265988097776331 digests:"\364\272 \026p\303\266\036\304e\330\337T8\t\353~sa\311\221k{\n\302\n\226\354%\213\3117" digests:"v\302\304I\323!#\263\301\370[\366I\345\323n\334\315\276\035\"\213\346\023\0178y`Ht\177\360" msg_type:IDENTITY_MSG > , Envelope: 84 bytes, Signature: 0 bytes]
	*/
	return s
}
