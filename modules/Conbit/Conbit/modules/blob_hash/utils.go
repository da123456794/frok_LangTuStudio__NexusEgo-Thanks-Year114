package blob_hash

import (
	"fmt"

	blob_hash_packet "github.com/LangTuStudio/Conbit/Conbit/modules/blob_hash/packet"
	"github.com/LangTuStudio/Conbit/nodes/defines"
	"github.com/LangTuStudio/Conbit/utils/packet_marshal"
)

// CallAPI ..
func CallAPI[
	T1 packet_marshal.Packet,
	T2 packet_marshal.Packet,
](request T1, f func() T2, node defines.Node) (result T2, err error) {
	originResp, err := node.CallWithResponse(
		request.Name(),
		defines.FromBytes(packet_marshal.Encode(request)),
	).
		SetTimeout(BlockingDeadline).
		BlockGetResult()

	if err != nil {
		err = fmt.Errorf("CallAPI: %v", err)
		return
	}

	rawResp, err := originResp.ToBytes()
	if err != nil {
		err = fmt.Errorf("CallAPI: %v", err)
		return
	}

	resultPacket, err := packet_marshal.Decode(rawResp, func() packet_marshal.Packet { return f() })
	if err != nil {
		return f(), fmt.Errorf("CallAPI: %v", err)
	}
	result = resultPacket.(T2)

	return
}

// gc 通过重设底层的映射以初步回收垃圾。
// 调用时将使用互斥锁确保底层映射不会被并发读写。
// 应当确保 gc 只被服务者所调用
func (b *BaseBlobHashHolder) gc() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.serverSpecial.pendingRequest) == 0 {
		b.serverSpecial.pendingRequest = make(map[blob_hash_packet.Hash][]blob_hash_packet.HashWithPosition)
	}
	if len(b.serverSpecial.pendingRequestBlocking) == 0 {
		b.serverSpecial.pendingRequestBlocking = make(map[blob_hash_packet.Hash]map[blob_hash_packet.HashWithPosition]chan struct{})
	}
}

// addTopendingRequest 向 b.serverSpecial.pendingRequest[hash] 追加 pos。
// 应当确保 addTopendingRequest 只被服务者所调用
func (b *BaseBlobHashHolder) addTopendingRequest(
	hash uint64, pos blob_hash_packet.HashWithPosition,
) {
	h := blob_hash_packet.Hash(hash)

	if b.serverSpecial.pendingRequest == nil {
		b.serverSpecial.pendingRequest = make(map[blob_hash_packet.Hash][]blob_hash_packet.HashWithPosition)
	}
	if b.serverSpecial.pendingRequest[h] == nil {
		b.serverSpecial.pendingRequest[h] = make([]blob_hash_packet.HashWithPosition, 0)
	}
	s := b.serverSpecial.pendingRequest[h]
	s = append(s, pos)
	b.serverSpecial.pendingRequest[h] = s

	if b.serverSpecial.pendingRequestBlocking == nil {
		b.serverSpecial.pendingRequestBlocking = make(map[blob_hash_packet.Hash]map[blob_hash_packet.HashWithPosition]chan struct{})
	}
	if b.serverSpecial.pendingRequestBlocking[h] == nil {
		b.serverSpecial.pendingRequestBlocking[h] = make(map[blob_hash_packet.HashWithPosition]chan struct{})
	}
	b.serverSpecial.pendingRequestBlocking[h][pos] = make(chan struct{})
}

// addToPendingRequestBlocking 在底层创建一个管道，
// 用于底层的其他实现阻塞并等待服务器对 hashWithPosition 的响应
// 应当确保 addToPendingRequestBlocking 只被服务者所调用
func (b *BaseBlobHashHolder) addToPendingRequestBlocking(hashWithPosition blob_hash_packet.HashWithPosition) {
	h := hashWithPosition.Hash
	if b.serverSpecial.pendingRequestBlocking == nil {
		b.serverSpecial.pendingRequestBlocking = make(map[blob_hash_packet.Hash]map[blob_hash_packet.HashWithPosition]chan struct{})
	}
	if b.serverSpecial.pendingRequestBlocking[h] == nil {
		b.serverSpecial.pendingRequestBlocking[h] = make(map[blob_hash_packet.HashWithPosition]chan struct{})
	}
	b.serverSpecial.pendingRequestBlocking[h][hashWithPosition] = make(chan struct{})
}
