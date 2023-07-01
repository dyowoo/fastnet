/**
* @File: global_obj.go
* @Author: Jason Woo
* @Date: 2023/6/30 16:21
**/

package xconf

import (
	"encoding/json"
	"fmt"
	"github.com/dyowoo/fastnet/xlog"
	"github.com/dyowoo/fastnet/xutils/commandline/args"
	"github.com/dyowoo/fastnet/xutils/commandline/uflag"
	"os"
	"reflect"
	"testing"
	"time"
)

const (
	ServerModeTcp       = "tcp"
	ServerModeWebsocket = "websocket"
)

const (
	WorkerModeHash = "Hash" // 默认使用取余的方式
	WorkerModeBind = "Bind" // 为每个连接分配一个worker
)

// Config
/*
存储一切有关框架的全局参数，供其他模块使用
一些参数也可以通过 用户根据 fastnet2.json来配置
*/
type Config struct {
	Host              string // 当前服务器主机IP
	TCPPort           int    // 当前服务器主机监听端口号
	WsPort            int    // 当前服务器主机websocket监听端口
	Name              string // 当前服务器名称
	Version           string // 当前版本号
	MaxPacketSize     uint32 // 读写数据包的最大值
	MaxConn           int    // 当前服务器主机允许的最大链接个数
	WorkerPoolSize    uint32 // 业务工作Worker池的数量
	MaxWorkerTaskLen  uint32 // 业务工作Worker对应负责的任务队列最大任务存储数量
	WorkerMode        string // 为链接分配worker的方式
	MaxMsgChanLen     uint32 // SendBuffMsg发送消息的缓冲最大长度
	IOReadBuffSize    uint32 // 每次IO最大的读取长度
	Mode              string // "tcp":tcp监听, "websocket":websocket 监听 为空时同时开启
	RouterSlicesMode  bool   // 路由模式 false为旧版本路由，true为启用新版本的路由 默认使用旧版本
	LogDir            string // 日志所在文件夹 默认"./log"
	LogFile           string // 日志文件名称   默认""  --如果没有设置日志文件，打印信息将打印至stderr
	LogSaveDays       int    // 日志最大保留天数
	LogFileSize       int64  // 日志单个日志最大容量 默认 64MB,单位：字节，记得一定要换算成MB（1024 * 1024）
	LogCons           bool   // 日志标准输出  默认 false
	LogIsolationLevel int    // 日志隔离级别  -- 0：全开 1：关debug 2：关debug/info 3：关debug/info/warn ...
	HeartbeatMax      int    // 最长心跳检测间隔时间(单位：秒),超过改时间间隔，则认为超时，从配置文件读取
	CertFile          string //  证书文件名称 默认""
	PrivateKeyFile    string //  私钥文件名称 默认"" --如果没有设置证书和私钥文件，则不启用TLS加密
}

// GlobalObject 定义一个全局的对象
var GlobalObject *Config

// PathExists  判断一个文件是否存在
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// Reload 读取用户的配置文件
func (g *Config) Reload() {
	confFilePath := args.Args.ConfigFile
	if confFileExists, _ := PathExists(confFilePath); confFileExists != true {
		// 配置文件不存在也需要用默认参数初始化日志模块配置
		g.InitLogConfig()

		xlog.ErrorF("config file %s is not exist!!", confFilePath)
		return
	}

	data, err := os.ReadFile(confFilePath)
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(data, g)
	if err != nil {
		panic(err)
	}

	g.InitLogConfig()
}

// Show 打印配置信息
func (g *Config) Show() {
	objVal := reflect.ValueOf(g).Elem()
	objType := reflect.TypeOf(*g)

	fmt.Println("===== Fastnet Global Config =====")
	for i := 0; i < objVal.NumField(); i++ {
		field := objVal.Field(i)
		typeField := objType.Field(i)

		fmt.Printf("%s: %v\n", typeField.Name, field.Interface())
	}
	fmt.Println("==============================")
}

func (g *Config) HeartbeatMaxDuration() time.Duration {
	return time.Duration(g.HeartbeatMax) * time.Second
}

func (g *Config) InitLogConfig() {
	if g.LogFile != "" {
		xlog.SetLogFile(g.LogDir, g.LogFile)
		xlog.SetCons(g.LogCons)
	}
	if g.LogSaveDays > 0 {
		xlog.SetMaxAge(g.LogSaveDays)
	}
	if g.LogFileSize > 0 {
		xlog.SetMaxSize(g.LogFileSize)
	}
	if g.LogIsolationLevel > xlog.LogDebug {
		xlog.SetLogLevel(g.LogIsolationLevel)
	}
}

func init() {
	pwd, err := os.Getwd()
	if err != nil {
		pwd = "."
	}

	args.InitConfigFlag(pwd+"/conf/fastnet.json", "The configuration file defaults to <exeDir>/conf/fastnet.json if it is not set.")

	// 防止 go test 出现"flag provided but not defined: -test.panic on exit0"等错误
	testing.Init()
	uflag.Parse()

	args.FlagHandle()

	// 初始化GlobalObject变量，设置一些默认值
	GlobalObject = &Config{
		Name:              "FastnetServerApp",
		Version:           "V1.0",
		TCPPort:           29000,
		WsPort:            28000,
		Host:              "0.0.0.0",
		MaxConn:           12000,
		MaxPacketSize:     4096,
		WorkerPoolSize:    10,
		MaxWorkerTaskLen:  1024,
		WorkerMode:        "",
		MaxMsgChanLen:     1024,
		LogDir:            pwd + "/log",
		LogFile:           "", // 默认日志文件为空，打印到stderr
		LogIsolationLevel: 0,
		HeartbeatMax:      10, // 默认心跳检测最长间隔为10秒
		IOReadBuffSize:    1024,
		CertFile:          "",
		PrivateKeyFile:    "",
		Mode:              ServerModeTcp,
		RouterSlicesMode:  true,
	}

	// 从配置文件中加载一些用户配置的参数
	GlobalObject.Reload()
}
