package blob_hash

import (
	"fmt"

	"github.com/LangTuStudio/Conbit/Conbit"
	blob_hash_packet "github.com/LangTuStudio/Conbit/Conbit/modules/blob_hash/packet"
	pkt "github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
	"github.com/LangTuStudio/Conbit/nodes/defines"
	"github.com/LangTuStudio/Conbit/utils/packet_marshal"
)

// registerListener 为服务者注册其所使用的监听器，
// registerListener 应当最多调用一次
func (b *BlobHashServerSide) registerListener(react Conbit.ReactCore, interact Conbit.InteractCore) {
	// MC Packet
	{
		react.SetTypedPacketCallBack(pkt.IDPlayStatus, func(p pkt.Packet) {
			b.onPlayStatus(p.(*pkt.PlayStatus))
		}, true)
		react.SetTypedPacketCallBack(pkt.IDClientCacheMissResponse, func(p pkt.Packet) {
			b.onCacheResponse(p.(*pkt.ClientCacheMissResponse))
		}, true)
		react.SetTypedPacketCallBack(pkt.IDLevelChunk, func(p pkt.Packet) {
			b.onLevelChunk(p.(*pkt.LevelChunk), interact.SendPacket)
		}, true)
		react.SetTypedPacketCallBack(pkt.IDSubChunk, func(p pkt.Packet) {
			b.onSubChunk(p.(*pkt.SubChunk), interact.SendPacket)
		}, true)
	}

	// Handle client request
	{
		b.bbhh.node.ExposeAPI("blob-hash-set-holder-request").CallBackAPI(
			func(in defines.Values, setResult func(defines.Values, error)) {
				bytes, err := in.ToBytes()
				if err != nil {
					return
				}

				pk, err := packet_marshal.Decode(
					bytes,
					func() packet_marshal.Packet {
						return new(blob_hash_packet.SetHolderRequest)
					},
				)
				if err != nil {
					return
				}

				resp := b.onSetHolderRequest(*(pk.(*blob_hash_packet.SetHolderRequest)))
				setResult(defines.FromBytes(packet_marshal.Encode(&resp)), nil)

				if resp.SuccessStates {
					go b.autoKeepAlive()
				}
			},
		)

		b.bbhh.node.ExposeAPI("blob-hash-get-hash-payload").CallBackAPI(
			func(in defines.Values, setResult func(defines.Values, error)) {
				bytes, err := in.ToBytes()
				if err != nil {
					return
				}

				pk, err := packet_marshal.Decode(
					bytes,
					func() packet_marshal.Packet {
						return new(blob_hash_packet.GetHashPayload)
					},
				)
				if err != nil {
					return
				}

				go func() {
					resp := b.onGetHashPayload(*(pk.(*blob_hash_packet.GetHashPayload)))
					setResult(defines.FromBytes(packet_marshal.Encode(&resp)), nil)
				}()
			},
		)

		b.bbhh.node.ExposeAPI("blob-hash-client-query-disk-hash-exist").CallBackAPI(
			func(in defines.Values, setResult func(defines.Values, error)) {
				bytes, err := in.ToBytes()
				if err != nil {
					return
				}

				pk, err := packet_marshal.Decode(
					bytes,
					func() packet_marshal.Packet {
						return new(blob_hash_packet.ClientQueryDiskHashExist)
					},
				)
				if err != nil {
					return
				}

				go func() {
					resp := b.onClientQueryDiskHashExist(*(pk.(*blob_hash_packet.ClientQueryDiskHashExist)))
					setResult(defines.FromBytes(packet_marshal.Encode(&resp)), nil)
				}()
			},
		)

		b.bbhh.node.ExposeAPI("blob-hash-client-get-disk-hash-payload").CallBackAPI(
			func(in defines.Values, setResult func(defines.Values, error)) {
				bytes, err := in.ToBytes()
				if err != nil {
					return
				}

				pk, err := packet_marshal.Decode(
					bytes,
					func() packet_marshal.Packet {
						return new(blob_hash_packet.ClientGetDiskHashPayload)
					},
				)
				if err != nil {
					return
				}

				go func() {
					resp := b.onClientGetDiskHashPayload(*(pk.(*blob_hash_packet.ClientGetDiskHashPayload)))
					setResult(defines.FromBytes(packet_marshal.Encode(&resp)), nil)
				}()
			},
		)
	}
}

// RegisteListener 为镜像资源持有者注册其所使用的监听器，
// RegisteListener 应当最多被调用一次
func (b *BlobHashMirrorWorldSide) RegisteListener(
	setTypedPacketCallBack func(packetID uint32, callback func(pkt.Packet), newGoroutine bool),
) {

	// MC Packet
	{
		setTypedPacketCallBack(pkt.IDSubChunk, func(p pkt.Packet) {
			b.onSubChunk(p.(*pkt.SubChunk))
		}, true)
		setTypedPacketCallBack(pkt.IDLevelChunk, func(p pkt.Packet) {
			b.onLevelChunk(p.(*pkt.LevelChunk))
		}, true)
	}

	// Handle server keep alive and disconnected
	{
		b.bbhh.node.ExposeAPI("blob-hash-keep-alive").InstantAPI(
			func(in defines.Values) (defines.Values, error) {
				if BlobHashDebug || BlobHashKeepAliveDebug {
					fmt.Println("m2s/KeepAlive: Keep alive")
				}
				return in, nil
			},
		)

		b.bbhh.node.ListenMessage("blob-hash-server-disconnected", func(msg defines.Values) {
			rawBytes, err := msg.ToBytes()
			if err != nil {
				return
			}

			pk, err := packet_marshal.Decode(
				rawBytes,
				func() packet_marshal.Packet {
					return new(blob_hash_packet.ServerDisconnected)
				},
			)
			if err != nil {
				return
			}

			if pk.(*blob_hash_packet.ServerDisconnected).MirrorWorldHolderName != b.bbhh.diskHolderName {
				return
			}

			if BlobHashDebug || BlobHashKeepAliveDebug {
				fmt.Println("m2s/ServerDisconnected: Server cancelled our holder status due to we dead")
			}

			b.bbhh.mu.Lock()
			b.bbhh.isDiskHolder = false
			b.bbhh.diskHolderName = ""
			b.bbhh.mu.Unlock()

			// Server thought we were dead,
			// however, we are still alive,
			// so we try to re-set as holder
			// again.
			client := BlobHashClientSide{bbhh: b.bbhh}
			go func() {
				// If re-set request failed,
				// then we lost the status of
				// mirror world holder.
				recoverStates := client.SetHolderRequest()
				if recoverStates {
					return
				}

				if BlobHashDebug || BlobHashKeepAliveDebug {
					fmt.Println("Mirror world holder: Failed to recover")
				}

				if b.bbhh.handler.handleServerDisconnect != nil {
					b.bbhh.handler.handleServerDisconnect()
				}
			}()
		}, true)
	}

	// Handle server request
	{
		b.bbhh.node.ExposeAPI("blob-hash-query-disk-hash-exist").CallBackAPI(
			func(in defines.Values, setResult func(defines.Values, error)) {
				bytes, err := in.ToBytes()
				if err != nil {
					return
				}

				if !b.bbhh.isDiskHolder || b.bbhh.handler.handleQueryDiskHashExist == nil {
					return
				}

				pk, err := packet_marshal.Decode(
					bytes,
					func() packet_marshal.Packet {
						return new(blob_hash_packet.QueryDiskHashExist)
					},
				)
				if err != nil {
					return
				}

				resp := b.onQueryDiskHashExist(*(pk.(*blob_hash_packet.QueryDiskHashExist)))
				setResult(defines.FromBytes(packet_marshal.Encode(&resp)), nil)
			},
		)

		b.bbhh.node.ExposeAPI("blob-hash-get-disk-hash-payload").CallBackAPI(
			func(in defines.Values, setResult func(defines.Values, error)) {
				bytes, err := in.ToBytes()
				if err != nil {
					return
				}

				if !b.bbhh.isDiskHolder || b.bbhh.handler.handleGetDiskHashPayload == nil {
					return
				}

				pk, err := packet_marshal.Decode(
					bytes,
					func() packet_marshal.Packet {
						return new(blob_hash_packet.GetDiskHashPayload)
					},
				)
				if err != nil {
					return
				}

				resp := b.onGetDiskHashPayload(*(pk.(*blob_hash_packet.GetDiskHashPayload)))
				setResult(defines.FromBytes(packet_marshal.Encode(&resp)), nil)
			},
		)

		b.bbhh.node.ExposeAPI("blob-hash-require-sync-hash-to-disk").CallBackAPI(
			func(in defines.Values, setResult func(defines.Values, error)) {
				bytes, err := in.ToBytes()
				if err != nil {
					return
				}

				if !b.bbhh.isDiskHolder || b.bbhh.handler.handleRequireSyncHashToDisk == nil {
					return
				}

				pk, err := packet_marshal.Decode(
					bytes,
					func() packet_marshal.Packet {
						return new(blob_hash_packet.RequireSyncHashToDisk)
					},
				)
				if err != nil {
					return
				}

				b.onRequireSyncHashToDisk(*(pk.(*blob_hash_packet.RequireSyncHashToDisk)))
				setResult(defines.FromBytes([]byte{}), nil)
			},
		)
	}
}
