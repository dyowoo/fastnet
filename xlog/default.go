/**
* @File: default.go
* @Author: Jason Woo
* @Date: 2023/6/30 15:32
**/

package xlog

import (
	"context"
	"fmt"
)

var Logger ILogger

type fastDefaultLog struct{}

func (log *fastDefaultLog) InfoF(format string, v ...interface{}) {
	StdFastLog.InfoF(format, v...)
}

func (log *fastDefaultLog) ErrorF(format string, v ...interface{}) {
	StdFastLog.ErrorF(format, v...)
}

func (log *fastDefaultLog) DebugF(format string, v ...interface{}) {
	StdFastLog.DebugF(format, v...)
}

func (log *fastDefaultLog) InfoFX(ctx context.Context, format string, v ...interface{}) {
	fmt.Println(ctx)
	StdFastLog.InfoF(format, v...)
}

func (log *fastDefaultLog) ErrorFX(ctx context.Context, format string, v ...interface{}) {
	fmt.Println(ctx)
	StdFastLog.ErrorF(format, v...)
}

func (log *fastDefaultLog) DebugFX(ctx context.Context, format string, v ...interface{}) {
	fmt.Println(ctx)
	StdFastLog.DebugF(format, v...)
}

func SetLogger(logger ILogger) {
	Logger = logger
}

func init() {
	Logger = new(fastDefaultLog)
}
