/**
* @File: interceptor.go
* @Author: Jason Woo
* @Date: 2023/6/30 15:16
**/

package fastnet

// IcReq 拦截器输入数据
type IcReq interface{}

// IcResp 拦截器输出数据
type IcResp interface{}

// IInterceptor 拦截器
type IInterceptor interface {
	Intercept(IChain) IcResp // 拦截器的拦截处理方法,由开发者定义
}
