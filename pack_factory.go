/**
* @File: pack_factory.go
* @Author: Jason Woo
* @Date: 2023/6/30 16:27
**/

package fastnet

import (
	"sync"
)

var packOnce sync.Once

type PackFactory struct{}

var factoryInstance *PackFactory

// Factory 生成不同封包解包的方式，单例
func Factory() *PackFactory {
	packOnce.Do(func() {
		factoryInstance = new(PackFactory)
	})

	return factoryInstance
}

// NewPack 创建一个具体的拆包解包对象
func (f *PackFactory) NewPack(kind string) IDataPack {
	var dataPack IDataPack

	switch kind {
	case FastDataPack:
		dataPack = NewDataPack()
	case FastDataPackOld:
		dataPack = NewDataPackLtv()
	default:
		dataPack = NewDataPack()
	}

	return dataPack
}
