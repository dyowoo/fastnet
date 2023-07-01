/**
* @File: data_pack.go
* @Author: Jason Woo
* @Date: 2023/6/30 15:16
**/

package fastnet

type IDataPack interface {
	GetHeadLen() uint32                // 获取包头长度方法
	Pack(msg IMessage) ([]byte, error) // 封包方法
	Unpack([]byte) (IMessage, error)   // 拆包方法
}

const (
	FastDataPack    string = "fastnet_pack_tlv_big_endian"
	FastDataPackOld string = "fastnet_pack_ltv_little_endian"
)

const (
	FastMessage string = "fastnet_message" // 默认标准报文协议格式
)
