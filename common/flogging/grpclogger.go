package flogging

import (
	"github.com/op/go-logging"
	"google.golang.org/grpc/grpclog"
)
//设grpc的日志,grpc默认只用go语言的标准日志接口,
//因此需要将logging封装成标准日志性追的接口,type grpclogger struct{ logger *logging.Logger() }
//然后通过initgrpclogger（）生成对象供rpc使用从而实现grpc也使用floging的效果
const GRPCModuleID = "grpc"

func initgrpclogger() {
	glogger := MustGetLogger(GRPCModuleID)
	grpclog.SetLogger(&grpclogger{glogger})
}

// grpclogger implements the standard Go logging interface and wraps the
// logger provided by the flogging package.  This is required in order to
// replace the default log used by the grpclog package.
type grpclogger struct {
	logger *logging.Logger
}

func (g *grpclogger) Fatal(args ...interface{}) {
	g.logger.Fatal(args...)
}

func (g *grpclogger) Fatalf(format string, args ...interface{}) {
	g.logger.Fatalf(format, args...)
}

func (g *grpclogger) Fatalln(args ...interface{}) {
	g.logger.Fatal(args...)
}

// NOTE: grpclog does not support leveled logs so for now use DEBUG
func (g *grpclogger) Print(args ...interface{}) {
	g.logger.Debug(args...)
}

func (g *grpclogger) Printf(format string, args ...interface{}) {
	g.logger.Debugf(format, args...)
}

func (g *grpclogger) Println(args ...interface{}) {
	g.logger.Debug(args...)
}
