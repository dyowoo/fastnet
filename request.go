/**
* @File: request.go
* @Author: Jason Woo
* @Date: 2023/6/30 15:17
**/

package fastnet

import (
	"github.com/dyowoo/fastnet/xconf"
	"sync"
)

type HandleStep int

// IFuncRequest 函数消息接口
type IFuncRequest interface {
	CallFunc()
}

// IRequest 实际上是把客户端请求的链接信息 和 请求的数据 包装到了 Request里
type IRequest interface {
	GetConnection() IConnection       // 获取请求连接信息
	GetData() []byte                  // 获取请求消息的数据
	GetMsgID() uint32                 // 获取请求的消息ID
	GetMessage() IMessage             // 获取请求消息的原始数据
	GetResponse() IcResp              // 获取解析完后序列化数据
	SetResponse(IcResp)               // 设置解析完后序列化数据
	BindRouter(router IRouter)        // 绑定这次请求由哪个路由处理
	Call()                            // 转进到下一个处理器开始执行 但是调用此方法的函数会根据先后顺序逆序执行
	Abort()                           // 终止处理函数的运行 但调用此方法的函数会执行完毕
	Goto(HandleStep)                  // 指定接下来的Handle去执行哪个Handler函数(慎用，会导致循环调用)
	BindRouterSlices([]RouterHandler) // 新路由操作
	RouterSlicesNext()                // 执行下一个函数
}

type BaseRequest struct{}

func (br *BaseRequest) GetConnection() IConnection       { return nil }
func (br *BaseRequest) GetData() []byte                  { return nil }
func (br *BaseRequest) GetMsgID() uint32                 { return 0 }
func (br *BaseRequest) GetMessage() IMessage             { return nil }
func (br *BaseRequest) GetResponse() IcResp              { return nil }
func (br *BaseRequest) SetResponse(IcResp)               {}
func (br *BaseRequest) BindRouter(IRouter)               {}
func (br *BaseRequest) Call()                            {}
func (br *BaseRequest) Abort()                           {}
func (br *BaseRequest) Goto(HandleStep)                  {}
func (br *BaseRequest) BindRouterSlices([]RouterHandler) {}
func (br *BaseRequest) RouterSlicesNext()                {}

const (
	PreHandle  HandleStep = iota // PreHandle for pre-processing
	Handle                       // Handle for processing
	PostHandle                   // PostHandle for post-processing
	HandleOver
)

// Request 请求
type Request struct {
	BaseRequest
	conn     IConnection     // 已经和客户端建立好的链接
	msg      IMessage        // 客户端请求的数据
	router   IRouter         // 请求处理的函数
	steps    HandleStep      // 用来控制路由函数执行
	stepLock *sync.RWMutex   // 并发互斥
	needNext bool            // 是否需要执行下一个路由函数
	icResp   IcResp          // 拦截器返回数据
	handlers []RouterHandler // 路由函数切片
	index    int8            // 路由函数切片索引
}

func (r *Request) GetResponse() IcResp {
	return r.icResp
}

func (r *Request) SetResponse(response IcResp) {
	r.icResp = response
}

func NewRequest(conn IConnection, msg IMessage) IRequest {
	req := new(Request)
	req.steps = PreHandle
	req.conn = conn
	req.msg = msg
	req.stepLock = new(sync.RWMutex)
	req.needNext = true
	req.index = -1

	return req
}

func (r *Request) GetMessage() IMessage {
	return r.msg
}

func (r *Request) GetConnection() IConnection {
	return r.conn
}

func (r *Request) GetData() []byte {
	return r.msg.GetData()
}

func (r *Request) GetMsgID() uint32 {
	return r.msg.GetMsgID()
}

func (r *Request) BindRouter(router IRouter) {
	r.router = router
}

func (r *Request) next() {
	if r.needNext == false {
		r.needNext = true
		return
	}

	r.stepLock.Lock()
	r.steps++
	r.stepLock.Unlock()
}

func (r *Request) Goto(step HandleStep) {
	r.stepLock.Lock()
	r.steps = step
	r.needNext = false
	r.stepLock.Unlock()
}

func (r *Request) Call() {

	if r.router == nil {
		return
	}

	for r.steps < HandleOver {
		switch r.steps {
		case PreHandle:
			r.router.PreHandle(r)
		case Handle:
			r.router.Handle(r)
		case PostHandle:
			r.router.PostHandle(r)
		}

		r.next()
	}

	r.steps = PreHandle
}

func (r *Request) Abort() {
	if xconf.GlobalObject.RouterSlicesMode {
		r.index = int8(len(r.handlers))
	} else {
		r.stepLock.Lock()
		r.steps = HandleOver
		r.stepLock.Unlock()
	}
}

func (r *Request) BindRouterSlices(handlers []RouterHandler) {
	r.handlers = handlers
}

func (r *Request) RouterSlicesNext() {
	r.index++
	for r.index < int8(len(r.handlers)) {
		r.handlers[r.index](r)
		r.index++
	}
}
