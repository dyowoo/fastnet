/**
* @File: msg_handler.go
* @Author: Jason Woo
* @Date: 2023/6/30 15:17
**/

package fastnet

import (
	"encoding/hex"
	"fmt"
	"github.com/dyowoo/fastnet/xconf"
	"github.com/dyowoo/fastnet/xlog"
	"sync"
)

type IMsgHandle interface {
	AddRouter(msgID uint32, router IRouter)                                //
	AddRouterSlices(msgId uint32, handler ...RouterHandler) IRouterSlices  //
	Group(start, end uint32, Handlers ...RouterHandler) IGroupRouterSlices //
	Use(Handlers ...RouterHandler) IRouterSlices                           //
	StartWorkerPool()                                                      // Start the worker pool
	SendMsgToTaskQueue(request IRequest)                                   // 将消息交给TaskQueue,由worker进行处理
	Execute(request IRequest)                                              // 执行责任链上的拦截器方法
	AddInterceptor(interceptor IInterceptor)                               // 注册责任链任务入口，每个拦截器处理完后，数据都会传递至下一个拦截器，使得消息可以层层处理层层传递，顺序取决于注册顺序
}

const (
	// WorkerIDWithoutWorkerPool (如果不启动Worker协程池，则会给MsgHandler分配一个虚拟的WorkerID，这个workerID为0, 便于指标统计
	// 启动了Worker协程池后，每个worker的ID为0,1,2,3...)
	WorkerIDWithoutWorkerPool int = 0
)

// MsgHandle 对消息的处理回调模块
type MsgHandle struct {
	routers        map[uint32]IRouter  // 存放每个MsgID 所对应的处理方法的map属性
	workerPoolSize uint32              // 业务工作Worker池的数量
	freeWorkers    map[uint32]struct{} // 空闲worker集合
	freeWorkerMu   sync.Mutex
	TaskQueue      []chan IRequest // Worker负责取任务的消息队列
	builder        *chainBuilder   // 责任链构造器
	routerSlices   *RouterSlices
}

func newMsgHandle() *MsgHandle {
	var freeWorkers map[uint32]struct{}
	if xconf.GlobalObject.WorkerMode == xconf.WorkerModeBind {
		// 为每个链接分配一个worker，避免同一worker处理多个链接时的互相影响
		// 同时可以减小MaxWorkerTaskLen，比如50，因为每个worker的负担减轻了
		xconf.GlobalObject.WorkerPoolSize = uint32(xconf.GlobalObject.MaxConn)
		freeWorkers = make(map[uint32]struct{}, xconf.GlobalObject.WorkerPoolSize)

		for i := uint32(0); i < xconf.GlobalObject.WorkerPoolSize; i++ {
			freeWorkers[i] = struct{}{}
		}
	}

	handle := &MsgHandle{
		routers:        make(map[uint32]IRouter),
		routerSlices:   NewRouterSlices(),
		workerPoolSize: xconf.GlobalObject.WorkerPoolSize,
		TaskQueue:      make([]chan IRequest, xconf.GlobalObject.WorkerPoolSize),
		freeWorkers:    freeWorkers,
		builder:        newChainBuilder(),
	}

	// 此处必须把 msgHandler 添加到责任链中，并且是责任链最后一环，在msgHandler中进行解码后由router做数据分发
	handle.builder.Tail(handle)

	return handle
}

// Use worker ID
// 占用workerID
func useWorker(conn IConnection) uint32 {
	mh, _ := conn.GetMsgHandler().(*MsgHandle)
	if mh == nil {
		xlog.ErrorF("useWorker failed, mh is nil")
		return 0
	}

	if xconf.GlobalObject.WorkerMode == xconf.WorkerModeBind {
		mh.freeWorkerMu.Lock()
		defer mh.freeWorkerMu.Unlock()

		for k := range mh.freeWorkers {
			delete(mh.freeWorkers, k)
			return k
		}
	}

	if mh.workerPoolSize <= 0 {
		return 0
	}

	// 根据ConnID来分配当前的连接应该由哪个worker负责处理
	// 轮询的平均分配法则
	// 得到需要处理此条连接的workerID
	return uint32(conn.GetConnID() % uint64(mh.workerPoolSize))
}

// 释放workerID
func freeWorker(conn IConnection) {
	mh, _ := conn.GetMsgHandler().(*MsgHandle)
	if mh == nil {
		xlog.ErrorF("useWorker failed, mh is nil")
		return
	}

	if xconf.GlobalObject.WorkerMode == xconf.WorkerModeBind {
		mh.freeWorkerMu.Lock()
		defer mh.freeWorkerMu.Unlock()

		mh.freeWorkers[conn.GetWorkerID()] = struct{}{}
	}
}

// Intercept 默认必经的数据处理拦截器
func (mh *MsgHandle) Intercept(chain IChain) IcResp {
	request := chain.Request()

	if request != nil {
		switch request.(type) {
		case IRequest:
			iRequest := request.(IRequest)

			if xconf.GlobalObject.WorkerPoolSize > 0 {
				// 已经启动工作池机制，将消息交给Worker处理
				mh.SendMsgToTaskQueue(iRequest)
			} else {
				// 从绑定好的消息和对应的处理方法中执行对应的Handle方法
				if !xconf.GlobalObject.RouterSlicesMode {
					go mh.doMsgHandler(iRequest, WorkerIDWithoutWorkerPool)
				} else if xconf.GlobalObject.RouterSlicesMode {
					go mh.doMsgHandlerSlices(iRequest, WorkerIDWithoutWorkerPool)
				}

			}
		}
	}

	return chain.Proceed(chain.Request())
}

func (mh *MsgHandle) AddInterceptor(interceptor IInterceptor) {
	if mh.builder != nil {
		mh.builder.AddInterceptor(interceptor)
	}
}

// SendMsgToTaskQueue 将消息交给TaskQueue,由worker进行处理
func (mh *MsgHandle) SendMsgToTaskQueue(request IRequest) {
	workerID := request.GetConnection().GetWorkerID()
	mh.TaskQueue[workerID] <- request
	xlog.DebugF("sendMsgToTaskQueue-->%s", hex.EncodeToString(request.GetData()))
}

// doFuncHandler 执行函数式请求
func (mh *MsgHandle) doFuncHandler(request IFuncRequest, workerID int) {
	defer func() {
		if err := recover(); err != nil {
			xlog.ErrorF("workerID: %d doFuncRequest panic: %v", workerID, err)
		}
	}()

	request.CallFunc()
}

// 立即以非阻塞方式处理消息
func (mh *MsgHandle) doMsgHandler(request IRequest, workerID int) {
	defer func() {
		if err := recover(); err != nil {
			xlog.ErrorF("workerID: %d doMsgHandler panic: %v", workerID, err)
		}
	}()

	msgId := request.GetMsgID()
	handler, ok := mh.routers[msgId]

	if !ok {
		xlog.ErrorF("api msgID = %d is not FOUND!", request.GetMsgID())
		return
	}

	// Request请求绑定Router对应关系
	request.BindRouter(handler)

	request.Call()
}

func (mh *MsgHandle) Execute(request IRequest) {
	// 将消息丢到责任链，通过责任链里拦截器层层处理层层传递
	mh.builder.Execute(request)
}

// AddRouter 为消息添加具体的处理逻辑
func (mh *MsgHandle) AddRouter(msgID uint32, router IRouter) {
	// 判断当前msg绑定的API处理方法是否已经存在
	if _, ok := mh.routers[msgID]; ok {
		msgErr := fmt.Sprintf("repeated api , msgID = %+v\n", msgID)
		panic(msgErr)
	}

	// 添加msg与api的绑定关系
	mh.routers[msgID] = router
	xlog.InfoF("add router msgID = %d", msgID)
}

// AddRouterSlices 切片路由添加
func (mh *MsgHandle) AddRouterSlices(msgId uint32, handler ...RouterHandler) IRouterSlices {
	mh.routerSlices.AddHandler(msgId, handler...)
	return mh.routerSlices
}

// Group 路由分组
func (mh *MsgHandle) Group(start, end uint32, Handlers ...RouterHandler) IGroupRouterSlices {
	return NewGroup(start, end, mh.routerSlices, Handlers...)
}

func (mh *MsgHandle) Use(Handlers ...RouterHandler) IRouterSlices {
	mh.routerSlices.Use(Handlers...)
	return mh.routerSlices
}

func (mh *MsgHandle) doMsgHandlerSlices(request IRequest, workerID int) {
	defer func() {
		if err := recover(); err != nil {
			xlog.ErrorF("workerID: %d doMsgHandler panic: %v", workerID, err)
		}
	}()

	msgId := request.GetMsgID()
	handlers, ok := mh.routerSlices.GetHandlers(msgId)
	if !ok {
		xlog.ErrorF("api msgID = %d is not FOUND!", request.GetMsgID())
		return
	}

	request.BindRouterSlices(handlers)
	request.RouterSlicesNext()
}

// StartOneWorker 启动一个Worker工作流程
func (mh *MsgHandle) StartOneWorker(workerID int, taskQueue chan IRequest) {
	xlog.InfoF("Worker ID = %d is started.", workerID)

	// 不断地等待队列中的消息
	for {
		select {
		// 有消息则取出队列的Request，并执行绑定的业务方法
		case request := <-taskQueue:
			switch req := request.(type) {
			case IFuncRequest:
				// 内部函数调用request
				mh.doFuncHandler(req, workerID)
			case IRequest:
				if !xconf.GlobalObject.RouterSlicesMode {
					mh.doMsgHandler(req, workerID)
				} else if xconf.GlobalObject.RouterSlicesMode {
					mh.doMsgHandlerSlices(req, workerID)
				}
			}
		}
	}
}

// StartWorkerPool starts the worker pool
func (mh *MsgHandle) StartWorkerPool() {
	// 遍历需要启动worker的数量，依此启动
	for i := 0; i < int(mh.workerPoolSize); i++ {
		// 给当前worker对应的任务队列开辟空间
		mh.TaskQueue[i] = make(chan IRequest, xconf.GlobalObject.MaxWorkerTaskLen)

		// 启动当前Worker，阻塞的等待对应的任务队列是否有消息传递进来
		go mh.StartOneWorker(i, mh.TaskQueue[i])
	}
}
