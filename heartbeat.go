/**
* @File: heartbeat.go
* @Author: Jason Woo
* @Date: 2023/6/30 15:16
**/

package fastnet

import (
	"fmt"
	"github.com/dyowoo/fastnet/xlog"
	"time"
)

const (
	HeartbeatDefaultMsgID uint32 = 99999
)

type IHeartbeatChecker interface {
	SetOnRemoteNotAlive(OnRemoteNotAlive)
	SetHeartbeatMsgFunc(HeartbeatMsgFunc)
	SetHeartbeatFunc(HeartbeatFunc)
	BindRouter(uint32, IRouter)
	BindRouterSlices(uint32, ...RouterHandler)
	Start()
	Stop()
	SendHeartbeatMsg() error
	BindConn(IConnection)
	Clone() IHeartbeatChecker
	MsgID() uint32
	Router() IRouter
	RouterSlices() []RouterHandler
}

// HeartbeatMsgFunc 用户自定义的心跳检测消息处理方法
type HeartbeatMsgFunc func(IConnection) []byte

// HeartbeatFunc 用户自定义心跳函数
type HeartbeatFunc func(IConnection) error

// OnRemoteNotAlive 用户自定义的远程连接不存活时的处理方法
type OnRemoteNotAlive func(IConnection)

type HeartbeatOption struct {
	MakeMsg          HeartbeatMsgFunc // 用户自定义的心跳检测消息处理方法
	OnRemoteNotAlive OnRemoteNotAlive // 用户自定义的远程连接不存活时的处理方法
	HeartbeatMsgID   uint32           // 用户自定义的心跳检测消息ID
	Router           IRouter          // 用户自定义的心跳检测消息业务处理路由
	RouterSlices     []RouterHandler  // 新版本的路由处理函数的集合
}

type HeartbeatChecker struct {
	interval         time.Duration    // 心跳检测时间间隔
	quitChan         chan bool        // 退出信号
	makeMsg          HeartbeatMsgFunc // 用户自定义的心跳检测消息处理方法
	onRemoteNotAlive OnRemoteNotAlive // 用户自定义的远程连接不存活时的处理方法
	msgID            uint32           // 心跳的消息ID
	router           IRouter          // 用户自定义的心跳检测消息业务处理路由
	routerSlices     []RouterHandler  // 用户自定义的心跳检测消息业务处理新路由
	conn             IConnection      // 绑定的链接
	beatFunc         HeartbeatFunc    // 用户自定义心跳发送函数
}

// HeatBeatDefaultRouter 收到remote心跳消息的默认回调路由业务
type HeatBeatDefaultRouter struct {
	BaseRouter
}

func (r *HeatBeatDefaultRouter) Handle(req IRequest) {
	xlog.InfoF("receive heartbeat from %s, MsgID = %+v, Data = %s",
		req.GetConnection().RemoteAddr(), req.GetMsgID(), string(req.GetData()))
}

func HeatBeatDefaultHandle(req IRequest) {
	xlog.InfoF("receive heartbeat from %s, MsgID = %+v, Data = %s",
		req.GetConnection().RemoteAddr(), req.GetMsgID(), string(req.GetData()))
}

func makeDefaultMsg(conn IConnection) []byte {
	msg := fmt.Sprintf("heartbeat [%s->%s]", conn.LocalAddr(), conn.RemoteAddr())
	return []byte(msg)
}

func notAliveDefaultFunc(conn IConnection) {
	xlog.InfoF("remote connection %s is not alive, stop it", conn.RemoteAddr())
	conn.Stop()
}

func NewHeartbeatChecker(interval time.Duration) IHeartbeatChecker {
	heartbeat := &HeartbeatChecker{
		interval: interval,
		quitChan: make(chan bool),

		// 均使用默认的心跳消息生成函数和远程连接不存活时的处理方法
		makeMsg:          makeDefaultMsg,
		onRemoteNotAlive: notAliveDefaultFunc,
		msgID:            HeartbeatDefaultMsgID,
		router:           &HeatBeatDefaultRouter{},
		routerSlices:     []RouterHandler{HeatBeatDefaultHandle},
		beatFunc:         nil,
	}

	return heartbeat
}

func (h *HeartbeatChecker) SetOnRemoteNotAlive(f OnRemoteNotAlive) {
	if f != nil {
		h.onRemoteNotAlive = f
	}
}

func (h *HeartbeatChecker) SetHeartbeatMsgFunc(f HeartbeatMsgFunc) {
	if f != nil {
		h.makeMsg = f
	}
}

func (h *HeartbeatChecker) SetHeartbeatFunc(beatFunc HeartbeatFunc) {
	if beatFunc != nil {
		h.beatFunc = beatFunc
	}
}

func (h *HeartbeatChecker) BindRouter(msgID uint32, router IRouter) {
	if router != nil && msgID != HeartbeatDefaultMsgID {
		h.msgID = msgID
		h.router = router
	}
}

func (h *HeartbeatChecker) BindRouterSlices(msgID uint32, handlers ...RouterHandler) {
	if len(handlers) > 0 && msgID != HeartbeatDefaultMsgID {
		h.msgID = msgID
		h.routerSlices = append(h.routerSlices, handlers...)
	}
}

func (h *HeartbeatChecker) start() {
	ticker := time.NewTicker(h.interval)
	for {
		select {
		case <-ticker.C:
			_ = h.check()
		case <-h.quitChan:
			ticker.Stop()
			return
		}
	}
}

func (h *HeartbeatChecker) Start() {
	go h.start()
}

func (h *HeartbeatChecker) Stop() {
	xlog.InfoF("heartbeat checker stop, connID=%+v", h.conn.GetConnID())
	h.quitChan <- true
}

func (h *HeartbeatChecker) SendHeartbeatMsg() error {

	msg := h.makeMsg(h.conn)

	err := h.conn.SendMsg(h.msgID, msg)
	if err != nil {
		xlog.ErrorF("send heartbeat msg error: %v, msgId=%+v msg=%+v", err, h.msgID, msg)
		return err
	}

	return nil
}

func (h *HeartbeatChecker) check() (err error) {
	if h.conn == nil {
		return nil
	}

	if !h.conn.IsAlive() {
		h.onRemoteNotAlive(h.conn)
	} else {
		if h.beatFunc != nil {
			err = h.beatFunc(h.conn)
		} else {
			err = h.SendHeartbeatMsg()
		}
	}

	return err
}

func (h *HeartbeatChecker) BindConn(conn IConnection) {
	h.conn = conn
	conn.SetHeartbeat(h)
}

// Clone 克隆到一个指定的链接上
func (h *HeartbeatChecker) Clone() IHeartbeatChecker {
	heartbeat := &HeartbeatChecker{
		interval:         h.interval,
		quitChan:         make(chan bool),
		beatFunc:         h.beatFunc,
		makeMsg:          h.makeMsg,
		onRemoteNotAlive: h.onRemoteNotAlive,
		msgID:            h.msgID,
		router:           h.router,
		routerSlices:     h.routerSlices,
		conn:             nil,
	}

	return heartbeat
}

func (h *HeartbeatChecker) MsgID() uint32 {
	return h.msgID
}

func (h *HeartbeatChecker) Router() IRouter {
	return h.router
}

func (h *HeartbeatChecker) RouterSlices() []RouterHandler {
	return h.routerSlices
}
