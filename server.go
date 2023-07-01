/**
* @File: server.go
* @Author: Jason Woo
* @Date: 2023/6/30 15:17
**/

package fastnet

import (
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/dyowoo/fastnet/xconf"
	"github.com/dyowoo/fastnet/xlog"
	"github.com/gorilla/websocket"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

// IServer Defines the server interface
type IServer interface {
	Start()                                                                // 启动服务器方法
	Stop()                                                                 // 停止服务器方法
	Serve()                                                                // 开启业务服务方法
	AddRouter(msgID uint32, router IRouter)                                // 路由功能：给当前服务注册一个路由业务方法，供客户端链接处理使用
	AddRouterSlices(msgID uint32, router ...RouterHandler) IRouterSlices   // 新版路由方式
	Group(start, end uint32, Handlers ...RouterHandler) IGroupRouterSlices // 路由组管理
	Use(Handlers ...RouterHandler) IRouterSlices                           // 公共组件管理
	GetConnMgr() IConnManager                                              // 得到链接管理
	SetOnConnStart(func(IConnection))                                      // 设置该Server的连接创建时Hook函数
	SetOnConnStop(func(IConnection))                                       // 设置该Server的连接断开时的Hook函数
	GetOnConnStart() func(IConnection)                                     // 得到该Server的连接创建时Hook函数
	GetOnConnStop() func(IConnection)                                      // 得到该Server的连接断开时的Hook函数
	GetPacket() IDataPack                                                  // 获取Server绑定的数据协议封包方式
	GetMsgHandler() IMsgHandle                                             // 获取Server绑定的消息处理模块
	SetPacket(IDataPack)                                                   // 设置Server绑定的数据协议封包方式
	StartHeartbeat(time.Duration)                                          // 启动心跳检测
	StartHeartbeatWithOption(time.Duration, *HeartbeatOption)              // 启动心跳检测(自定义回调)
	GetHeartbeat() IHeartbeatChecker                                       // 获取心跳检测器
	GetLengthField() *LengthField                                          //
	SetDecoder(IDecoder)                                                   //
	AddInterceptor(IInterceptor)                                           //
	SetWebsocketAuth(func(r *http.Request) error)                          // 添加websocket认证方法
	ServerName() string                                                    // 获取服务器名称
}

// Server 接口实现，定义一个Server服务类
type Server struct {
	name             string // 服务器的名称
	ipVersion        string
	ip               string                 // 服务绑定的IP地址
	port             int                    // 服务绑定的端口
	wsPort           int                    // 服务绑定的websocket 端口 (Websocket port the server is bound to)
	msgHandler       IMsgHandle             // 当前Server的消息管理模块，用来绑定MsgID和对应的处理方法
	routerSlicesMode bool                   // 路由模式
	connMgr          IConnManager           // 当前Server的链接管理器
	onConnStart      func(conn IConnection) // 该Server的连接创建时Hook函数
	onConnStop       func(conn IConnection) // 该Server的连接断开时的Hook函数
	packet           IDataPack              // 数据报文封包方式
	exitChan         chan struct{}          // 异步捕获链接关闭状态
	decoder          IDecoder               // 断粘包解码器
	heartbeatChecker IHeartbeatChecker      // 心跳检测器
	upgrader         *websocket.Upgrader
	websocketAuth    func(r *http.Request) error
	cID              uint64
}

// 根据config创建一个服务器句柄
func newServerWithConfig(config *xconf.Config, ipVersion string, opts ...Option) IServer {
	PrintLogo()

	s := &Server{
		name:             config.Name,
		ipVersion:        ipVersion,
		ip:               config.Host,
		port:             config.TCPPort,
		wsPort:           config.WsPort,
		msgHandler:       newMsgHandle(),
		routerSlicesMode: config.RouterSlicesMode,
		connMgr:          newConnManager(),
		exitChan:         nil,
		packet:           Factory().NewPack(FastDataPack),
		decoder:          NewTLVDecoder(), // 默认使用TLV的解码方式
		upgrader: &websocket.Upgrader{
			ReadBufferSize: int(config.IOReadBuffSize),
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}

	for _, opt := range opts {
		opt(s)
	}

	// 提示当前配置信息
	//config.Show()

	return s
}

// NewServer 创建一个服务器句柄
func NewServer(opts ...Option) IServer {
	return newServerWithConfig(xconf.GlobalObject, "tcp", opts...)
}

// NewUserConfServer 创建一个服务器句柄
func NewUserConfServer(config *xconf.Config, opts ...Option) IServer {
	// 刷新用户配置到全局配置变量
	xconf.UserConfToGlobal(config)

	s := newServerWithConfig(config, "tcp4", opts...)
	return s
}

// NewDefaultRouterSlicesServer 创建一个默认自带一个Recover处理器的服务器句柄
func NewDefaultRouterSlicesServer(opts ...Option) IServer {
	xconf.GlobalObject.RouterSlicesMode = true
	s := newServerWithConfig(xconf.GlobalObject, "tcp", opts...)
	s.Use(RouterRecovery)
	return s
}

// NewUserConfDefaultRouterSlicesServer 创建一个用户配置的自带一个Recover处理器的服务器句柄，如果用户不希望Use这个方法，那么应该使用NewUserConfServer
func NewUserConfDefaultRouterSlicesServer(config *xconf.Config, opts ...Option) IServer {
	if !config.RouterSlicesMode {
		panic("routerSlicesMode is false")
	}

	// 刷新用户配置到全局配置变量
	xconf.UserConfToGlobal(config)

	s := newServerWithConfig(xconf.GlobalObject, "tcp4", opts...)
	s.Use(RouterRecovery)
	return s
}

func (s *Server) StartConn(conn IConnection) {
	if s.heartbeatChecker != nil {
		heartBeatChecker := s.heartbeatChecker.Clone()

		heartBeatChecker.BindConn(conn)
	}

	conn.Start()
}

func (s *Server) ListenTcpConn() {
	addr, err := net.ResolveTCPAddr(s.ipVersion, fmt.Sprintf("%s:%d", s.ip, s.port))
	if err != nil {
		xlog.ErrorF("[start] resolve tcp addr err: %v\n", err)
		return
	}

	var listener net.Listener
	if xconf.GlobalObject.CertFile != "" && xconf.GlobalObject.PrivateKeyFile != "" {
		crt, err := tls.LoadX509KeyPair(xconf.GlobalObject.CertFile, xconf.GlobalObject.PrivateKeyFile)
		if err != nil {
			panic(err)
		}

		tlsConfig := &tls.Config{}
		tlsConfig.Certificates = []tls.Certificate{crt}
		tlsConfig.Time = time.Now
		tlsConfig.Rand = rand.Reader
		listener, err = tls.Listen(s.ipVersion, fmt.Sprintf("%s:%d", s.ip, s.port), tlsConfig)
		if err != nil {
			panic(err)
		}
	} else {
		listener, err = net.ListenTCP(s.ipVersion, addr)
		if err != nil {
			panic(err)
		}
	}

	go func() {
		for {
			// 设置服务器最大连接控制,如果超过最大连接，则等待
			if s.connMgr.Len() >= xconf.GlobalObject.MaxConn {
				xlog.InfoF("exceeded the maxConnNum:%d, wait:%d", xconf.GlobalObject.MaxConn, AcceptDelay.duration)
				AcceptDelay.Delay()
				continue
			}
			// 阻塞等待客户端建立连接请求
			conn, err := listener.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					xlog.ErrorF("listener closed")
					return
				}
				xlog.ErrorF("accept err: %v", err)
				AcceptDelay.Delay()
				continue
			}

			AcceptDelay.Reset()

			// 处理该新连接请求的 业务 方法， 此时应该有 handler 和 conn是绑定的
			newCid := atomic.AddUint64(&s.cID, 1)
			dealConn := newServerConn(s, conn, newCid)

			go s.StartConn(dealConn)

		}
	}()

	select {
	case <-s.exitChan:
		err := listener.Close()
		if err != nil {
			xlog.ErrorF("listener close err: %v", err)
		}
	}
}

func (s *Server) ListenWebsocketConn() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 设置服务器最大连接控制,如果超过最大连接，则等待
		if s.connMgr.Len() >= xconf.GlobalObject.MaxConn {
			xlog.InfoF("exceeded the maxConnNum:%d, wait:%d", xconf.GlobalObject.MaxConn, AcceptDelay.duration)
			AcceptDelay.Delay()
			return
		}

		// 如果需要 websocket 认证请设置认证信息
		if s.websocketAuth != nil {
			err := s.websocketAuth(r)
			if err != nil {
				xlog.ErrorF(" websocket auth err:%v", err)
				w.WriteHeader(401)
				AcceptDelay.Delay()
				return
			}
		}

		// 判断 header 里面是有子协议
		if len(r.Header.Get("Sec-Websocket-Protocol")) > 0 {
			s.upgrader.Subprotocols = websocket.Subprotocols(r)
		}

		// 升级成 websocket 连接
		conn, err := s.upgrader.Upgrade(w, r, nil)
		if err != nil {
			xlog.ErrorF("new websocket err:%v", err)
			w.WriteHeader(500)
			AcceptDelay.Delay()
			return
		}
		AcceptDelay.Reset()

		// 处理该新连接请求的 业务 方法， 此时应该有 handler 和 conn是绑定的
		newCid := atomic.AddUint64(&s.cID, 1)
		wsConn := newWebsocketConn(s, conn, newCid)

		go s.StartConn(wsConn)
	})

	err := http.ListenAndServe(fmt.Sprintf("%s:%d", s.ip, s.wsPort), nil)
	if err != nil {
		panic(err)
	}
}

// Start 开启网络服务
func (s *Server) Start() {
	xlog.InfoF("[start] server name: %s,listener at ip: %s, port %d is starting", s.name, s.ip, s.port)
	s.exitChan = make(chan struct{})

	// 将解码器添加到拦截器
	if s.decoder != nil {
		s.msgHandler.AddInterceptor(s.decoder)
	}

	// 启动worker工作池机制
	s.msgHandler.StartWorkerPool()

	// 开启一个go去做服务端Listener业务
	switch xconf.GlobalObject.Mode {
	case xconf.ServerModeTcp:
		go s.ListenTcpConn()
	case xconf.ServerModeWebsocket:
		go s.ListenWebsocketConn()
	default:
		go s.ListenTcpConn()
		go s.ListenWebsocketConn()
	}
}

// Stop 停止服务
func (s *Server) Stop() {
	xlog.InfoF("[stop] fastnet2 server, name %s", s.name)

	// 将其他需要清理的连接信息或者其他信息 也要一并停止或者清理
	s.connMgr.ClearConn()
	s.exitChan <- struct{}{}
	close(s.exitChan)
}

// Serve 运行服务
func (s *Server) Serve() {
	s.Start()
	// 阻塞,否则主Go退出
	c := make(chan os.Signal, 1)
	// 监听指定信号 ctrl+c kill信号
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	sig := <-c
	xlog.InfoF("[serve] fastnet2 server, name %s, serve interrupt, signal = %v", s.name, sig)
}

func (s *Server) AddRouter(msgID uint32, router IRouter) {
	if s.routerSlicesMode {
		panic("server routerSlicesMode is true ")
	}
	s.msgHandler.AddRouter(msgID, router)
}

func (s *Server) AddRouterSlices(msgID uint32, router ...RouterHandler) IRouterSlices {
	if !s.routerSlicesMode {
		panic("server routerSlicesMode is false ")
	}
	return s.msgHandler.AddRouterSlices(msgID, router...)
}

func (s *Server) Group(start, end uint32, Handlers ...RouterHandler) IGroupRouterSlices {
	if !s.routerSlicesMode {
		panic("server routerSlicesMode is false")
	}
	return s.msgHandler.Group(start, end, Handlers...)
}

func (s *Server) Use(Handlers ...RouterHandler) IRouterSlices {
	if !s.routerSlicesMode {
		panic("server routerSlicesMode is false")
	}
	return s.msgHandler.Use(Handlers...)
}

func (s *Server) GetConnMgr() IConnManager {
	return s.connMgr
}

func (s *Server) SetOnConnStart(hookFunc func(IConnection)) {
	s.onConnStart = hookFunc
}

func (s *Server) SetOnConnStop(hookFunc func(IConnection)) {
	s.onConnStop = hookFunc
}

func (s *Server) GetOnConnStart() func(IConnection) {
	return s.onConnStart
}

func (s *Server) GetOnConnStop() func(IConnection) {
	return s.onConnStop
}

func (s *Server) GetPacket() IDataPack {
	return s.packet
}

func (s *Server) SetPacket(packet IDataPack) {
	s.packet = packet
}

func (s *Server) GetMsgHandler() IMsgHandle {
	return s.msgHandler
}

// StartHeartbeat 启动心跳检测
// interval 每次发送心跳的时间间隔
func (s *Server) StartHeartbeat(interval time.Duration) {
	checker := NewHeartbeatChecker(interval)

	// 添加心跳检测的路由
	// 检测当前路由模式
	if s.routerSlicesMode {
		s.AddRouterSlices(checker.MsgID(), checker.RouterSlices()...)
	} else {
		s.AddRouter(checker.MsgID(), checker.Router())
	}

	// server绑定心跳检测器
	s.heartbeatChecker = checker
}

// StartHeartbeatWithOption 启动心跳检测
// option 心跳检测的配置
func (s *Server) StartHeartbeatWithOption(interval time.Duration, option *HeartbeatOption) {
	checker := NewHeartbeatChecker(interval)

	if option != nil {
		checker.SetHeartbeatMsgFunc(option.MakeMsg)
		checker.SetOnRemoteNotAlive(option.OnRemoteNotAlive)
		// 检测当前路由模式
		if s.routerSlicesMode {
			checker.BindRouterSlices(option.HeartbeatMsgID, option.RouterSlices...)
		} else {
			checker.BindRouter(option.HeartbeatMsgID, option.Router)
		}
	}

	// 添加心跳检测的路由
	// 检测当前路由模式
	if s.routerSlicesMode {
		s.AddRouterSlices(checker.MsgID(), checker.RouterSlices()...)
	} else {
		s.AddRouter(checker.MsgID(), checker.Router())
	}

	// server绑定心跳检测器
	s.heartbeatChecker = checker
}

func (s *Server) GetHeartbeat() IHeartbeatChecker {
	return s.heartbeatChecker
}

func (s *Server) SetDecoder(decoder IDecoder) {
	s.decoder = decoder
}

func (s *Server) GetLengthField() *LengthField {
	if s.decoder != nil {
		return s.decoder.GetLengthField()
	}
	return nil
}

func (s *Server) AddInterceptor(interceptor IInterceptor) {
	s.msgHandler.AddInterceptor(interceptor)
}

func (s *Server) SetWebsocketAuth(f func(r *http.Request) error) {
	s.websocketAuth = f
}

func (s *Server) ServerName() string {
	return s.name
}

func init() {}
