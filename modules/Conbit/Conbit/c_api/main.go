package main

/*
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
*/
import "C"
import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
	"unsafe"

	"github.com/LangTuStudio/Conbit/Conbit"
	"github.com/LangTuStudio/Conbit/Conbit/blocks"
	"github.com/LangTuStudio/Conbit/Conbit/bundle"
	"github.com/LangTuStudio/Conbit/Conbit/chunks/chunk"
	"github.com/LangTuStudio/Conbit/Conbit/chunks/define"
	"github.com/LangTuStudio/Conbit/Conbit/modules/area_request"
	"github.com/LangTuStudio/Conbit/Conbit/modules/blob_hash"
	"github.com/LangTuStudio/Conbit/Conbit/rental_server_impact/access_helper"
	"github.com/LangTuStudio/Conbit/Conbit/rental_server_impact/info_collect_utils"
	"github.com/LangTuStudio/Conbit/Conbit/supported_nbt_data"
	"github.com/LangTuStudio/Conbit/i18n"
	"github.com/LangTuStudio/Conbit/nodes"
	"github.com/LangTuStudio/Conbit/nodes/defines"
	"github.com/LangTuStudio/Conbit/nodes/underlay_conn"
	"github.com/vmihailenco/msgpack/v5"

	"github.com/LangTuStudio/Conbit/minecraft/nbt"
	"github.com/LangTuStudio/Conbit/minecraft/protocol"
	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
)

var GOmegaCore Conbit.MicroOmega
var GNode defines.Node
var GResultWaiter map[string]func(ret string, err error)
var GPacketNameIDMapping map[string]uint32
var GPacketIDNameMapping map[uint32]string
var GPool packet.Pool
var omegaMutex sync.RWMutex

//export OmegaAvailable
func OmegaAvailable() bool {
	// 添加安全检查，防止在GOmegaCore未初始化时调用
	omegaMutex.RLock()
	core := GOmegaCore
	omegaMutex.RUnlock()

	if core == nil {
		return false
	}
	return true
}

//export OmegaAPIVersion
func OmegaAPIVersion() int32 {
	// 返回API版本号
	omegaMutex.RLock()
	core := GOmegaCore
	omegaMutex.RUnlock()

	if core == nil {
		return int32(0) // 如果没有初始化，返回0
	}
	return int32(115) // 最新API版本
}

const (
	EventTypeOmegaConnErr           = "OmegaConnErr"
	EventTypeCommandResponseCB      = "CommandResponseCB"
	EventTypeNewPacket              = "MCPacket"
	EventTypePlayerInterceptedInput = "PlayerInterceptInput"
	EventTypePlayerChange           = "PlayerChange"
	EventTypeChat                   = "Chat"
	EventTypeNamedCommandBlockMsg   = "NamedCommandBlockMsg"
	EventTypeSoftCallResp           = "SoftCallResp"
	EventTypeSoftListen             = "SoftListen"
	EventTypeSoftAPICall            = "SoftAPICall"
)

type GEvent struct {
	EventType string
	// 调用语言可能希望将某个回调绑定到事件回调上，由于我们无法将事件传入 go 内部，
	// 因此此字段用以帮助调用语言找到目标回调
	RetrieverID string
	// Golang 有自己的GC，当一个数据从 GO 的视野中消失的时候，得假设它可能已经被回收了
	// 然而，实际上event的数据可以是任何类型的
	// 我们要求外部语言在通过 epoll 知道一个 event 发生（知道 event 的 emitter 和 retrieve id 后）
	// 立刻调用 omit event 忽略这个事件（这样 go 也可以顺利回收资源）
	// 或者 consume xxx 将这个事件立刻转为特定的类型
	Data any
}

var GEventsChan chan *GEvent
var GCurrentEvent *GEvent

//export EventPoll
func EventPoll() (EventType *C.char, RetrieverID *C.char) {
	if GEventsChan == nil {
		return C.CString(EventTypeOmegaConnErr), C.CString("")
	}
	e := <-GEventsChan
	if e == nil {
		return C.CString(""), C.CString("")
	}
	GCurrentEvent = e
	return C.CString(e.EventType), C.CString(e.RetrieverID)
}

//export OmitEvent
func OmitEvent() {
	GCurrentEvent = nil
}

// Async Actions

//export ConsumeCommandResponseCB
func ConsumeCommandResponseCB() *C.char {
	if GCurrentEvent == nil {
		return C.CString("")
	}
	p, ok := GCurrentEvent.Data.(*packet.CommandOutput)
	if !ok {
		return C.CString("")
	}
	bs, _ := json.Marshal(p)
	return C.CString(string(bs))
}

//export ConsumeSoftData
func ConsumeSoftData() *C.char {
	if GCurrentEvent == nil {
		return C.CString("")
	}
	p, ok := GCurrentEvent.Data.(string)
	if !ok {
		return C.CString("")
	}
	return C.CString(p)
}

//export ConsumeSoftCall
func ConsumeSoftCall(cbID *C.char) *C.char {
	if GCurrentEvent == nil {
		return C.CString("")
	}
	p, ok := GCurrentEvent.Data.(*ArgWithCb)
	if !ok {
		return C.CString("")
	}
	if cbID == nil {
		return C.CString("")
	}
	if GResultWaiter == nil {
		GResultWaiter = map[string]func(ret string, err error){}
	}
	GResultWaiter[C.GoString(cbID)] = func(ret string, err error) {
		p.cb(defines.FromString(ret), err)
	}
	return C.CString(p.args)
}

//export FinishSoftCall
func FinishSoftCall(cbID *C.char, jsonStr *C.char, errStr *C.char) {
	if cbID == nil || jsonStr == nil || errStr == nil {
		return
	}
	gCbID := C.GoString(cbID)
	gJsonStr := C.GoString(jsonStr)
	gErrStr := C.GoString(errStr)
	h := GResultWaiter[gCbID]
	delete(GResultWaiter, gCbID)
	if gErrStr == "" {
		h(gJsonStr, nil)
	} else {
		h(gJsonStr, errors.New(gErrStr))
	}
}

//export SendWebSocketCommandNeedResponse
func SendWebSocketCommandNeedResponse(cmd *C.char, retrieverID *C.char) {
	if GOmegaCore == nil || GEventsChan == nil || cmd == nil || retrieverID == nil {
		return
	}
	GoRetrieverID := C.GoString(retrieverID)
	GOmegaCore.GetGameControl().SendWebSocketCmdNeedResponse(C.GoString(cmd)).AsyncGetResult(func(p *packet.CommandOutput, err error) {
		if err != nil {
			p = nil
		}
		GEventsChan <- &GEvent{EventTypeCommandResponseCB, GoRetrieverID, p}
	})
}

//export SendPlayerCommandNeedResponse
func SendPlayerCommandNeedResponse(cmd *C.char, retrieverID *C.char) {
	if GOmegaCore == nil || GEventsChan == nil || cmd == nil || retrieverID == nil {
		return
	}
	GoRetrieverID := C.GoString(retrieverID)
	GOmegaCore.GetGameControl().SendPlayerCmdNeedResponse(C.GoString(cmd)).AsyncGetResult(func(p *packet.CommandOutput, err error) {
		if err != nil {
			p = nil
		}
		GEventsChan <- &GEvent{EventTypeCommandResponseCB, GoRetrieverID, p}
	})
}

//export SoftCall
func SoftCall(api *C.char, jsonStr *C.char, retrieverID *C.char) {
	// 首先检查依赖是否可用
	if GNode == nil || GEventsChan == nil {
		return
	}

	// 在函数开头就进行快速参数验证
	if api == nil || jsonStr == nil || retrieverID == nil {
		return
	}

	// 在调用 C.GoString 前保存参数值，避免重复检查
	apiValue := ""
	jsonStrValue := ""
	retrieverIDValue := ""

	// 一次性安全地获取所有字符串值（如果可能的话）
	// 但注意，如果指针无效，这仍会导致panic，这是CGO的限制
	// 然而，在大多数正常用例中，这种检查应该是有效的
	apiValue = C.GoString(api)
	jsonStrValue = C.GoString(jsonStr)
	retrieverIDValue = C.GoString(retrieverID)

	// 使用获取的值进行实际操作
	GNode.CallWithResponse(apiValue, defines.FromString(jsonStrValue)).AsyncGetResult(func(ret defines.Values, err error) {
		var msg string
		msg, _ = ret.ToString()
		GEventsChan <- &GEvent{EventTypeSoftCallResp, retrieverIDValue, msg}
	})
}

//export SoftListen
func SoftListen(api *C.char) {
	if GNode == nil || GEventsChan == nil || api == nil {
		return
	}
	goAPI := C.GoString(api)
	GNode.ListenMessage(goAPI, func(data defines.Values) {
		msg, _ := data.ToString()
		GEventsChan <- &GEvent{EventTypeSoftListen, goAPI, msg}
	}, false)
}

type ArgWithCb struct {
	args string
	cb   func(defines.Values, error)
}

//export SoftReg
func SoftReg(api *C.char) {
	if GNode == nil || api == nil {
		return
	}
	gAPI := C.GoString(api)
	GNode.ExposeAPI(gAPI).CallBackAPI(func(in defines.Values, setResult func(defines.Values, error)) {
		args, _ := in.ToString()
		GEventsChan <- &GEvent{EventTypeSoftAPICall, gAPI, &ArgWithCb{
			args: args,
			cb:   setResult,
		}}
	})
}

//export SoftPub
func SoftPub(api *C.char, jsonStr *C.char) {
	if GNode == nil || api == nil || jsonStr == nil {
		return
	}
	GNode.PublishMessage(C.GoString(api), defines.FromString(C.GoString(jsonStr)))
}

//export SendWOCommand
func SendWOCommand(cmd *C.char) {
	if GOmegaCore == nil || cmd == nil {
		return
	}
	GOmegaCore.GetGameControl().SendWOCmd(C.GoString(cmd))
}

//export SendWebSocketCommandOmitResponse
func SendWebSocketCommandOmitResponse(cmd *C.char) {
	if GOmegaCore == nil || cmd == nil {
		return
	}
	GOmegaCore.GetGameControl().SendWebSocketCmdOmitResponse(C.GoString(cmd))
}

//export SendPlayerCommandOmitResponse
func SendPlayerCommandOmitResponse(cmd *C.char) {
	if GOmegaCore == nil || cmd == nil {
		return
	}
	GOmegaCore.GetGameControl().SendPlayerCmdOmitResponse(C.GoString(cmd))
}

//export SendGamePacket
func SendGamePacket(packetID int, jsonStr *C.char) (err *C.char) {
	if jsonStr == nil {
		return C.CString("jsonStr is nil")
	}
	pk := GPool[uint32(packetID)]()
	_err := json.Unmarshal([]byte(C.GoString(jsonStr)), &pk)
	if _err != nil {
		return C.CString(_err.Error())
	}
	GOmegaCore.GetGameControl().SendPacket(pk)
	return C.CString("")
}

type NoEOFByteReader struct {
	s []byte
	i int
}

func (nbr *NoEOFByteReader) Read(b []byte) (n int, err error) {
	if len(b) == 0 {
		return 0, nil
	}
	if nbr.i >= len(nbr.s) {
		return 0, io.EOF
	}
	n = copy(b, nbr.s[nbr.i:])
	nbr.i += n
	return
}

func (nbr *NoEOFByteReader) ReadByte() (b byte, err error) {
	if nbr.i >= len(nbr.s) {
		return 0, io.EOF
	}
	b = nbr.s[nbr.i]
	nbr.i++
	return b, nil
}

func bytesToCharArr(goByteSlice []byte) *C.char {
	if len(goByteSlice) == 0 {
		return nil
	}
	// 验证数组索引安全性
	if len(goByteSlice) > 0 {
		ptr := C.malloc(C.size_t(len(goByteSlice)))
		if ptr == nil {
			return nil
		}
		C.memmove(ptr, (unsafe.Pointer)(&goByteSlice[0]), C.size_t(len(goByteSlice)))
		return (*C.char)(ptr)
	}
	return nil
}

//export JsonStrAsIsGamePacketBytes
func JsonStrAsIsGamePacketBytes(packetID C.int32_t, jsonStr *C.char) (pktBytes *C.char, l C.int32_t, err *C.char) {
	if jsonStr == nil {
		return nil, 0, C.CString("jsonStr is nil")
	}
	pk := GPool[uint32(packetID)]()
	if pk == nil {
		return nil, 0, C.CString("invalid packet ID")
	}
	jsonStrGo := C.GoString(jsonStr)
	if jsonStrGo == "" {
		return nil, 0, C.CString("jsonStr is empty")
	}
	_err := json.Unmarshal([]byte(jsonStrGo), &pk)
	if _err != nil {
		return nil, 0, C.CString(_err.Error())
	}
	b := &bytes.Buffer{}
	w := protocol.NewWriter(b, 0)
	// hdr := pk.ID()
	// w.Varuint32(&hdr)
	pk.Marshal(w)
	bs := b.Bytes()
	l = C.int32_t(len(bs))
	return bytesToCharArr(bs), l, nil
}

type SimplePos struct {
	X, Y, Z int
}

// 	if !done {
// 		fmt.Printf("place command block @ [%v,%v,%v] fail\n", opt.X, opt.Y, opt.Z)
// 	} else {
// 		fmt.Printf("place command block @ [%v,%v,%v] ok\n", opt.X, opt.Y, opt.Z)
// 	}
// }, time.Second*10)

// listeners

// disconnect event

//export ConsumeOmegaConnError
func ConsumeOmegaConnError() *C.char {
	if GCurrentEvent == nil {
		return C.CString("no current event")
	}
	err, ok := GCurrentEvent.Data.(error)
	if !ok {
		return C.CString("invalid event data type")
	}
	return C.CString(err.Error())
}

// packet event

var GAllPacketsListenerEnabled = false

//export ListenAllPackets
func ListenAllPackets() {
	if GAllPacketsListenerEnabled {
		panic("should only call ListenAllPackets once")
	}
	GAllPacketsListenerEnabled = true
	GOmegaCore.GetGameListener().SetAnyPacketCallBack(func(p packet.Packet) {
		GEventsChan <- &GEvent{
			EventType:   EventTypeNewPacket,
			RetrieverID: GPacketIDNameMapping[p.ID()],
			Data:        p,
		}
	}, true)
}

//export ResetListenPacketsStatus
func ResetListenPacketsStatus() {
	// 请勿在连接未结束时调用, 可能产生意想不到的后果
	GAllPacketsListenerEnabled = false
}

//export GetPacketNameIDMapping
func GetPacketNameIDMapping() *C.char {
	marshal, err := json.Marshal(GPacketNameIDMapping)
	if err != nil {
		panic(err)
	}
	return C.CString(string(marshal))
}

//export ConsumeMCPacket
func ConsumeMCPacket() (packetDataAsJsonStr *C.char, convertError *C.char) {
	p := (GCurrentEvent.Data).(packet.Packet)
	marshal, err := json.Marshal(p)
	packetDataAsJsonStr = C.CString(string(marshal))
	convertError = nil
	if err != nil {
		convertError = C.CString(string(err.Error()))
	}
	return
}

// 添加缺失的API接口

//export LoadBlobCache
func LoadBlobCache(hash uint64) (pktBytes *C.char, length C.int) {
	// 增加基本nil检查
	if GOmegaCore == nil {
		return nil, 0
	}

	blobHashInterface := GOmegaCore.GetBlobHashHolder()
	if blobHashInterface == nil {
		return nil, 0
	}

	blobHashHolder, ok := blobHashInterface.(*blob_hash.BlobHashHolder)
	if !ok || blobHashHolder == nil {
		return nil, 0
	}

	payload := blobHashHolder.LoadBlobCache(hash)
	if payload == nil || len(payload) == 0 {
		return nil, 0
	}

	// 验证数组索引安全性
	if len(payload) > 0 {
		length = C.int(len(payload))
		ptr := C.malloc(C.size_t(length))
		if ptr == nil {
			return nil, 0
		}
		C.memmove(ptr, unsafe.Pointer(&payload[0]), C.size_t(length))
		return (*C.char)(ptr), length
	}

	return nil, 0
}

//export UpdateBlobCache
func UpdateBlobCache(hash uint64, payload *C.char, length C.int) C.uint8_t {
	if GOmegaCore == nil || payload == nil || length <= 0 {
		return 0
	}

	// 获取BlobHashHolder
	blobHashInterface := GOmegaCore.GetBlobHashHolder()
	if blobHashInterface == nil {
		return 0
	}

	// 类型断言转换为正确的类型
	blobHashHolder, ok := blobHashInterface.(*blob_hash.BlobHashHolder)
	if !ok || blobHashHolder == nil {
		return 0
	}

	goBytes := C.GoBytes(unsafe.Pointer(payload), length)
	success := blobHashHolder.UpdateBlobCache(hash, goBytes)
	if success {
		return 1
	}
	return 0
}

// 数据包处理接口
// 这些接口是基于现有GPool和数据包系统实现的

//export SendMsgpackGamePacket
func SendMsgpackGamePacket(packetID C.int32_t, msgpackData *C.char, length C.int32_t) *C.char {
	if GOmegaCore == nil || msgpackData == nil {
		return C.CString("GOmegaCore not initialized")
	}

	if length <= 0 {
		return C.CString("invalid data length")
	}

	goBytes := C.GoBytes(unsafe.Pointer(msgpackData), C.int(length))
	pk := GPool[uint32(packetID)]()
	if pk == nil {
		return C.CString("invalid packet ID")
	}

	// 使用 defer-recover 来处理 Marshal 过程中可能出现的 panic
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				// 捕获panic，但在这里我们不能中断函数执行，而是需要安全处理
			}
		}()
		reader := bytes.NewReader(goBytes)
		r := protocol.NewReader(reader, 0, false)
		pk.Marshal(r) // 从字节数据恢复到数据包结构
	}()

	GOmegaCore.GetGameControl().SendPacket(pk)
	return C.CString("")
}

//export SendCustomGamePacket
func SendCustomGamePacket(packetID C.int32_t, data *C.char, length C.int32_t) *C.char {
	if GOmegaCore == nil || data == nil {
		return C.CString("GOmegaCore not initialized")
	}

	if length <= 0 {
		return C.CString("invalid data length")
	}

	goBytes := C.GoBytes(unsafe.Pointer(data), C.int(length))
	pk := GPool[uint32(packetID)]()
	if pk == nil {
		return C.CString("invalid packet ID")
	}

	// 使用 defer-recover 来处理 Marshal 过程中可能出现的 panic
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				// 捕获panic，但在这里我们不能中断函数执行，而是需要安全处理
			}
		}()
		reader := bytes.NewReader(goBytes)
		r := protocol.NewReader(reader, 0, false)
		pk.Marshal(r) // 从字节数据恢复到数据包结构
	}()

	GOmegaCore.GetGameControl().SendPacket(pk)
	return C.CString("")
}

//export SendBytesGamePacket
func SendBytesGamePacket(packetID C.int32_t, payload *C.char, length C.int32_t) *C.char {
	if GOmegaCore == nil || payload == nil {
		return C.CString("GOmegaCore not initialized")
	}

	if length <= 0 {
		return C.CString("invalid payload length")
	}

	goBytes := C.GoBytes(unsafe.Pointer(payload), C.int(length))
	pk := GPool[uint32(packetID)]()
	if pk == nil {
		return C.CString("invalid packet ID")
	}

	// 使用 defer-recover 来处理 Marshal 过程中可能出现的 panic
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				// 捕获panic，但在这里我们不能中断函数执行，而是需要安全处理
			}
		}()
		reader := bytes.NewReader(goBytes)
		r := protocol.NewReader(reader, 0, false)
		pk.Marshal(r) // 从字节数据恢复到数据包结构
	}()

	GOmegaCore.GetGameControl().SendPacket(pk)
	return C.CString("")
}

//export ConsumeMCBytesPacket
func ConsumeMCBytesPacket() (pktBytes *C.char, length C.int32_t) {
	if GCurrentEvent == nil {
		return nil, 0
	}

	p, ok := GCurrentEvent.Data.(packet.Packet)
	if !ok {
		return nil, 0
	}

	buf := &bytes.Buffer{}
	w := protocol.NewWriter(buf, 0)
	p.Marshal(w)

	data := buf.Bytes()
	if len(data) == 0 {
		return nil, 0
	}

	// 验证数组索引安全性
	if len(data) > 0 {
		length = C.int32_t(len(data))
		ptr := C.malloc(C.size_t(length))
		if ptr == nil {
			return nil, 0
		}
		C.memmove(ptr, unsafe.Pointer(&data[0]), C.size_t(length))
		return (*C.char)(ptr), length
	}

	return nil, 0
}

//export ConsumeMCPacketToMsgpack
func ConsumeMCPacketToMsgpack() (packetDataAsMsgpack *C.char, bs_len C.int32_t, convertError *C.char) {
	if GCurrentEvent == nil {
		return nil, 0, C.CString("no current event")
	}

	p, ok := GCurrentEvent.Data.(packet.Packet)
	if !ok {
		return nil, 0, C.CString("event data is not a packet")
	}

	bs, err := msgpack.Marshal(p)
	if err != nil {
		return nil, 0, C.CString(err.Error())
	}

	// 验证数组索引安全性
	if len(bs) > 0 {
		bs_len = C.int32_t(len(bs))
		ptr := C.malloc(C.size_t(bs_len))
		if ptr == nil {
			return nil, 0, C.CString("failed to allocate memory")
		}
		C.memmove(ptr, unsafe.Pointer(&bs[0]), C.size_t(bs_len))
		return (*C.char)(ptr), bs_len, nil
	}

	return nil, 0, nil
}

//export SendJsonGamePacket
func SendJsonGamePacket(packetID C.int32_t, jsonStr *C.char, length C.int32_t) *C.char {
	if GOmegaCore == nil || jsonStr == nil {
		return C.CString("GOmegaCore not initialized")
	}

	if length <= 0 {
		return C.CString("invalid json string length")
	}

	goBytes := C.GoBytes(unsafe.Pointer(jsonStr), C.int(length))
	pk := GPool[uint32(packetID)]()
	if pk == nil {
		return C.CString("invalid packet ID")
	}

	errUnmarshal := json.Unmarshal(goBytes, pk)
	if errUnmarshal != nil {
		return C.CString(errUnmarshal.Error())
	}

	GOmegaCore.GetGameControl().SendPacket(pk)
	return C.CString("")
}

//export SetPacketFilterMode
func SetPacketFilterMode(pkID int32, enabled byte) *C.char {
	// 这个功能在当前实现中可能需要额外的API支持
	// 由于MicroOmega接口没有提供SetPacketFilter功能，我们暂时返回错误
	return C.CString("SetPacketFilterMode not implemented in current version")
}

//export TranslateChunkNBT
func TranslateChunkNBT(chunkNBT *C.char, length C.int32_t) (result *C.char, result_len C.int32_t) {
	if length <= 0 || chunkNBT == nil {
		return nil, 0
	}

	goBytes := C.GoBytes(unsafe.Pointer(chunkNBT), C.int(length))

	// 由于ToolDelta插件需要将NBT格式的区块数据转换为二进制块数据
	// 我们使用area_request包中的SubChunkDecode函数来实现此功能
	// 这个函数可以解析NBT格式的子区块数据并将其转换为内部结构

	subChunkIndex, subChunk, nbts, err := area_request.SubChunkDecode(goBytes)
	if err != nil {
		// 如果解析失败，返回空结果
		return nil, 0
	}

	// 构建输出缓冲区，重新编码转换后的数据
	var outputBuffer bytes.Buffer

	// 写入子区块版本号（版本9是常用的）
	outputBuffer.WriteByte(9)

	// 计算并写入存储层数量
	storageCount := byte(0)
	if subChunk != nil {
		layers := subChunk.Layers()
		storageCount = byte(len(layers))
		if storageCount > 255 { // 限制存储层数量
			storageCount = 255
		}
	} else {
		// 如果没有子区块，写入1个空层
		storageCount = 1
	}
	outputBuffer.WriteByte(storageCount)

	// 写入子区块索引
	outputBuffer.WriteByte(byte(subChunkIndex))

	// 写入每个存储层
	if subChunk != nil {
		layers := subChunk.Layers()
		for i := 0; i < int(storageCount) && i < len(layers); i++ {
			layer := layers[i]
			if layer != nil {
				encodePalettedStorage(&outputBuffer, layer)
			} else {
				// 写入空存储层
				outputBuffer.WriteByte(0) // blockSize为0表示空存储
			}
		}
	} else {
		// 写入空存储层
		outputBuffer.WriteByte(0)
	}

	// 写入NBT块实体数据（如果存在）
	for _, nbtData := range nbts {
		if nbtData != nil {
			encoder := nbt.NewEncoderWithEncoding(&outputBuffer, nbt.NetworkLittleEndian)
			if encodeErr := encoder.Encode(nbtData); encodeErr != nil {
				// 如果编码块实体失败，继续处理其他数据
				continue
			}
		}
	}

	// 获取转换后的数据
	convertedData := outputBuffer.Bytes()
	result_len = C.int32_t(len(convertedData))

	if result_len > 0 {
		ptr := C.malloc(C.size_t(result_len))
		if ptr == nil {
			return nil, 0
		}
		C.memmove(ptr, unsafe.Pointer(&convertedData[0]), C.size_t(result_len))
		return (*C.char)(ptr), result_len
	}

	return nil, 0
}

// encodePalettedStorage 将调色板存储编码到输出缓冲区
func encodePalettedStorage(buf *bytes.Buffer, storage *chunk.PalettedStorage) {
	// 获取存储块大小 - 使用BitsPerIndex方法获取位数
	blockSize := storage.BitsPerIndex()

	// 写入blockSize（左移1位）
	buf.WriteByte(byte(blockSize) << 1)

	// 如果blockSize为0，表示空存储，不需要额外数据
	if blockSize == 0 {
		return
	}

	// 写入uint32数据数组
	indices := storage.Indices()
	for _, val := range indices {
		buf.WriteByte(byte(val))
		buf.WriteByte(byte(val >> 8))
		buf.WriteByte(byte(val >> 16))
		buf.WriteByte(byte(val >> 24))
	}

	// 写入调色板数据
	palette := storage.Palette()
	if palette != nil {
		paletteCount := int32(len(palette.Values))
		protocol.WriteVarint32(buf, paletteCount)

		for _, val := range palette.Values {
			protocol.WriteVarint32(buf, int32(val))
		}
	}
}

// Bot
//

//export GetClientMaintainedBotBasicInfo
func GetClientMaintainedBotBasicInfo() *C.char {
	basicInfo := GOmegaCore.GetMicroUQHolder().GetBotBasicInfo()
	basicInfoMap := map[string]any{
		"BotName":      basicInfo.GetBotName(),
		"BotRuntimeID": basicInfo.GetBotRuntimeID(),
		"BotUniqueID":  basicInfo.GetBotUniqueID(),
		"BotIdentity":  basicInfo.GetBotIdentity(),
		"BotUUIDStr":   basicInfo.GetBotUUIDStr(),
		"BotUID":       basicInfo.GetBotUID(),
	}
	data, _ := json.Marshal(basicInfoMap)
	return C.CString(string(data))
}

//export GetClientMaintainedExtendInfo
func GetClientMaintainedExtendInfo() *C.char {
	extendInfo := GOmegaCore.GetMicroUQHolder().GetExtendInfo()
	extendInfoMap := map[string]any{}
	if worldName, found := extendInfo.GetWorldName(); found {
		extendInfoMap["WorldName"] = worldName
	}
	if worldSeed, found := extendInfo.GetWorldSeed(); found {
		extendInfoMap["WorldSeed"] = worldSeed
	}
	if worldGenerator, found := extendInfo.GetWorldGenerator(); found {
		extendInfoMap["WorldGenerator"] = worldGenerator
	}
	if levelID, found := extendInfo.GetLevelID(); found {
		extendInfoMap["LevelID"] = levelID
	}
	if thres, found := extendInfo.GetCompressThreshold(); found {
		extendInfoMap["CompressThreshold"] = thres
	}
	if worldGameMode, found := extendInfo.GetWorldGameMode(); found {
		extendInfoMap["WorldGameMode"] = worldGameMode
	}
	if worldDifficulty, found := extendInfo.GetWorldDifficulty(); found {
		extendInfoMap["WorldDifficulty"] = worldDifficulty
	}
	if time, found := extendInfo.GetTime(); found {
		extendInfoMap["Time"] = time
	}
	if dayTime, found := extendInfo.GetDayTime(); found {
		extendInfoMap["DayTime"] = dayTime
	}
	if timePercent, found := extendInfo.GetDayTimePercent(); found {
		extendInfoMap["TimePercent"] = timePercent
	}
	if gameRules, found := extendInfo.GetGameRules(); found {
		extendInfoMap["GameRules"] = gameRules
	}
	data, _ := json.Marshal(extendInfoMap)
	return C.CString(string(data))
}

// Player 描述单个的 Conbit.PlayerKit
type Player struct {
	// 描述该结构体实际所携带的 PlayerKit 负载
	GPlayer Conbit.PlayerKit
	// 描述该 Player 的引用计数
	UsingCount int
}

// 描述多个玩家的 PlayerKit 。
//
// 每当一个 Player 每新增一个使用者(如 Python 等)，
// 对应 Player 的引用计数都会加一。
//
// 当相应的使用者尝试释放一个 Player 时，
// UsingCount 将减一，
// 直到归零后，真正地被回收。
//
// 如果必要，可以考虑使用 ForceReleaseBindPlayer
// 进行强制回收，但这是危险的。
//
// 相关的引用计数不在 Go 处控制，
// 它们由使用者根据实际情况增加或减少，
// Go 处仅在引用计数归零后释放数据
type Players map[string]Player

// players
var GPlayers struct {
	Players
	sync.RWMutex
}

//export AddGPlayerUsingCount
func AddGPlayerUsingCount(uuid *C.char, delta int) {
	GPlayers.Lock()
	defer GPlayers.Unlock()

	uuidStr := C.GoString(uuid)
	player, found := GPlayers.Players[uuidStr]
	if !found {
		playerKit, found := GOmegaCore.GetPlayerInteract().GetPlayerKitByUUIDString(uuidStr)
		if !found {
			return
		}
		player = Player{GPlayer: playerKit}
	}

	player.UsingCount = player.UsingCount + delta
	GPlayers.Players[uuidStr] = player

	if player.UsingCount <= 0 {
		new := make(map[string]Player)
		delete(GPlayers.Players, uuidStr)
		for key, value := range GPlayers.Players {
			new[key] = value
		}
		GPlayers.Players = new
	}
}

//export ForceReleaseBindPlayer
func ForceReleaseBindPlayer(uuidStr *C.char) {
	GPlayers.Lock()
	defer GPlayers.Unlock()

	new := make(map[string]Player)
	delete(GPlayers.Players, C.GoString(uuidStr))
	for key, value := range GPlayers.Players {
		new[key] = value
	}
	GPlayers.Players = new
}

//export GetAllOnlinePlayers
func GetAllOnlinePlayers() *C.char {
	GPlayers.Lock()
	defer GPlayers.Unlock()

	players := GOmegaCore.GetPlayerInteract().ListAllPlayers()
	retPlayers := []string{}
	for _, player := range players {
		uuidStr, _ := player.GetUUIDString()
		retPlayers = append(retPlayers, uuidStr)
	}
	data, _ := json.Marshal(retPlayers)
	return C.CString(string(data))
}

//export GetPlayerByName
func GetPlayerByName(name *C.char) *C.char {
	GPlayers.Lock()
	defer GPlayers.Unlock()

	player, found := GOmegaCore.GetPlayerInteract().GetPlayerKit(C.GoString(name))
	if found {
		uuidStr, _ := player.GetUUIDString()
		return C.CString(uuidStr)
	}
	return C.CString("")
}

//export GetPlayerByUUID
func GetPlayerByUUID(uuid *C.char) *C.char {
	GPlayers.Lock()
	defer GPlayers.Unlock()

	player, found := GOmegaCore.GetPlayerInteract().GetPlayerKitByUUIDString(C.GoString(uuid))
	if found {
		uuidStr, _ := player.GetUUIDString()
		return C.CString(uuidStr)
	}
	return C.CString("")
}

//export PlayerName
func PlayerName(uuidStr *C.char) *C.char {
	GPlayers.RLock()
	defer GPlayers.RUnlock()

	p := GPlayers.Players[C.GoString(uuidStr)]
	name, _ := p.GPlayer.GetUsername()
	return C.CString(name)
}

//export PlayerEntityUniqueID
func PlayerEntityUniqueID(uuidStr *C.char) int64 {
	GPlayers.RLock()
	defer GPlayers.RUnlock()

	p := GPlayers.Players[C.GoString(uuidStr)]
	entityUniqueID, _ := p.GPlayer.GetEntityUniqueID()
	return entityUniqueID
}

//export PlayerLoginTime
func PlayerLoginTime(uuidStr *C.char) int64 {
	GPlayers.RLock()
	defer GPlayers.RUnlock()

	p := GPlayers.Players[C.GoString(uuidStr)]
	loginTime, _ := p.GPlayer.GetLoginTime()
	return loginTime.Unix()
}

//export PlayerPlatformChatID
func PlayerPlatformChatID(uuidStr *C.char) *C.char {
	GPlayers.RLock()
	defer GPlayers.RUnlock()

	p := GPlayers.Players[C.GoString(uuidStr)]
	name, _ := p.GPlayer.GetPlatformChatID()
	return C.CString(name)
}

//export PlayerBuildPlatform
func PlayerBuildPlatform(uuidStr *C.char) int32 {
	GPlayers.RLock()
	defer GPlayers.RUnlock()

	p := GPlayers.Players[C.GoString(uuidStr)]
	buildPlatform, _ := p.GPlayer.GetBuildPlatform()
	return buildPlatform
}

//export PlayerSkinID
func PlayerSkinID(uuidStr *C.char) *C.char {
	GPlayers.RLock()
	defer GPlayers.RUnlock()

	p := GPlayers.Players[C.GoString(uuidStr)]
	SkinID, _ := p.GPlayer.GetSkinID()
	return C.CString(SkinID)
}

// //export PlayerPropertiesFlag
// func PlayerPropertiesFlag(uuidStr *C.char) uint32 {
// 	GPlayers.RLock()
// 	defer GPlayers.RUnlock()
//
// 	p := GPlayers.Players[C.GoString(uuidStr)]
// 	PropertiesFlag, _ := p.GPlayer.GetPropertiesFlag()
// 	return PropertiesFlag
// }

// //export PlayerCommandPermissionLevel
// func PlayerCommandPermissionLevel(uuidStr *C.char) uint32 {
// 	GPlayers.RLock()
// 	defer GPlayers.RUnlock()
//
// 	p := GPlayers.Players[C.GoString(uuidStr)]
// 	CommandPermissionLevel, _ := p.GPlayer.GetCommandPermissionLevel()
// 	return CommandPermissionLevel
// }

// //export PlayerActionPermissions
// func PlayerActionPermissions(uuidStr *C.char) uint32 {
// 	GPlayers.RLock()
// 	defer GPlayers.RUnlock()
//
// 	p := GPlayers.Players[C.GoString(uuidStr)]
// 	ActionPermissions, _ := p.GPlayer.GetActionPermissions()
// 	return ActionPermissions
// }

// //export PlayerGetAbilityString
// func PlayerGetAbilityString(uuidStr *C.char) *C.char {
// 	GPlayers.RLock()
// 	defer GPlayers.RUnlock()
//
// 	p := GPlayers.Players[C.GoString(uuidStr)]
// 	adventureFlagsMap, actionPermissionMap, _ := p.GPlayer.GetAbilityString()
// 	abilityMap := map[string]map[string]bool{
// 		"AdventureFlagsMap":   adventureFlagsMap,
// 		"ActionPermissionMap": actionPermissionMap,
// 	}
// 	data, _ := json.Marshal(abilityMap)
// 	return C.CString(string(data))
// }

// //export PlayerOPPermissionLevel
// func PlayerOPPermissionLevel(uuidStr *C.char) uint32 {
// 	GPlayers.RLock()
// 	defer GPlayers.RUnlock()
//
// 	p := GPlayers.Players[C.GoString(uuidStr)]
// 	OPPermissionLevel, _ := p.GPlayer.GetOPPermissionLevel()
// 	return OPPermissionLevel
// }

// //export PlayerCustomStoredPermissions
// func PlayerCustomStoredPermissions(uuidStr *C.char) uint32 {
// 	GPlayers.RLock()
// 	defer GPlayers.RUnlock()
//
// 	p := GPlayers.Players[C.GoString(uuidStr)]
// 	CustomStoredPermissions, _ := p.GPlayer.GetCustomStoredPermissions()
// 	return CustomStoredPermissions
// }

//export PlayerCanBuild
func PlayerCanBuild(uuidStr *C.char) bool {
	GPlayers.RLock()
	defer GPlayers.RUnlock()

	p := GPlayers.Players[C.GoString(uuidStr)]
	hasAbility, _ := p.GPlayer.CanBuild()
	return hasAbility
}

//export PlayerSetBuild
func PlayerSetBuild(uuidStr *C.char, allow bool) {
	GPlayers.RLock()
	defer GPlayers.RUnlock()

	p := GPlayers.Players[C.GoString(uuidStr)]
	p.GPlayer.SetBuildAbility(allow)
}

//export PlayerCanMine
func PlayerCanMine(uuidStr *C.char) bool {
	GPlayers.RLock()
	defer GPlayers.RUnlock()

	p := GPlayers.Players[C.GoString(uuidStr)]
	hasAbility, _ := p.GPlayer.CanMine()
	return hasAbility
}

//export PlayerSetMine
func PlayerSetMine(uuidStr *C.char, allow bool) {
	GPlayers.RLock()
	defer GPlayers.RUnlock()

	p := GPlayers.Players[C.GoString(uuidStr)]
	p.GPlayer.SetMineAbility(allow)
}

//export PlayerCanDoorsAndSwitches
func PlayerCanDoorsAndSwitches(uuidStr *C.char) bool {
	GPlayers.RLock()
	defer GPlayers.RUnlock()

	p := GPlayers.Players[C.GoString(uuidStr)]
	hasAbility, _ := p.GPlayer.CanDoorsAndSwitches()
	return hasAbility
}

//export PlayerSetDoorsAndSwitches
func PlayerSetDoorsAndSwitches(uuidStr *C.char, allow bool) {
	GPlayers.RLock()
	defer GPlayers.RUnlock()

	p := GPlayers.Players[C.GoString(uuidStr)]
	p.GPlayer.SetDoorsAndSwitchesAbility(allow)
}

//export PlayerCanOpenContainers
func PlayerCanOpenContainers(uuidStr *C.char) bool {
	GPlayers.RLock()
	defer GPlayers.RUnlock()

	p := GPlayers.Players[C.GoString(uuidStr)]
	hasAbility, _ := p.GPlayer.CanOpenContainers()
	return hasAbility
}

//export PlayerSetOpenContainers
func PlayerSetOpenContainers(uuidStr *C.char, allow bool) {
	GPlayers.RLock()
	defer GPlayers.RUnlock()

	p := GPlayers.Players[C.GoString(uuidStr)]
	p.GPlayer.SetOpenContainersAbility(allow)
}

//export PlayerCanAttackPlayers
func PlayerCanAttackPlayers(uuidStr *C.char) bool {
	GPlayers.RLock()
	defer GPlayers.RUnlock()

	p := GPlayers.Players[C.GoString(uuidStr)]
	hasAbility, _ := p.GPlayer.CanAttackPlayers()
	return hasAbility
}

//export PlayerSetAttackPlayers
func PlayerSetAttackPlayers(uuidStr *C.char, allow bool) {
	GPlayers.RLock()
	defer GPlayers.RUnlock()

	p := GPlayers.Players[C.GoString(uuidStr)]
	p.GPlayer.SetAttackPlayersAbility(allow)
}

//export PlayerCanAttackMobs
func PlayerCanAttackMobs(uuidStr *C.char) bool {
	GPlayers.RLock()
	defer GPlayers.RUnlock()

	p := GPlayers.Players[C.GoString(uuidStr)]
	hasAbility, _ := p.GPlayer.CanAttackMobs()
	return hasAbility
}

//export PlayerSetAttackMobs
func PlayerSetAttackMobs(uuidStr *C.char, allow bool) {
	GPlayers.RLock()
	defer GPlayers.RUnlock()

	p := GPlayers.Players[C.GoString(uuidStr)]
	p.GPlayer.SetAttackMobsAbility(allow)
}

//export PlayerCanOperatorCommands
func PlayerCanOperatorCommands(uuidStr *C.char) bool {
	GPlayers.RLock()
	defer GPlayers.RUnlock()

	p := GPlayers.Players[C.GoString(uuidStr)]
	hasAbility, _ := p.GPlayer.CanOperatorCommands()
	return hasAbility
}

//export PlayerSetOperatorCommands
func PlayerSetOperatorCommands(uuidStr *C.char, allow bool) {
	GPlayers.RLock()
	defer GPlayers.RUnlock()

	p := GPlayers.Players[C.GoString(uuidStr)]
	p.GPlayer.SetOperatorCommandsAbility(allow)
}

//export PlayerCanTeleport
func PlayerCanTeleport(uuidStr *C.char) bool {
	GPlayers.RLock()
	defer GPlayers.RUnlock()

	p := GPlayers.Players[C.GoString(uuidStr)]
	hasAbility, _ := p.GPlayer.CanTeleport()
	return hasAbility
}

//export PlayerSetTeleport
func PlayerSetTeleport(uuidStr *C.char, allow bool) {
	GPlayers.RLock()
	defer GPlayers.RUnlock()

	p := GPlayers.Players[C.GoString(uuidStr)]
	p.GPlayer.SetTeleportAbility(allow)
}

//export PlayerStatusInvulnerable
func PlayerStatusInvulnerable(uuidStr *C.char) bool {
	GPlayers.RLock()
	defer GPlayers.RUnlock()

	p := GPlayers.Players[C.GoString(uuidStr)]
	hasAbility, _ := p.GPlayer.StatusInvulnerable()
	return hasAbility
}

//export PlayerStatusFlying
func PlayerStatusFlying(uuidStr *C.char) bool {
	GPlayers.RLock()
	defer GPlayers.RUnlock()

	p := GPlayers.Players[C.GoString(uuidStr)]
	hasAbility, _ := p.GPlayer.StatusFlying()
	return hasAbility
}

//export PlayerStatusMayFly
func PlayerStatusMayFly(uuidStr *C.char) bool {
	GPlayers.RLock()
	defer GPlayers.RUnlock()

	p := GPlayers.Players[C.GoString(uuidStr)]
	hasAbility, _ := p.GPlayer.StatusMayFly()
	return hasAbility
}

//export PlayerDeviceID
func PlayerDeviceID(uuidStr *C.char) *C.char {
	GPlayers.RLock()
	defer GPlayers.RUnlock()

	p := GPlayers.Players[C.GoString(uuidStr)]
	name, _ := p.GPlayer.GetDeviceID()
	return C.CString(name)
}

//export PlayerEntityRuntimeID
func PlayerEntityRuntimeID(uuidStr *C.char) uint64 {
	GPlayers.RLock()
	defer GPlayers.RUnlock()

	p := GPlayers.Players[C.GoString(uuidStr)]
	EntityRuntimeID, _ := p.GPlayer.GetEntityRuntimeID()
	return EntityRuntimeID
}

//export PlayerEntityMetadata
func PlayerEntityMetadata(uuidStr *C.char) *C.char {
	GPlayers.RLock()
	defer GPlayers.RUnlock()

	p := GPlayers.Players[C.GoString(uuidStr)]
	entityMetadata, _ := p.GPlayer.GetEntityMetadata()
	data, _ := json.Marshal(entityMetadata)
	return C.CString(string(data))
}

//export PlayerIsOP
func PlayerIsOP(uuidStr *C.char) bool {
	GPlayers.RLock()
	defer GPlayers.RUnlock()

	p := GPlayers.Players[C.GoString(uuidStr)]
	isOP, _ := p.GPlayer.IsOP()
	return isOP
}

//export PlayerOnline
func PlayerOnline(uuidStr *C.char) bool {
	GPlayers.RLock()
	defer GPlayers.RUnlock()

	p := GPlayers.Players[C.GoString(uuidStr)]
	return p.GPlayer.StillOnline()
}

//export PlayerChat
func PlayerChat(uuidStr *C.char, msg *C.char) {
	GPlayers.RLock()
	defer GPlayers.RUnlock()

	p := GPlayers.Players[C.GoString(uuidStr)]
	p.GPlayer.Say(C.GoString(msg))
}

//export PlayerTitle
func PlayerTitle(uuidStr *C.char, title, subTitle *C.char) {
	GPlayers.RLock()
	defer GPlayers.RUnlock()

	p := GPlayers.Players[C.GoString(uuidStr)]
	p.GPlayer.SubTitle(C.GoString(subTitle), C.GoString(title))
}

//export PlayerActionBar
func PlayerActionBar(uuidStr *C.char, actionBar *C.char) {
	GPlayers.RLock()
	defer GPlayers.RUnlock()

	p := GPlayers.Players[C.GoString(uuidStr)]
	p.GPlayer.ActionBar(C.GoString(actionBar))
}

// //export SetPlayerAbility
// func SetPlayerAbility(uuidStr *C.char, jsonFlags *C.char) {
// 	GPlayers.RLock()
// 	defer GPlayers.RUnlock()
//
// 	p := GPlayers.Players[C.GoString(uuidStr)]
// 	// abilityMap := map[string]map[string]bool{
// 	// 	"AdventureFlagsMap":   adventureFlagsMap,
// 	// 	"ActionPermissionMap": actionPermissionMap,
// 	// }
// 	abilityMap := map[string]map[string]bool{}
// 	json.Unmarshal([]byte(C.GoString(jsonFlags)), &abilityMap)
// 	adventureFlagsMap := abilityMap["AdventureFlagsMap"]
// 	actionPermissionMap := abilityMap["ActionPermissionMap"]
// 	fmt.Println(adventureFlagsMap)
// 	fmt.Println(actionPermissionMap)
// 	p.GPlayer.SetAbilityString(adventureFlagsMap, actionPermissionMap)
// }

//export InterceptPlayerJustNextInput
func InterceptPlayerJustNextInput(uuidStr *C.char, retrieverID *C.char) {
	GPlayers.RLock()
	defer GPlayers.RUnlock()

	p := GPlayers.Players[C.GoString(uuidStr)]
	retrieverIDStr := C.GoString(retrieverID)
	p.GPlayer.GetInput(false).AsyncGetResult(func(chat *Conbit.GameChat, err error) {
		GEventsChan <- &GEvent{
			EventType:   EventTypePlayerInterceptedInput,
			RetrieverID: retrieverIDStr,
			Data:        chat,
		}
	})
}

// BotActions exporter
// 也许这块应该放到更合适的地方..

//export UseHotbarItem
func UseHotbarItem(slotID C.uint8_t) *C.char {
	if GOmegaCore == nil {
		return C.CString("GOmegaCore not initialized")
	}

	errGo := GOmegaCore.GetBotAction().UseHotBarItem(uint8(slotID))
	if errGo != nil {
		return C.CString(errGo.Error())
	}

	return C.CString("")
}

//export UseHotbarItemOnBlock
func UseHotbarItemOnBlock(x C.int32_t, y C.int32_t, z C.int32_t, blockNEMCRuntimeID C.uint32_t, slotID C.uint8_t, face C.int32_t) *C.char {
	if GOmegaCore == nil {
		return C.CString("GOmegaCore not initialized")
	}

	pos := define.CubePos{int(x), int(y), int(z)}
	errGo := GOmegaCore.GetBotAction().UseHotBarItemOnBlock(pos, uint32(blockNEMCRuntimeID), int32(face), uint8(slotID))
	if errGo != nil {
		return C.CString(errGo.Error())
	}

	return C.CString("")
}

//export DropItemFromHotBar
func DropItemFromHotBar(slotID C.uint8_t) {
	if GOmegaCore != nil {
		GOmegaCore.GetBotAction().DropItemFromHotBar(uint8(slotID))
	}
}

//export MoveItemInsideHotBarOrInventory
func MoveItemInsideHotBarOrInventory(sourceSlot C.uint8_t, targetSlot C.uint8_t, count C.uint8_t) *C.char {
	if GOmegaCore == nil {
		return C.CString("GOmegaCore not initialized")
	}

	errGo := GOmegaCore.GetBotAction().MoveItemInsideHotBarOrInventory(uint8(sourceSlot), uint8(targetSlot), uint8(count))
	if errGo != nil {
		return C.CString(errGo.Error())
	}

	return C.CString("")
}

//export GetInventoryContent
func GetInventoryContent(windowID C.uint32_t, slotID C.uint8_t) *C.char {
	if GOmegaCore != nil {
		instance, found := GOmegaCore.GetBotAction().GetInventoryContent(uint32(windowID), uint8(slotID))
		if found {
			itemData := map[string]interface{}{
				"found": true,
				"item":  instance,
			}
			data, _ := json.Marshal(itemData)
			return C.CString(string(data))
		} else {
			itemData := map[string]interface{}{
				"found": false,
			}
			data, _ := json.Marshal(itemData)
			return C.CString(string(data))
		}
	}
	return C.CString("")
}

//export SelectHotBar
func SelectHotBar(slotID C.uint8_t) *C.char {
	if GOmegaCore == nil {
		return C.CString("GOmegaCore not initialized")
	}

	errGo := GOmegaCore.GetBotAction().SelectHotBar(uint8(slotID))
	if errGo != nil {
		return C.CString(errGo.Error())
	}

	return C.CString("")
}

//export SleepTick
func SleepTick(ticks C.int32_t) {
	if GOmegaCore != nil {
		GOmegaCore.GetBotAction().SleepTick(int(ticks))
	}
}

//export PlaceCommandBlock
func PlaceCommandBlock(optionJson *C.char) *C.char {
	if GOmegaCore == nil {
		return C.CString("GOmegaCore not initialized")
	}

	option := supported_nbt_data.CommandBlockSupportedData{}
	errUnmarshal := json.Unmarshal([]byte(C.GoString(optionJson)), &option)
	if errUnmarshal != nil {
		return C.CString(errUnmarshal.Error())
	}

	// 从JSON中提取位置信息
	type PositionData struct {
		X int `json:"X"`
		Y int `json:"Y"`
		Z int `json:"Z"`
	}

	posData := PositionData{}
	errPos := json.Unmarshal([]byte(C.GoString(optionJson)), &posData)
	if errPos != nil {
		return C.CString(errPos.Error())
	}

	errGo := GOmegaCore.GetBotAction().HighLevelPlaceCommandBlock(define.CubePos{posData.X, posData.Y, posData.Z}, &option, 3)
	if errGo != nil {
		return C.CString(errGo.Error())
	} else {
		return nil
	}
}

//export RenameItemWithAnvil
func RenameItemWithAnvil(anvilX C.int32_t, anvilY C.int32_t, anvilZ C.int32_t, blockNEMCRID C.uint32_t, slotID C.uint8_t, newName *C.char) *C.char {
	if GOmegaCore == nil {
		return C.CString("GOmegaCore not initialized")
	}

	anvilPos := define.CubePos{int(anvilX), int(anvilY), int(anvilZ)}
	name := C.GoString(newName)

	errGo := GOmegaCore.GetBotAction().UseAnvil(anvilPos, uint32(blockNEMCRID), uint8(slotID), name)
	if errGo != nil {
		return C.CString(errGo.Error())
	}

	// 丢弃重命名后的物品
	GOmegaCore.GetBotAction().DropItemFromHotBar(uint8(slotID))

	return C.CString("")
}

//export GetBlockRuntimeID
func GetBlockRuntimeID(blockID *C.char) C.int32_t {
	result, find := blocks.BlockStrToRuntimeID(C.GoString(blockID))
	if find {
		return C.int32_t(result)
	} else {
		return C.int32_t(-1)
	}
}

//export SetBlock
func SetBlock(x C.int32_t, y C.int32_t, z C.int32_t, blockName *C.char) *C.char {
	if GOmegaCore == nil {
		return C.CString("GOmegaCore not initialized")
	}

	blockStr := C.GoString(blockName)

	cmd := GOmegaCore.GetBotAction().ConstructGeneralCommand("setblock " + fmt.Sprintf("%d %d %d %s", x, y, z, blockStr))
	cmd.Send()

	return C.CString("")
}

//export FillBlocks
func FillBlocks(startX C.int32_t, startY C.int32_t, startZ C.int32_t, endX C.int32_t, endY C.int32_t, endZ C.int32_t, blockName *C.char) *C.char {
	if GOmegaCore == nil {
		return C.CString("GOmegaCore not initialized")
	}

	cmd := GOmegaCore.GetBotAction().ConstructGeneralCommand("fill " + fmt.Sprintf("%d %d %d %d %d %d %s", startX, startY, startZ, endX, endY, endZ, C.GoString(blockName)))
	cmd.Send()

	return C.CString("")
}

//export ConsumeChat
func ConsumeChat() *C.char {
	chat := GCurrentEvent.Data.(*Conbit.GameChat)
	bs, _ := json.Marshal(chat)
	return C.CString(string(bs))
}

var GListenPlayerChangeListened = false

//export ListenPlayerChange
func ListenPlayerChange() {
	GPlayers.Lock()
	defer GPlayers.Unlock()

	if GListenPlayerChangeListened {
		panic("ListenPlayerChange should only called once")
	}
	GListenPlayerChangeListened = true
	GOmegaCore.GetPlayerInteract().ListenPlayerChange(func(player Conbit.PlayerKit, action string) {
		uuidStr, _ := player.GetUUIDString()
		GEventsChan <- &GEvent{
			EventType:   EventTypePlayerChange,
			RetrieverID: uuidStr,
			Data:        action,
		}
	})
}

//export ResetListenPlayerChangeStatus
func ResetListenPlayerChangeStatus() {
	// 请勿在连接未结束时调用, 可能产生意想不到的后果
	GListenPlayerChangeListened = false
}

//export ConsumePlayerChange
func ConsumePlayerChange() (change *C.char) {
	return C.CString(GCurrentEvent.Data.(string))
}

var GListenChatListened = false

//export ListenChat
func ListenChat() {
	if GListenChatListened {
		panic("ListenPlayerChat should only called once")
	}
	GListenChatListened = true
	GOmegaCore.GetPlayerInteract().SetOnChatCallBack(func(chat *Conbit.GameChat) {
		GEventsChan <- &GEvent{
			EventType:   EventTypeChat,
			RetrieverID: "",
			Data:        chat,
		}
	})
}

//export ListenCommandBlock
func ListenCommandBlock(name *C.char) {
	gName := C.GoString(name)
	GOmegaCore.GetPlayerInteract().SetOnSpecificCommandBlockTellCallBack(gName, func(chat *Conbit.GameChat) {
		GEventsChan <- &GEvent{
			EventType:   EventTypeNamedCommandBlockMsg,
			RetrieverID: gName,
			Data:        chat,
		}
	})
}

// utils

//export FreeMem
func FreeMem(address unsafe.Pointer) {
	C.free(address)
}

func prepareOmegaAPIs(omegaCore Conbit.MicroOmega) {
	GEventsChan = make(chan *GEvent, 1024)
	omegaMutex.Lock() // 写锁
	GOmegaCore = omegaCore
	core := GOmegaCore
	omegaMutex.Unlock() // 释放写锁

	// 用局部变量获取映射关系，避免在锁外访问可能被其他线程修改的GOmegaCore
	GPacketNameIDMapping = core.GetGameListener().GetMCPacketNameIDMapping()
	{
		GPacketIDNameMapping = map[uint32]string{}
		for name, id := range GPacketNameIDMapping {
			GPacketIDNameMapping[id] = name
		}
	}
	GPool = packet.NewPool()
	go func() {
		err := <-omegaCore.WaitClosed()
		omegaMutex.Lock() // 写锁
		GOmegaCore = nil
		omegaMutex.Unlock() // 释放写锁
		GEventsChan <- &GEvent{
			EventTypeOmegaConnErr,
			"",
			err,
		}
	}()
	GPlayers.Players = make(Players)
}

//export ConnectOmega
func ConnectOmega(address *C.char) (Cerr *C.char) {
	if GOmegaCore != nil {
		return C.CString("connect has been established")
	}
	var node defines.Node
	// ctx := context.Background()
	{
		client, err := underlay_conn.NewClientFromBasicNet(C.GoString(address), time.Second)
		if err != nil {
			return C.CString(err.Error())
		}
		slave, err := nodes.NewSlaveNode(client)
		if err != nil {
			return C.CString(err.Error())
		}
		node = nodes.NewGroup("Conbit", slave, false)
		if !node.CheckNetTag("access-point") {
			return C.CString(i18n.T(i18n.S_no_access_point_in_network))
		}
	}
	GNode = node
	omegaCore, err := bundle.NewEndPointMicroOmega(node)
	if err != nil {
		return C.CString(err.Error())
	}
	prepareOmegaAPIs(omegaCore)
	return nil
}

//export ResetGOmega
func ResetGOmega() {
	omegaMutex.Lock()
	GOmegaCore = nil
	omegaMutex.Unlock()
}

//export StartOmega
func StartOmega(address *C.char, impactOptionsJson *C.char) (Cerr *C.char) {
	if GOmegaCore != nil {
		return C.CString("connect has been established")
	}
	var node defines.Node
	accessOption := access_helper.DefaultOptions()
	// ctx := context.Background()
	{
		impactOption := &access_helper.ImpactOption{}
		json.Unmarshal([]byte(C.GoString(impactOptionsJson)), &impactOption)
		if err := info_collect_utils.ReadUserInfoAndUpdateImpactOptions(impactOption); err != nil {
			return C.CString(err.Error())
		}

		accessOption.ImpactOption = impactOption
		accessOption.MakeBotCreative = true
		accessOption.DisableCommandBlock = false
		accessOption.ReasonWithPrivilegeStuff = true

		{
			server, err := underlay_conn.NewServerFromBasicNet(C.GoString(address))
			if err != nil {
				panic(err)
			}
			// server := nodes.NewSimpleNewMasterNodeServer(socket)
			master := nodes.NewMasterNode(server)
			node = nodes.NewGroup("Conbit", master, false)
		}
	}
	GNode = node
	ctx := context.Background()
	omegaCore, err := access_helper.ImpactServer(ctx, node, accessOption)
	if err != nil {
		return C.CString(err.Error())
	}
	prepareOmegaAPIs(omegaCore)
	return nil
}

func main() {
	//Windows: go build  -tags fbconn -o fbconn.dll -buildmode=c-shared main.go
	//Linux: go build -tags fbconn -o libfbconn.so -buildmode=c-shared main.go
	//Macos: go build -o omega_conn.dylib -buildmode=c-shared main.go
	//将生成的文件 (fbconn.dll 或 libfbconn.so 或 fbconn.dylib) 放在 conn.py 同一个目录下
}
