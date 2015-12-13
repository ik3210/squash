package log

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"time"
)

//日志级别 Debug < Release < Error < Fatal
const (
	_            = iota
	debugLevel   //非关键日志
	releaseLevel //关键日志
	errorLevel   //错误日志
	fatalLevel   //致命错误日志。每次输出Fatal日志之后游戏服务器进程就会结束
)

//日志输出前缀字符串
const (
	printDebugLevel   = "[debug  ] "
	printReleaseLevel = "[release] "
	printErrorLevel   = "[error  ] "
	printFatalLevel   = "[fatal  ] "
)

//上层Logger
type Logger struct {
	level      int         //日志级别
	baseLogger *log.Logger //底层logger
	baseFile   *os.File    //日志写入的文件
}

//创建上层logger
func New(strLevel string, pathname string) (*Logger, error) {
	var level int

	//设置日志级别
	switch strings.ToLower(strLevel) {
	case "debug":
		level = debugLevel
	case "release":
		level = releaseLevel
	case "error":
		level = errorLevel
	case "fatal":
		level = fatalLevel
	default:
		return nil, errors.New("unknown level: " + strLevel)
	}

	var baseLogger *log.Logger
	var baseFile *os.File

	if pathname != "" { //日志写入到文件
		now := time.Now()

		//文件名以时间命名
		filename := fmt.Sprintf("%d%02d%02d_%02d_%02d_%02d.log",
			now.Year(),
			now.Month(),
			now.Day(),
			now.Hour(),
			now.Minute(),
			now.Second())

		//创建文件
		file, err := os.Create(path.Join(pathname, filename))
		//创建失败
		if err != nil {
			return nil, err
		}

		//创建底层logger
		baseLogger = log.New(file, "", log.LstdFlags)
		//保存文件引用
		baseFile = file
	} else { //日志输出到标准输出
		baseLogger = log.New(os.Stdout, "", log.LstdFlags)
	}

	//创建上层logger
	logger := new(Logger)
	//设置日志级别
	logger.level = level
	//保存底层logger
	logger.baseLogger = baseLogger
	//保存文件引用
	logger.baseFile = baseFile

	return logger, nil
}

//关闭上层logger
func (logger *Logger) Close() {
	//写入文件存在，关闭文件
	if logger.baseFile != nil {
		logger.baseFile.Close()
	}

	//置空字段
	logger.baseLogger = nil
	logger.baseFile = nil
}

//上层logger输出日志
func (logger *Logger) doPrintf(level int, printLevel string, format string, a ...interface{}) {
	//日志级别小于设定的日志级别
	if level < logger.level {
		return
	}

	//底层logger为空
	if logger.baseLogger == nil {
		panic("logger closed")
	}

	//输出日志
	format = printLevel + format //前缀+格式
	logger.baseLogger.Printf(format, a...)

	//日志级别为fatal，退出程序
	if level == fatalLevel {
		os.Exit(1)
	}
}

//上层logger输出Debug日志
func (logger *Logger) Debug(format string, a ...interface{}) {
	logger.doPrintf(debugLevel, printDebugLevel, format, a...)
}

//上层logger输出Release日志
func (logger *Logger) Release(format string, a ...interface{}) {
	logger.doPrintf(releaseLevel, printReleaseLevel, format, a...)
}

//上层logger输出Error日志
func (logger *Logger) Error(format string, a ...interface{}) {
	logger.doPrintf(errorLevel, printErrorLevel, format, a...)
}

//上层logger输出Fatal日志
func (logger *Logger) Fatal(format string, a ...interface{}) {
	logger.doPrintf(fatalLevel, printFatalLevel, format, a...)
}

//创建一个默认的logger，日志级别为debug（使用者不必自定义logger，直接引入包就可以输出日志）
var gLogger, _ = New("debug", "")

//包导出函数，传入一个logger替换默认的gLogger
func Export(logger *Logger) {
	if logger != nil {
		gLogger = logger
	}
}

//包级别输出Debug日志
func Debug(format string, a ...interface{}) {
	gLogger.Debug(format, a...)
}

//包级别输出Release日志
func Release(format string, a ...interface{}) {
	gLogger.Release(format, a...)
}

//包级别输出Error日志
func Error(format string, a ...interface{}) {
	gLogger.Error(format, a...)
}

//包级别输出Fatal日志
func Fatal(format string, a ...interface{}) {
	gLogger.Fatal(format, a...)
}

//gLogger关闭
func Close() {
	gLogger.Close()
}
