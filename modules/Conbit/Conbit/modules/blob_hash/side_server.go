package blob_hash

import (
	"fmt"
	"time"

	blob_hash_packet "github.com/LangTuStudio/Conbit/Conbit/modules/blob_hash/packet"
	"github.com/LangTuStudio/Conbit/nodes/defines"
	"github.com/LangTuStudio/Conbit/utils/packet_marshal"
	"github.com/google/uuid"
	"github.com/pterm/pterm"
)

// BlobHashServerSide ..
type BlobHashServerSide struct {
	bbhh *BaseBlobHashHolder
}

// ------------------------- Server request to mirror world holder -------------------------

// QueryDiskHashExist 向镜像存档持有人发起检索请求，
// 目的仅在于检索 hashes 是否命中镜像存档中的存储
func (b *BlobHashServerSide) QueryDiskHashExist(hashes []blob_hash_packet.HashWithPosition) (
	hit []blob_hash_packet.HashWithPosition,
	miss []blob_hash_packet.HashWithPosition,
) {
	request := blob_hash_packet.QueryDiskHashExist{
		Hashes: hashes,
	}
	resp, err := CallAPI(
		&request,
		func() *blob_hash_packet.QueryDiskHashExistResponse {
			return new(blob_hash_packet.QueryDiskHashExistResponse)
		},
		b.bbhh.node,
	)
	if err != nil || resp.HolderName != b.bbhh.diskHolderName || len(resp.HolderName) == 0 {
		hit = make([]blob_hash_packet.HashWithPosition, 0)
		miss = append(miss, hashes...)
		return
	}

	defer func() {
		if BlobHashDebug {
			fmt.Printf("s2m/QueryDiskHashExist: request = %v, resp = %v\n", request, resp)
		}
	}()

	hit = make([]blob_hash_packet.HashWithPosition, 0)
	miss = make([]blob_hash_packet.HashWithPosition, 0)
	for index, value := range hashes {
		if resp.States[index] {
			hit = append(hit, value)
		} else {
			miss = append(miss, value)
		}
	}

	return
}

// GetDiskHashPayload 从镜像存档持有人请求
// hashes 的二进制数据荷载。
// hit 指示命中的部分，miss 指示未命中的部分
func (b *BlobHashServerSide) GetDiskHashPayload(hashes []blob_hash_packet.HashWithPosition) (
	hit []blob_hash_packet.PayloadByHash,
	miss []blob_hash_packet.HashWithPosition,
) {
	request := blob_hash_packet.GetDiskHashPayload{
		Hashes: hashes,
	}
	resp, err := CallAPI(
		&request,
		func() *blob_hash_packet.GetDiskHashPayloadResponse {
			return new(blob_hash_packet.GetDiskHashPayloadResponse)
		},
		b.bbhh.node,
	)
	if err != nil || resp.HolderName != b.bbhh.diskHolderName || len(resp.HolderName) == 0 {
		hit = make([]blob_hash_packet.PayloadByHash, 0)
		miss = append(miss, hashes...)
		return
	}

	defer func() {
		if BlobHashDebug {
			fixedResp := make([]blob_hash_packet.HashWithPosition, 0)
			for _, value := range resp.Payload {
				fixedResp = append(fixedResp, value.Hash)
			}
			fmt.Printf("s2m/GetDiskHashPayload: request = %v, resp = %v\n", request, fixedResp)
		}
	}()

	mapping := make(map[blob_hash_packet.HashWithPosition]*blob_hash_packet.PayloadByHash)
	for _, value := range resp.Payload {
		mapping[value.Hash] = &value
	}

	hit = make([]blob_hash_packet.PayloadByHash, 0)
	miss = make([]blob_hash_packet.HashWithPosition, 0)
	for _, value := range hashes {
		if mapping[value] != nil {
			hit = append(hit, *mapping[value])
		} else {
			miss = append(miss, value)
		}
	}

	return
}

// RequireSyncHashToDisk 命令镜像存档持有人
// 将 payload 指示的 (hash, payload) 储存到镜像存档中
func (b *BlobHashServerSide) RequireSyncHashToDisk(payLoad []blob_hash_packet.PayloadByHash) {
	request := blob_hash_packet.RequireSyncHashToDisk{
		Payload: payLoad,
	}

	defer func() {
		if BlobHashDebug {
			hashes := make([]blob_hash_packet.HashWithPosition, 0)
			for _, value := range payLoad {
				hashes = append(hashes, value.Hash)
			}
			fmt.Printf("s2m/RequireSyncHashToDisk: request = %v\n", hashes)
		}
	}()

	b.bbhh.node.CallOmitResponse(request.Name(), defines.FromBytes(packet_marshal.Encode(&request)))
}

// ------------------------- Server keep alive with mirror world holder -------------------------

// autoKeepAlive 在服务者接受来自客户端的 SetHolderRequest 请求后执行，
// 用于不断地检查镜像存档持有人是否已经死亡。当死亡时，更新底层相关数据。
// 对于同一个 SetHolderRequest 而言，autoKeepAlive 应至多调用一次
func (b *BlobHashServerSide) autoKeepAlive() {
	ticker := time.NewTicker(KeepAliveDeadline)
	defer func() {
		ticker.Stop()

		if BlobHashDebug || BlobHashKeepAliveDebug {
			fmt.Println("s2m/KeepAlive: Mirror world holder was dead")
			fmt.Println("s2m/ServerDisconnected: Make mirror world holder disconnected")
		}

		disconnected := blob_hash_packet.ServerDisconnected{
			MirrorWorldHolderName: b.bbhh.diskHolderName,
		}

		b.bbhh.mu.Lock()
		b.bbhh.isDiskHolder = false
		b.bbhh.diskHolderName = ""
		b.bbhh.mu.Unlock()

		b.bbhh.node.PublishMessage(
			disconnected.Name(),
			defines.FromBytes(packet_marshal.Encode(&disconnected)),
		)
	}()

	for range ticker.C {
		request := blob_hash_packet.KeepAlive{
			UUID: uuid.NewString(),
		}

		resp, err := b.bbhh.node.CallWithResponse(
			request.Name(),
			defines.FromBytes(packet_marshal.Encode(&request)),
		).
			SetTimeout(KeepAliveDeadline).
			BlockGetResult()

		if err != nil {
			return
		}

		respBytes, err := resp.ToBytes()
		if err != nil {
			return
		}

		pk, err := packet_marshal.Decode(
			respBytes,
			func() packet_marshal.Packet {
				return new(blob_hash_packet.KeepAlive)
			},
		)
		if err != nil {
			return
		}

		if pk.(*blob_hash_packet.KeepAlive).UUID != request.UUID {
			return
		}

		if BlobHashDebug || BlobHashKeepAliveDebug {
			fmt.Println("s2m/KeepAlive: Success")
		}
	}
}

// ------------------------- Server response to client -------------------------

// onSetHolderRequest 处理来自客户端的
// “设置自身为镜像存档持有者”的请求
func (b *BlobHashServerSide) onSetHolderRequest(pk blob_hash_packet.SetHolderRequest) blob_hash_packet.SetHolderResponse {
	b.bbhh.mu.Lock()
	defer b.bbhh.mu.Unlock()

	resp := blob_hash_packet.SetHolderResponse{
		SuccessStates: len(b.bbhh.diskHolderName) == 0,
	}
	defer func() {
		if BlobHashDebug {
			fmt.Printf("s2c/SetHolderResponse: pk = %v, resp = %v\n", pk, resp)
		}
	}()

	if resp.SuccessStates {
		b.bbhh.diskHolderName = uuid.New().String()
		resp.HolderName = b.bbhh.diskHolderName
	}

	return resp
}

// onGetHashPayload 处理来自客户端的
// blob hash 请求。
//
// 服务者作为缓存数据集的权威维护者，
// 为客户端返回命中的缓存的二进制荷载
func (b *BlobHashServerSide) onGetHashPayload(pk blob_hash_packet.GetHashPayload) blob_hash_packet.GetHashPayloadResponse {
	resp := blob_hash_packet.GetHashPayloadResponse{
		Payload: make([]blob_hash_packet.PayloadByHash, 0),
	}
	request := make([]blob_hash_packet.HashWithPosition, 0)
	defer func() {
		if BlobHashDebug {
			fixedResp := make([]blob_hash_packet.HashWithPosition, 0)
			for _, value := range resp.Payload {
				fixedResp = append(fixedResp, value.Hash)
			}
			fmt.Printf("s2c/GetHashPayloadResponse: pk = %v, resp = %v\n", pk, fixedResp)
		}
	}()

	for _, value := range pk.Hashes {
		c := b.bbhh.cache.Load(uint64(value.Hash))

		if c == nil {
			var channel chan struct{}

			b.bbhh.mu.Lock()
			if b.bbhh.serverSpecial.pendingRequestBlocking != nil {
				if b.bbhh.serverSpecial.pendingRequestBlocking[value.Hash] != nil {
					channel = b.bbhh.serverSpecial.pendingRequestBlocking[value.Hash][value]
				}
			}
			b.bbhh.mu.Unlock()

			if channel != nil {
				if BlobHashDebug {
					fmt.Printf("onGetHashPayload: %v is still pending\n", value.Hash)
				}
				timer := time.NewTimer(WaitingBlobCacheComingDeadline)
				select {
				case <-channel:
					c = b.bbhh.cache.Load(uint64(value.Hash))
					if BlobHashDebug {
						fmt.Printf("onGetHashPayload: Finish %v\n", value.Hash)
					}
				case <-timer.C:
					if BlobHashDebug {
						fmt.Printf("onGetHashPayload: %v time out\n", value.Hash)
					}
				}
				timer.Stop()
			}
		}

		if c != nil {
			resp.Payload = append(resp.Payload, blob_hash_packet.PayloadByHash{
				Hash:    value,
				Payload: c,
			})
			continue
		}

		request = append(request, value)
	}

	if len(request) > 0 && len(b.bbhh.diskHolderName) != 0 {
		got, _ := b.GetDiskHashPayload(request)
		resp.Payload = append(resp.Payload, got...)

		if BlobHashDebug || BlonHashDiskHitDebug {
			hashes := make([]blob_hash_packet.HashWithPosition, 0)
			for _, value := range got {
				hashes = append(hashes, value.Hash)
			}
			pterm.Success.Printfln("onGetHashPayload: Request blob hash %v from disk and hit %v", request, hashes)
		}
	}

	return resp
}

// onClientQueryDiskHashExist 处理来自客户端的 ClientQueryDiskHashExist 请求
func (b *BlobHashServerSide) onClientQueryDiskHashExist(
	pk blob_hash_packet.ClientQueryDiskHashExist,
) blob_hash_packet.ClientQueryDiskHashExistResponse {
	if len(b.bbhh.diskHolderName) == 0 {
		return blob_hash_packet.ClientQueryDiskHashExistResponse{
			States: make([]bool, len(pk.Hashes)),
		}
	}

	request := blob_hash_packet.QueryDiskHashExist{
		Hashes: pk.Hashes,
	}
	resp, err := CallAPI(
		&request,
		func() *blob_hash_packet.QueryDiskHashExistResponse {
			return new(blob_hash_packet.QueryDiskHashExistResponse)
		},
		b.bbhh.node,
	)
	if err != nil || resp.HolderName != b.bbhh.diskHolderName || len(resp.HolderName) == 0 {
		return blob_hash_packet.ClientQueryDiskHashExistResponse{
			States: make([]bool, len(pk.Hashes)),
		}
	}

	defer func() {
		if BlobHashDebug {
			fmt.Printf("s2c/ClientQueryDiskHashExistResponse: pk = %v, resp = %v\n", pk, resp)
		}
	}()

	return blob_hash_packet.ClientQueryDiskHashExistResponse{
		States: resp.States,
	}
}

// onClientGetDiskHashPayload 处理来自客户端的 ClientGetDiskHashPayload 请求
func (b *BlobHashServerSide) onClientGetDiskHashPayload(
	pk blob_hash_packet.ClientGetDiskHashPayload,
) blob_hash_packet.ClientGetDiskHashPayloadResponse {
	if len(b.bbhh.diskHolderName) == 0 {
		return blob_hash_packet.ClientGetDiskHashPayloadResponse{
			Payload: make([]blob_hash_packet.PayloadByHash, 0),
		}
	}

	request := blob_hash_packet.GetDiskHashPayload{
		Hashes: pk.Hashes,
	}
	resp, err := CallAPI(
		&request,
		func() *blob_hash_packet.GetDiskHashPayloadResponse {
			return new(blob_hash_packet.GetDiskHashPayloadResponse)
		},
		b.bbhh.node,
	)
	if err != nil || resp.HolderName != b.bbhh.diskHolderName || len(resp.HolderName) == 0 {
		return blob_hash_packet.ClientGetDiskHashPayloadResponse{
			Payload: make([]blob_hash_packet.PayloadByHash, 0),
		}
	}

	defer func() {
		if BlobHashDebug {
			fixedResp := make([]blob_hash_packet.HashWithPosition, 0)
			for _, value := range resp.Payload {
				fixedResp = append(fixedResp, value.Hash)
			}
			fmt.Printf("s2c/ClientGetDiskHashPayload: pk = %v, resp = %v\n", pk, fixedResp)
		}
	}()

	return blob_hash_packet.ClientGetDiskHashPayloadResponse{
		Payload: resp.Payload,
	}
}

// ------------------------- End -------------------------
