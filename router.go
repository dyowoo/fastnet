/**
* @File: router.go
* @Author: Jason Woo
* @Date: 2023/6/30 15:17
**/

package fastnet

import (
	"strconv"
	"sync"
)

/*
IRouter
路由接口， 这里面路由是 使用框架者给该链接自定的 处理业务方法
路由里的IRequest 则包含用该链接的链接信息和该链接的请求数据信息
*/
type IRouter interface {
	PreHandle(request IRequest)  // 在处理conn业务之前的钩子方法
	Handle(request IRequest)     // 处理conn业务的方法
	PostHandle(request IRequest) // 处理conn业务之后的钩子方法
}

/*
RouterHandler
方法切片集合式路路由
不同于旧版 新版本仅保存路由方法集合，具体执行交给每个请求的 IRequest
*/
type RouterHandler func(request IRequest)
type IRouterSlices interface {
	Use(Handlers ...RouterHandler)                                         // 添加全局组件
	AddHandler(msgId uint32, handlers ...RouterHandler)                    // 添加业务处理器集合
	Group(start, end uint32, Handlers ...RouterHandler) IGroupRouterSlices // 路由分组管理，并且会返回一个组管理器
	GetHandlers(MsgId uint32) ([]RouterHandler, bool)                      // 获得当前的所有注册在MsgId的处理器集合
}

type IGroupRouterSlices interface {
	Use(Handlers ...RouterHandler)                      // 添加全局组件
	AddHandler(MsgId uint32, Handlers ...RouterHandler) // 添加业务处理器集合
}

// BaseRouter 实现router时，先嵌入这个基类，然后根据需要对这个基类的方法进行重写
type BaseRouter struct{}

// (这里之所以BaseRouter的方法都为空，
// 是因为有的Router不希望有PreHandle或PostHandle
// 所以Router全部继承BaseRouter的好处是，不需要实现PreHandle和PostHandle也可以实例化)

// PreHandle -
func (br *BaseRouter) PreHandle(req IRequest) {}

// Handle -
func (br *BaseRouter) Handle(req IRequest) {}

// PostHandle -
func (br *BaseRouter) PostHandle(req IRequest) {}

// 新切片集合式路由
// 新版本路由基本逻辑,用户可以传入不等数量的路由路由处理器
// 路由本体会讲这些路由处理器函数全部保存,在请求来的时候找到，并交由IRequest去执行
// 路由可以设置全局的共用组件通过Use方法
// 路由可以分组,通过Group,分组也有自己对应Use方法设置组共有组件

type RouterSlices struct {
	Apis     map[uint32][]RouterHandler
	Handlers []RouterHandler
	sync.RWMutex
}

func NewRouterSlices() *RouterSlices {
	return &RouterSlices{
		Apis:     make(map[uint32][]RouterHandler, 10),
		Handlers: make([]RouterHandler, 0, 6),
	}
}

func (r *RouterSlices) Use(handles ...RouterHandler) {
	r.Handlers = append(r.Handlers, handles...)
}

func (r *RouterSlices) AddHandler(msgId uint32, Handlers ...RouterHandler) {
	if _, ok := r.Apis[msgId]; ok {
		panic("repeated api , msgId = " + strconv.Itoa(int(msgId)))
	}

	finalSize := len(r.Handlers) + len(Handlers)
	mergedHandlers := make([]RouterHandler, finalSize)
	copy(mergedHandlers, r.Handlers)
	copy(mergedHandlers[len(r.Handlers):], Handlers)
	r.Apis[msgId] = append(r.Apis[msgId], mergedHandlers...)
}

func (r *RouterSlices) GetHandlers(MsgId uint32) ([]RouterHandler, bool) {
	r.RLock()
	defer r.RUnlock()

	handlers, ok := r.Apis[MsgId]

	return handlers, ok
}

func (r *RouterSlices) Group(start, end uint32, Handlers ...RouterHandler) IGroupRouterSlices {
	return NewGroup(start, end, r, Handlers...)
}

type GroupRouter struct {
	start    uint32
	end      uint32
	handlers []RouterHandler
	router   IRouterSlices
}

func NewGroup(start, end uint32, router *RouterSlices, Handlers ...RouterHandler) *GroupRouter {
	g := &GroupRouter{
		start:    start,
		end:      end,
		handlers: make([]RouterHandler, 0, len(Handlers)),
		router:   router,
	}

	g.handlers = append(g.handlers, Handlers...)

	return g
}

func (g *GroupRouter) Use(Handlers ...RouterHandler) {
	g.handlers = append(g.handlers, Handlers...)
}

func (g *GroupRouter) AddHandler(MsgId uint32, Handlers ...RouterHandler) {
	if MsgId < g.start || MsgId > g.end {
		panic("add s_router to group err in msgId:" + strconv.Itoa(int(MsgId)))
	}

	finalSize := len(g.handlers) + len(Handlers)
	mergedHandlers := make([]RouterHandler, finalSize)
	copy(mergedHandlers, g.handlers)
	copy(mergedHandlers[len(g.handlers):], Handlers)

	g.router.AddHandler(MsgId, mergedHandlers...)
}
