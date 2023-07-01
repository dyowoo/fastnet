/**
* @File: request_func.go
* @Author: Jason Woo
* @Date: 2023/6/30 16:37
**/

package fastnet

type RequestFunc struct {
	BaseRequest
	conn     IConnection
	callFunc func()
}

func (rf *RequestFunc) GetConnection() IConnection {
	return rf.conn
}

func (rf *RequestFunc) CallFunc() {
	if rf.callFunc != nil {
		rf.callFunc()
	}
}

func NewFuncRequest(conn IConnection, callFunc func()) IRequest {
	req := new(RequestFunc)
	req.conn = conn
	req.callFunc = callFunc
	return req
}
