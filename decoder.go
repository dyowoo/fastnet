/**
* @File: decoder.go
* @Author: Jason Woo
* @Date: 2023/6/30 15:16
**/

package fastnet

type IDecoder interface {
	IInterceptor
	GetLengthField() *LengthField
}
