package go_uber_org_zap_zapcore

/*
因为 zap 本身不支持切割归档日志文件，为了添加日志切割归档功能，我们将使用第三方库 lumberjack 来实现。、
安装 lumberjack
将 lumberjack 加入 zap logger
要在 zap 中加入 lumberjack 支持，我们需要修改 WriteSyncer 代码。我们将按照下面的代码修改 getLogWriter() 函数：
go get -uv github.com/natefinch/lumberjack
*/

//func getLogWriter1() zapcore.WriteSyncer {
//	lumberJackLogger := &lumberjack.Logger{
//		Filename:   "./test.log",  //日志文件的位置；
//		MaxSize:    10,// 在进行切割之前，日志文件的最大大小（以MB为单位）；
//		MaxBackups: 5, // 保留旧文件的最大个数；
//		MaxAge:     30, //保留旧文件的最大天数；
//		Compress:   false, //是否压缩/归档旧文件；
//	}
//	return zapcore.AddSync(lumberJackLogger)
//}

//
//var sugarLogger *zap.SugaredLogger
//
//func InitLogger() {
//
//	encoder := getEncoder()
//	writeSyncer := getLogWriter()
//	core := zapcore.NewCore(encoder, writeSyncer, zapcore.DebugLevel)
//
//	// zap.AddCaller()  添加将调用函数信息记录到日志中的功能。
//	logger := zap.New(core, zap.AddCaller())
//	sugarLogger = logger.Sugar()
//}
//
//func getEncoder() zapcore.Encoder {
//	encoderConfig := zap.NewProductionEncoderConfig()
//	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder // 修改时间编码器
//
//	// 在日志文件中使用大写字母记录日志级别
//	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
//	// NewConsoleEncoder 打印更符合人们观察的方式
//	return zapcore.NewConsoleEncoder(encoderConfig)
//}
//
//func getLogWriter() zapcore.WriteSyncer {
//	lumberJackLogger := &lumberjack.Logger{
//		Filename:   "./test.log",
//		MaxSize:    10,
//		MaxBackups: 5,
//		MaxAge:     30,
//		Compress:   false,
//	}
//	return zapcore.AddSync(lumberJackLogger)
//}
//
//func main() {
//	InitLogger()
//	sugarLogger.Info("this is info message")
//	sugarLogger.Infof("this is %s, %d", "aaa", 1234)
//	sugarLogger.Error("this is error message")
//	sugarLogger.Info("this is info message")
//}
//
//
