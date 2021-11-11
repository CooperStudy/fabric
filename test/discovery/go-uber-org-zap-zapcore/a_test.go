package go_uber_org_zap_zapcore

import (
	"go.uber.org/zap"
	"testing"
)
/*
zap 是 uber 开源的 Go 高性能日志库，支持不同的日志级别，
能够打印基本信息等，但不支持日志的分割，
这里我们可以使用 lumberjack 也是 zap 官方推荐用于日志分割，
结合这两个库我们就可以实现以下功能的日志机制：
能够将事件记录到文件中，而不是应用程序控制台；
日志切割能够根据文件大小、时间或间隔等来切割日志文件；
支持不同的日志级别，例如 DEBUG ， INFO ， WARN ， ERROR 等；
能够打印基本信息，如调用文件、函数名和行号，日志时间等；

3. 配置 zap Logger

zap 提供了两种类型的日志记录器—和 Logger 和 Sugared Logger 。两者之间的区别是：
在每一微秒和每一次内存分配都很重要的上下文中，使用Logger。它甚至比SugaredLogger更快，内存分配次数也更少，但它只支持强类型的结构化日志记录。
在性能很好但不是很关键的上下文中，使用SugaredLogger。它比其他结构化日志记录包快 4-10 倍，并且支持结构化和 printf 风格的日志记录。
所以一般场景下我们使用 Sugared Logger 就足够了。
通过调用zap.NewProduction()/zap.NewDevelopment()或者zap.NewExample()创建一个 Logger 。
上面的每一个函数都将创建一个 logger 。唯一的区别在于它将记录的信息不同。例如 production logger 默认记录调用函数信息、日期和时间等。
通过 Logger 调用 INFO 、 ERROR 等。
默认情况下日志都会打印到应用程序的 console 界面。

 */
func TestNewExample(t *testing.T) {
	log := zap.NewExample()
	log.Debug("this is debug message")
	log.Info("this is info message")
	log.Info("this is info message with fileds", zap.Int("age", 24), zap.String("agender", "man"))
	log.Warn("this is warn message")
	log.Error("this is error message")
	log.Panic("this is panic message")
}

func TestNewDevelopment(t *testing.T) {
	log, _ := zap.NewDevelopment()
	log.Debug("this is debug message")
	log.Info("this is info message")
	log.Info("this is info message with fileds",
		zap.Int("age", 24), zap.String("agender", "man"))
	//log.Warn("this is warn message")
	//log.Error("this is error message")
	// log.DPanic("This is a DPANIC message")
	// log.Panic("this is panic message")
	// log.Fatal("This is a FATAL message")
}

func TestNewProduction(t *testing.T) {
	log, _ := zap.NewProduction()
	log.Debug("this is debug message")
	log.Info("this is info message")
	log.Info("this is info message with fileds",
		zap.Int("age", 24), zap.String("agender", "man"))
	log.Warn("this is warn message")
	log.Error("this is error message")
}
/*
Example和Production使用的是 json 格式输出，Development 使用行的形式输出
Development
从警告级别向上打印到堆栈中来跟踪
始终打印包/文件/行（方法）
在行尾添加任何额外字段作为 json 字符串
以大写形式打印级别名称
以毫秒为单位打印 ISO8601 格式的时间戳
Production
调试级别消息不记录
Error , Dpanic 级别的记录，会在堆栈中跟踪文件， Warn 不会
始终将调用者添加到文件中
以时间戳格式打印日期
以小写形式打印级别名称
在上面的代码中，我们首先创建了一个 Logger ，然后使用 Info / Error 等 Logger 方法记录消息。
 */

func TestPrintf(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	slogger := logger.Sugar()

	slogger.Debugf("debug message age is %d, agender is %s", 19, "man")
	slogger.Info("Info() uses sprint")
	slogger.Infof("Infof() uses %s", "sprintf")
	slogger.Infow("Infow() allows tags", "name", "Legolas", "type", 1)
}
/*
 把日志吸入文件
 */
func TestLoggerToFile(t *testing.T){

}