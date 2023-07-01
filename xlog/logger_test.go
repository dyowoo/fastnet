/**
* @File: logger_test.go
* @Author: Jason Woo
* @Date: 2023/6/30 15:36
**/

package xlog_test

import (
	"github.com/dyowoo/fastnet/xlog"
	"testing"
)

func TestLogger(t *testing.T) {
	xlog.Info("fastnet xlog info")
}
