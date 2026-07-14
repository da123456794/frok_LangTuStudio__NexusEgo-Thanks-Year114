package blob_hash

import (
	"fmt"

	"github.com/LangTuStudio/Conbit/Conbit/chunks/define"
	blob_hash_packet "github.com/LangTuStudio/Conbit/Conbit/modules/blob_hash/packet"
	"github.com/LangTuStudio/Conbit/minecraft/protocol"
	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
	"github.com/pterm/pterm"
)

// onPlayStatus 检查 pk.Status 是否是 packet.PlayStatusPlayerSpawn。
// 如果是，则说明登录序列已然完成
func (b *BlobHashServerSide) onPlayStatus(pk *packet.PlayStatus) {
	if pk.Status == packet.PlayStatusPlayerSpawn {
		b.bbhh.serverSpecial.finishLoginSequenceDoOnce.Do(func() {
			close(b.bbhh.serverSpecial.finishLoginSequence)
		})
	}
}

// onCacheResponse 处理服务器对我们 ACK
// 给服务器的响应，且只被服务者调用。
//
// 我们将记录未被记录的 (hash, payload) 对，
// 并以 hash 作为键，payload 作为值。
//
// 完成记录后，将 blob cache 抄送至位于终结点的镜像存档持有者
func (b *BlobHashServerSide) onCacheResponse(pk *packet.ClientCacheMissResponse) {
	defer b.bbhh.gc()
	sync := make([]blob_hash_packet.PayloadByHash, 0)

	if BlobHashDebug {
		hashes := make([]uint64, 0)
		for _, value := range pk.Blobs {
			hashes = append(hashes, value.Hash)
		}
		fmt.Printf("onCacheResponse: Recive blob hash: %v\n", hashes)
	}

	b.bbhh.mu.Lock()
	for _, value := range pk.Blobs {
		h := blob_hash_packet.Hash(value.Hash)
		_ = b.bbhh.cache.Update(value.Hash, value.Payload)

		for _, v := range b.bbhh.serverSpecial.pendingRequest[h] {
			sync = append(sync, blob_hash_packet.PayloadByHash{
				Hash:    v,
				Payload: value.Payload,
			})
		}
		for _, v := range b.bbhh.serverSpecial.pendingRequestBlocking[h] {
			close(v)
		}

		delete(b.bbhh.serverSpecial.pendingRequest, h)
		delete(b.bbhh.serverSpecial.pendingRequestBlocking, h)
	}
	b.bbhh.mu.Unlock()

	if len(sync) > 0 && len(b.bbhh.diskHolderName) != 0 {
		if BlobHashDebug || BlonHashDiskHitDebug {
			hashes := make([]blob_hash_packet.HashWithPosition, 0)
			for _, value := range sync {
				hashes = append(hashes, value.Hash)
			}
			pterm.Info.Printfln("onCacheResponse: Require sync hash to disk: %v", hashes)
		}
		b.RequireSyncHashToDisk(sync)
	}
}

// onLevelChunk 从 pk 中检索 blob hash，
// 并记下已被记录和未被记录的 hash，
// 然后作为 ACK 应答给服务器。
//
// 如果镜像存档中已经存在缓存，
// 则向租赁服报告的对应字段一样返回命中
func (b *BlobHashServerSide) onLevelChunk(pk *packet.LevelChunk, sendPacket func(packet.Packet)) {
	if !pk.CacheEnabled {
		return
	}
	defer b.bbhh.gc()

	ack := packet.ClientCacheBlobStatus{
		MissHashes: make([]uint64, 0),
		HitHashes:  make([]uint64, 0),
	}
	dm := define.Dimension(pk.Dimension)

	defer func() {
		if BlobHashDebug {
			fmt.Printf("onLevelChunk: ACK Level Chunk: %v\n", ack)
		}
	}()

	// If mirror world holder is not exist,
	// then just compute miss hashes and hit hashes
	// from our local underlying blob hash cache set,
	// and send ACK to server immediately.
	if len(b.bbhh.diskHolderName) == 0 {
		for index, value := range pk.BlobHashes {
			// 0 hash means this entry is a sub chunk that full of empty,
			// and there is no need to do any other operations.
			if value == 0 {
				continue
			}
			// Then, we check current hash we have or not.
			if payload := b.bbhh.cache.Load(value); payload != nil {
				ack.HitHashes = append(ack.HitHashes, value)
			} else {
				ack.MissHashes = append(ack.MissHashes, value)
				// index is len(pk.BlobHashes)-1 means that this is a biome data,
				// and we can't add it to the pending request blocking due to our
				// code construction.
				if index != len(pk.BlobHashes)-1 {
					b.bbhh.mu.Lock()
					b.bbhh.addToPendingRequestBlocking(blob_hash_packet.HashWithPosition{
						Hash: blob_hash_packet.Hash(value),
						SubChunkPos: protocol.SubChunkPos{
							pk.Position[0],
							int32(index) + int32(dm.RangeUpperInclude()[0]>>4),
							pk.Position[1],
						},
						Dimension: uint8(dm),
					})
					b.bbhh.mu.Unlock()
				}
			}
		}
		sendPacket(&ack)
		return
	}

	request := make([]blob_hash_packet.HashWithPosition, 0)
	sync := make([]blob_hash_packet.PayloadByHash, 0)

	for index, value := range pk.BlobHashes {
		// 0 hash means this entry is a sub chunk that full of empty,
		// and there is no need to do any other operations.
		if value == 0 {
			continue
		}
		// index is len(pk.BlobHashes)-1 means this is a biome data,
		// and we don't need to sync to mirror world side,
		// so there is no need to query its states from mirror world.
		if index == len(pk.BlobHashes)-1 {
			// payload is not nil means we actually have the payload
			// of the biome data, and don't need to further operations.
			if payload := b.bbhh.cache.Load(value); payload != nil {
				ack.HitHashes = append(ack.HitHashes, value)
				continue
			}
			// Otherwise, we missing the payload of this biome data,
			// and we add it to miss hash list and waiting server to
			// send them back.
			ack.MissHashes = append(ack.MissHashes, value)
			continue
		}
		// Always query mirror holder whether they have those hashes,
		// due to it is possible for we have hash with payload,
		// but the mirror world side don't have.
		request = append(
			request,
			blob_hash_packet.HashWithPosition{
				Hash: blob_hash_packet.Hash(value),
				SubChunkPos: [3]int32{
					pk.Position[0],
					int32(index) + int32(dm.RangeUpperInclude()[0]>>4),
					pk.Position[1],
				},
				Dimension: uint8(dm),
			},
		)
	}

	if len(request) > 0 {
		// Ack mirror world holder abouth which hashes they hit and miss.
		hit, miss := b.QueryDiskHashExist(request)
		// We iter all missed hashes which given by mirror world side.
		for _, value := range miss {
			// If the payload we load from our cache is not nil,
			// then it means we have a hash with payload but mirror
			// world don't have. So, we add this to sync list and
			// then sync to the mirror world side.
			if payload := b.bbhh.cache.Load(uint64(value.Hash)); payload != nil {
				sync = append(sync, blob_hash_packet.PayloadByHash{
					Hash:    value,
					Payload: payload,
				})
				ack.HitHashes = append(ack.HitHashes, uint64(value.Hash))
				continue
			}
			// Both we and mirror world side miss this hash,
			// and we add it to ack.MissHashes and waiting server
			// to send there payload back.
			ack.MissHashes = append(ack.MissHashes, uint64(value.Hash))
			b.bbhh.mu.Lock()
			b.bbhh.addTopendingRequest(uint64(value.Hash), value)
			b.bbhh.addToPendingRequestBlocking(value)
			b.bbhh.mu.Unlock()
		}
		// hit is a list that the payload with hash
		// that mirror world side actually has.
		for _, value := range hit {
			ack.HitHashes = append(ack.HitHashes, uint64(value.Hash))
		}
	}

	// sync is not empty means that some payload with hashes we have,
	// but mirror world side don't have, and we need to sync those
	// payload and hashes to mirror world side.
	if len(sync) > 0 {
		b.RequireSyncHashToDisk(sync)
	}

	// Send ACK to server to let them know which hashes we hit,
	// and which we missing.
	sendPacket(&ack)
}

// onSubChunk 从 pk 中检索 blob hash，
// 并记下已被记录和未被记录的 hash，
// 然后作为 ACK 应答给服务器。
//
// 如果镜像存档中已经存在缓存，
// 则向租赁服报告的对应字段一样返回命中
func (b *BlobHashServerSide) onSubChunk(pk *packet.SubChunk, sendPacket func(packet.Packet)) {
	if !pk.CacheEnabled {
		return
	}
	defer b.bbhh.gc()

	ack := packet.ClientCacheBlobStatus{
		MissHashes: make([]uint64, 0),
		HitHashes:  make([]uint64, 0),
	}

	defer func() {
		if BlobHashDebug {
			fmt.Printf("onSubChunk: ACK Sub Chunk: %v\n", ack)
		}
	}()

	// If mirror world holder is not exist,
	// then just compute miss hashes and hit hashes
	// from our local underlying blob hash cache set,
	// and send ACK to server immediately.
	if len(b.bbhh.diskHolderName) == 0 {
		for _, value := range pk.SubChunkEntries {
			// 0 hash means this entry is a sub chunk that full of empty,
			// and there is no need to do any other operations.
			if value.BlobHash == 0 {
				continue
			}
			// Then, we check current hash we have or not.
			if payload := b.bbhh.cache.Load(value.BlobHash); payload != nil {
				ack.HitHashes = append(ack.HitHashes, value.BlobHash)
			} else {
				// No need to add this hash to pending request
				// due to there is no mirror world holder.
				ack.MissHashes = append(ack.MissHashes, value.BlobHash)
				b.bbhh.mu.Lock()
				b.bbhh.addToPendingRequestBlocking(blob_hash_packet.HashWithPosition{
					Hash: blob_hash_packet.Hash(value.BlobHash),
					SubChunkPos: protocol.SubChunkPos{
						pk.Position[0] + int32(value.Offset[0]),
						pk.Position[1] + int32(value.Offset[1]),
						pk.Position[2] + int32(value.Offset[2]),
					},
					Dimension: uint8(pk.Dimension),
				})
				b.bbhh.mu.Unlock()
			}
		}
		sendPacket(&ack)
		return
	}

	request := make([]blob_hash_packet.HashWithPosition, 0)
	sync := make([]blob_hash_packet.PayloadByHash, 0)

	for _, value := range pk.SubChunkEntries {
		// 0 hash means this sub chunk is full of empty,
		// and there is no need to do any other operations.
		if value.BlobHash == 0 {
			continue
		}
		// Always query mirror holder whether they have those hashes,
		// due to it is possible for we have hash with payload,
		// but the mirror world side don't have.
		request = append(
			request,
			blob_hash_packet.HashWithPosition{
				Hash: blob_hash_packet.Hash(value.BlobHash),
				SubChunkPos: [3]int32{
					pk.Position[0] + int32(value.Offset[0]),
					pk.Position[1] + int32(value.Offset[1]),
					pk.Position[2] + int32(value.Offset[2]),
				},
				Dimension: uint8(pk.Dimension),
			},
		)
	}

	if len(request) > 0 {
		// Ack mirror world holder abouth which hashes they hit and miss.
		hit, miss := b.QueryDiskHashExist(request)
		// We iter all missed hashes which given by mirror world side.
		for _, value := range miss {
			// If the payload we load from our cache is not nil,
			// then it means we have a hash with payload but mirror
			// world don't have. So, we add this to sync list and
			// then sync to the mirror world side.
			if payload := b.bbhh.cache.Load(uint64(value.Hash)); payload != nil {
				sync = append(sync, blob_hash_packet.PayloadByHash{
					Hash:    value,
					Payload: payload,
				})
				ack.HitHashes = append(ack.HitHashes, uint64(value.Hash))
				continue
			}
			// Both we and mirror world side miss this hash,
			// and we add it to ack.MissHashes and waiting server
			// to send there payload back.
			ack.MissHashes = append(ack.MissHashes, uint64(value.Hash))
			b.bbhh.mu.Lock()
			b.bbhh.addTopendingRequest(uint64(value.Hash), value)
			b.bbhh.addToPendingRequestBlocking(value)
			b.bbhh.mu.Unlock()
		}
		// hit is a list that the payload with hash
		// that mirror world side actually has.
		for _, value := range hit {
			ack.HitHashes = append(ack.HitHashes, uint64(value.Hash))
		}
	}

	// sync is not empty means that some payload with hashes we have,
	// but mirror world side don't have, and we need to sync those
	// payload and hashes to mirror world side.
	if len(sync) > 0 {
		b.RequireSyncHashToDisk(sync)
	}

	// Send ACK to server to let them know which hashes we hit,
	// and which we missing.
	sendPacket(&ack)
}
