/**
* @File: logo.go
* @Author: Jason Woo
* @Date: 2023/6/30 16:38
**/

package fastnet

import (
	"fmt"
	"github.com/dyowoo/fastnet/xconf"
)

var fastnetLog = `
   ████                     ██                      ██  
  ░██░                     ░██                     ░██  
 ██████  ██████    ██████ ██████ ███████   █████  ██████
░░░██░  ░░░░░░██  ██░░░░ ░░░██░ ░░██░░░██ ██░░░██░░░██░ 
  ░██    ███████ ░░█████   ░██   ░██  ░██░███████  ░██  
  ░██   ██░░░░██  ░░░░░██  ░██   ░██  ░██░██░░░░   ░██  
  ░██  ░░████████ ██████   ░░██  ███  ░██░░██████  ░░██ 
  ░░    ░░░░░░░░ ░░░░░░     ░░  ░░░   ░░  ░░░░░░    ░░  `

func PrintLogo() {
	fmt.Println(fastnetLog)
	fmt.Printf("\n[FastNet] Version: %s, MaxConn: %d, MaxPacketSize: %d\n",
		xconf.GlobalObject.Version,
		xconf.GlobalObject.MaxConn,
		xconf.GlobalObject.MaxPacketSize)
}
