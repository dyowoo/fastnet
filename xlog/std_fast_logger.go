/**
* @File: std_fast_logger.go
* @Author: Jason Woo
* @Date: 2023/6/30 15:33
**/

package xlog

/*

   全局默认提供一个Log对外句柄，可以直接使用API系列调用
   全局日志对象 StdFastLog
   注意：本文件方法不支持自定义，无法替换日志记录模式，如果需要自定义Logger:

   请使用如下方法:
   xlog.SetLogger(yourLogger)
   xlog.Ins().InfoF()等方法
*/

// StdFastLog creates a global log
var StdFastLog = NewFastLog("", BitDefault)

// Flags gets the flags of StdFastLog
func Flags() int {
	return StdFastLog.Flags()
}

// ResetFlags sets the flags of StdFastLog
func ResetFlags(flag int) {
	StdFastLog.ResetFlags(flag)
}

// AddFlag adds a flag to StdFastLog
func AddFlag(flag int) {
	StdFastLog.AddFlag(flag)
}

// SetPrefix sets the log prefix of StdFastLog
func SetPrefix(prefix string) {
	StdFastLog.SetPrefix(prefix)
}

// SetLogFile sets the log file of StdFastLog
func SetLogFile(fileDir string, fileName string) {
	StdFastLog.SetLogFile(fileDir, fileName)
}

// SetMaxAge 最大保留天数
func SetMaxAge(ma int) {
	StdFastLog.SetMaxAge(ma)
}

// SetMaxSize 单个日志最大容量 单位：字节
func SetMaxSize(ms int64) {
	StdFastLog.SetMaxSize(ms)
}

// SetCons 同时输出控制台
func SetCons(b bool) {
	StdFastLog.SetConsole(b)
}

// SetLogLevel sets the log level of StdFastLog
func SetLogLevel(logLevel int) {
	StdFastLog.SetLogLevel(logLevel)
}

func DebugF(format string, v ...interface{}) {
	StdFastLog.DebugF(format, v...)
}

func Debug(v ...interface{}) {
	StdFastLog.Debug(v...)
}

func InfoF(format string, v ...interface{}) {
	StdFastLog.InfoF(format, v...)
}

func Info(v ...interface{}) {
	StdFastLog.Info(v...)
}

func WarnF(format string, v ...interface{}) {
	StdFastLog.WarnF(format, v...)
}

func Warn(v ...interface{}) {
	StdFastLog.Warn(v...)
}

func ErrorF(format string, v ...interface{}) {
	StdFastLog.ErrorF(format, v...)
}

func Error(v ...interface{}) {
	StdFastLog.Error(v...)
}

func FatalF(format string, v ...interface{}) {
	StdFastLog.FatalF(format, v...)
}

func Fatal(v ...interface{}) {
	StdFastLog.Fatal(v...)
}

func PanicF(format string, v ...interface{}) {
	StdFastLog.PanicF(format, v...)
}

func Panic(v ...interface{}) {
	StdFastLog.Panic(v...)
}

func Stack(v ...interface{}) {
	StdFastLog.Stack(v...)
}

func init() {
	// 因为StdFastLog对象 对所有输出方法做了一层包裹，所以在打印调用函数的时候，比正常的logger对象多一层调用
	// 一般的fastLogger对象 calledDepth=2, StdFastLog的calledDepth=3
	StdFastLog.calledDepth = 3
}
