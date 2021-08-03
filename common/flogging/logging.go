package flogging

import (
	"io"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/op/go-logging"
)

const (
	pkgLogID      = "flogging"
	//默认日志输出
	defaultFormat = "%{color}%{time:2006-01-02 15:04:05.000 MST} [%{module}] %{shortfunc} -> %{level:.4s} %{id:03x}%{color:reset} %{message}"
	//等级
	defaultLevel  = logging.INFO
)

var (
	//自带一个名为flogging的日志记录者logger，规定默认的日志输出和等级，
	logger *logging.Logger

	defaultOutput *os.File //默认输出端

	//日志级别映射
	modules          map[string]string //存放素有fabric模块日志级别
	peerStartModules map[string]string //存放每个peer启动之时日志级别映射,每个peer启动完成之时调用setPeerStartup初始化
    //并且可以通过调用revertToPeerStartupLever()恢复初始值
	lock sync.RWMutex
	once sync.Once
)
/*
   通过代用reset函数初始化一系列默认值
 */
func init() {
	//创建一个名字的日志对象
	logger = logging.MustGetLogger(pkgLogID)
	Reset()
	initgrpclogger()//初始化grpc的日志对象
}

// Reset sets to logging to the defaults defined in this package.
func Reset() {
	modules = make(map[string]string)
	lock = sync.RWMutex{}

	defaultOutput = os.Stderr
	InitBackend(SetFormat(defaultFormat), defaultOutput)
	InitFromSpec("")
}

// SetFormat sets the logging format.
func SetFormat(formatSpec string) logging.Formatter {
	if formatSpec == "" {
		formatSpec = defaultFormat
	}
	//创建一个日志输出对象format，确定输入出格式
	return logging.MustStringFormatter(formatSpec)
}

// InitBackend sets up the logging backend based on
// the provided logging formatter and I/O writer.
func InitBackend(formatter logging.Formatter, output io.Writer) {
	//创建一个日志输出对象backend,也就打印的位置,
	backend := logging.NewLogBackend(output, "", 0)
	//将输出格式和输出对象绑定
	backendFormatter := logging.NewBackendFormatter(backend, formatter)
	//将绑定了格式的输出对象设置为日志的输出对象
	//log打印会按格式输出到backendFormatter所代表的对象里
	logging.SetBackend(backendFormatter).SetLevel(defaultLevel, "")
}

// DefaultLevel returns the fallback value for loggers to use if parsing fails.
func DefaultLevel() string {
	return defaultLevel.String()
}

// GetModuleLevel gets the current logging level for the specified module.
func GetModuleLevel(module string) string {
	// logging.GetLevel() returns the logging level for the module, if defined.
	// Otherwise, it returns the default logging level, as set by
	// `flogging/logging.go`.
	level := logging.GetLevel(module).String()
	return level
}

// SetModuleLevel sets the logging level for the modules that match the supplied
// regular expression. Can be used to dynamically change the log level for the
// module.
func SetModuleLevel(moduleRegExp string, level string) (string, error) {
	return setModuleLevel(moduleRegExp, level, true, false)
}

func setModuleLevel(moduleRegExp string, level string, isRegExp bool, revert bool) (string, error) {
	var re *regexp.Regexp
	logLevel, err := logging.LogLevel(level)
	if err != nil {
		logger.Warningf("Invalid logging level '%s' - ignored", level)
	} else {
		if !isRegExp || revert {
			logging.SetLevel(logLevel, moduleRegExp)
			logger.Debugf("Module '%s' logger enabled for log level '%s'", moduleRegExp, level)
		} else {
			re, err = regexp.Compile(moduleRegExp)
			if err != nil {
				logger.Warningf("Invalid regular expression: %s", moduleRegExp)
				return "", err
			}
			lock.Lock()
			defer lock.Unlock()
			for module := range modules {
				if re.MatchString(module) {
					logging.SetLevel(logging.Level(logLevel), module)
					modules[module] = logLevel.String()
					logger.Debugf("Module '%s' logger enabled for log level '%s'", module, logLevel)
				}
			}
		}
	}
	return logLevel.String(), err
}

// MustGetLogger is used in place of `logging.MustGetLogger` to allow us to
// store a map of all modules and submodules that have loggers in the system.
func MustGetLogger(module string) *logging.Logger {
	l := logging.MustGetLogger(module)
	lock.Lock()
	defer lock.Unlock()
	modules[module] = GetModuleLevel(module)
	return l
}

// InitFromSpec initializes the logging based on the supplied spec. It is
// exposed externally so that consumers of the flogging package may parse their
// own logging specification. The logging specification has the following form:
//		[<module>[,<module>...]=]<level>[:[<module>[,<module>...]=]<level>...]
func InitFromSpec(spec string) string {
	levelAll := defaultLevel
	var err error

	if spec != "" {
		fields := strings.Split(spec, ":")
		for _, field := range fields {
			split := strings.Split(field, "=")
			switch len(split) {
			case 1:
				if levelAll, err = logging.LogLevel(field); err != nil {
					logger.Warningf("Logging level '%s' not recognized, defaulting to '%s': %s", field, defaultLevel, err)
					levelAll = defaultLevel // need to reset cause original value was overwritten
				}
			case 2:
				// <module>[,<module>...]=<level>
				levelSingle, err := logging.LogLevel(split[1])
				if err != nil {
					logger.Warningf("Invalid logging level in '%s' ignored", field)
					continue
				}

				if split[0] == "" {
					logger.Warningf("Invalid logging override specification '%s' ignored - no module specified", field)
				} else {
					modules := strings.Split(split[0], ",")
					for _, module := range modules {
						logger.Debugf("Setting logging level for module '%s' to '%s'", module, levelSingle)
						logging.SetLevel(levelSingle, module)
					}
				}
			default:
				logger.Warningf("Invalid logging override '%s' ignored - missing ':'?", field)
			}
		}
	}

	logging.SetLevel(levelAll, "") // set the logging level for all modules

	// iterate through modules to reload their level in the modules map based on
	// the new default level
	for k := range modules {
		MustGetLogger(k)
	}
	// register flogging logger in the modules map
	MustGetLogger(pkgLogID)

	return levelAll.String()
}

// SetPeerStartupModulesMap saves the modules and their log levels.
// this function should only be called at the end of peer startup.
func SetPeerStartupModulesMap() {
	lock.Lock()
	defer lock.Unlock()

	once.Do(func() {
		peerStartModules = make(map[string]string)
		for k, v := range modules {
			peerStartModules[k] = v
		}
	})
}

// GetPeerStartupLevel returns the peer startup level for the specified module.
// It will return an empty string if the input parameter is empty or the module
// is not found
func GetPeerStartupLevel(module string) string {
	if module != "" {
		if level, ok := peerStartModules[module]; ok {
			return level
		}
	}

	return ""
}

// RevertToPeerStartupLevels reverts the log levels for all modules to the level
// defined at the end of peer startup.
func RevertToPeerStartupLevels() error {
	lock.RLock()
	defer lock.RUnlock()
	for key := range peerStartModules {
		_, err := setModuleLevel(key, peerStartModules[key], false, true)
		if err != nil {
			return err
		}
	}
	logger.Info("Log levels reverted to the levels defined at the end of peer startup")
	return nil
}
