/**
* @File: main.go
* @Author: Jason Woo
* @Date: 2023/6/30 16:45
**/

package main

import (
	"fmt"
	"github.com/dyowoo/fastnet"
)

func helloHandle(request fastnet.IRequest) {
	fmt.Printf("receive from client: msgID = %d, data = %+v, len = %d\n", request.GetMsgID(), string(request.GetData()), len(request.GetData()))
}

func main() {
	s := fastnet.NewServer()

	s.AddRouterSlices(1, helloHandle)

	s.Serve()
}
