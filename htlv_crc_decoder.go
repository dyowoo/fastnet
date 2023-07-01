/**
* @File: htlv_crc_decoder.go
* @Author: Jason Woo
* @Date: 2023/6/30 15:59
**/

package fastnet

import (
	"encoding/hex"
	"github.com/dyowoo/fastnet/xlog"
	"math"
)

const HeaderSize = 5

type HtlvCrcDecoder struct {
	Head    byte   // HeaderCode(头码)
	FunCode byte   // FunctionCode(功能码)
	Length  byte   // DataLength(数据长度)
	Body    []byte // BodyData(数据内容)
	Crc     []byte // CRC校验
	Data    []byte // Original data content(原始数据内容)
}

func NewHTLVCRCDecoder() IDecoder {
	return &HtlvCrcDecoder{}
}

func (hcd *HtlvCrcDecoder) GetLengthField() *LengthField {
	//+------+-------+---------+--------+--------+
	//| 头码  | 功能码 | 数据长度 | 数据内容 | CRC校验 |
	//| 1字节 | 1字节  | 1字节   | N字节   |  2字节  |
	//+------+-------+---------+--------+--------+
	// 头码   功能码 数据长度      Body                         CRC
	// A2      10     0E        0102030405060708091011121314 050B
	// 说明：
	//   1.数据长度len是14(0E),这里的len仅仅指Body长度;
	//
	//   lengthFieldOffset   = 2   (len的索引下标是2，下标从0开始) 长度字段的偏差
	//   lengthFieldLength   = 1   (len是1个byte) 长度字段占的字节数
	//   lengthAdjustment    = 2   (len只表示Body长度，程序只会读取len个字节就结束，但是CRC还有2byte没读呢，所以为2)
	//   initialBytesToStrip = 0   (这个0表示完整的协议内容，如果不想要A2，那么这里就是1) 从解码帧中第一次去除的字节数
	//   maxFrameLength      = 255 + 4(起始码、功能码、CRC) (len是1个byte，所以最大长度是无符号1个byte的最大值)
	return &LengthField{
		MaxFrameLength:      math.MaxInt8 + 4,
		LengthFieldOffset:   2,
		LengthFieldLength:   1,
		LengthAdjustment:    2,
		InitialBytesToStrip: 0,
	}
}

func (hcd *HtlvCrcDecoder) decode(data []byte) *HtlvCrcDecoder {
	dataSize := len(data)

	htlvData := HtlvCrcDecoder{
		Data: data,
	}

	htlvData.Head = data[0]
	htlvData.FunCode = data[1]
	htlvData.Length = data[2]
	htlvData.Body = data[3 : dataSize-2]
	htlvData.Crc = data[dataSize-2 : dataSize]

	if !CheckCRC(data[:dataSize-2], htlvData.Crc) {
		xlog.DebugF("crc check error %s %s\n", hex.EncodeToString(data), hex.EncodeToString(htlvData.Crc))
		return nil
	}

	return &htlvData
}

func (hcd *HtlvCrcDecoder) Intercept(chain IChain) IcResp {
	message := chain.GetIMessage()
	if message == nil {
		return chain.ProceedWithIMessage(message, nil)
	}

	data := message.GetData()

	// 读取的数据不超过包头，直接进入下一层
	if len(data) < HeaderSize {
		return chain.ProceedWithIMessage(message, nil)
	}

	htlvData := hcd.decode(data)

	// 将解码后的数据重新设置到IMessage中, Router需要MsgID来寻址
	message.SetMsgID(uint32(htlvData.FunCode))

	// 将解码后的数据进入下一层
	return chain.ProceedWithIMessage(message, *htlvData)
}
