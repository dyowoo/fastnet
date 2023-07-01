/**
* @File: main.go
* @Author: Jason Woo
* @Date: 2023/6/30 16:48
**/

package main

import (
	"fmt"
	"github.com/dyowoo/fastnet"
	"os"
	"os/signal"
	"time"
)

func main() {
	l := 100
	var clientList = make(map[int]fastnet.IClient, l)
	for i := 1; i <= l; i++ {
		go func(i int) {
			client := fastnet.NewClient("127.0.0.1", 29001)

			client.Start()

			clientList[i] = client

			time.Sleep(time.Second * 10)

			go func(i int) {
				for {
					_ = client.Conn().SendMsg(1, []byte(fmt.Sprintf("client: %3d", i)))
					time.Sleep(time.Second * 1)
				}
			}(i)

		}(i)
	}

	// close
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	sig := <-c
	fmt.Println("===exit===", sig)

	for i := 1; i <= l; i++ {
		clientList[i].Stop()
	}

	time.Sleep(time.Second * 2)
}
