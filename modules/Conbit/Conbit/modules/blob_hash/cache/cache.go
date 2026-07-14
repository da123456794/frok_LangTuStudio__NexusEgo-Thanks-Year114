package blob_hash_cache

import (
	"fmt"
	"sync"
	"time"

	blob_hash_packet "github.com/LangTuStudio/Conbit/Conbit/modules/blob_hash/packet"
	"github.com/cespare/xxhash/v2"
)

// NewCache 创建一个容量最大为 maxHashAllow 的缓存数据集。
// 当缓存数据集容量达到上限时，将会触发垃圾回收。
//
// maxHashAllow 不得低于 MaxHashAllowLower，否则不能严格
// 保证单个区块正在使用的 blob hash 不会被意外地垃圾回收
func NewCache(maxHashAllow int) *Cache {
	return &Cache{
		mapping:        make(map[blob_hash_packet.Hash]timeAndPayload),
		maxHashAllow:   max(MaxHashAllowLower, maxHashAllow),
		mu:             new(sync.RWMutex),
		gcIsInSchedule: false,
		lastGCTime:     time.Time{},
	}
}

// Update 将 hash 与 payLoad 关联起来，并将它
// 们放入底层的缓存数据集合。
//
// 保证 Update 在返回值后，Update 的调用者可以
// 安全的修改 payload 所指示的切片。
//
// Update 的调用者有责任校验来自服务器的 hash
// 是否与 xxhash.Sum64(payload) 等价。
//
// 如果 Update 返回假，则说明使用者发送了不正确的
// (hash, payload) 对，此时底层 blob hash cache
// 缓存数据集将拒绝接受并不发生任何变化。
//
// 作为一种特殊情况，长度为 0 的数据荷载不会被接受，
// Update 会永远为它返回假
func (c *Cache) Update(hash uint64, payload []byte) bool {
	// payload with 0 length is not allowed.
	// It's better to panic here, but we hope
	// Python side would not crashed immediately.
	if len(payload) == 0 {
		return false
	}

	// By this way, len(payload) is not 0,
	// and we can start to deep copy payload.
	copiedPayload := make([]byte, len(payload))
	copy(copiedPayload, payload)

	// If the debug is enable, then we print we update the hash.
	if BlobHashCacheDebug {
		fmt.Println("Cache: Update", hash)
	}

	// It's our responsibility to check this (hash, payload)
	// is verified or not. For a invalid (hash, payload),
	// it's necessary for us to reject.
	if realHash := xxhash.Sum64(payload); hash != realHash {
		if BlobHashCacheDebug || BlobHashCachePrintVerifyError {
			fmt.Printf(
				"[WARN] Cache: Failed to verify passed payload (given hash is %d but actually they are %d)\n",
				hash, realHash,
			)
		}
		return false
	}

	// By pass the verification,
	// we can start to save this
	// (hash, payload) object.
	c.mu.Lock()
	defer c.mu.Unlock()

	// Not only save the (hash, payload)
	// itself, we also save the update time
	// of this object.
	h := blob_hash_packet.Hash(hash)
	c.mapping[h] = timeAndPayload{
		LastReciveUnixTime: time.Now().Unix(),
		Payload:            copiedPayload,
	}

	// The number of caches cannot be unlimited,
	// which means we may need to reclaim the excess caches.
	if len(c.mapping) > c.maxHashAllow {
		c.gcBySchedule()
	}

	return true
}

// Load 从底层检索 hash 所指示的数据负载。
//
// 保证返回的 payload 是深拷贝的，这意味着
// Load 的调用者可以安全的修改返回的负载。
//
// 如果 hash 没有命中缓存数据集，则返回 nil
func (c *Cache) Load(hash uint64) (payload []byte) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Load hash from c.mapping.
	result, ok := c.mapping[blob_hash_packet.Hash(hash)]

	// If the debug is enable, then we print hit states.
	if BlobHashCacheDebug {
		if ok {
			fmt.Println("Cache: Load", hash)
		} else {
			fmt.Printf("Cache: Load %d but not found\n", hash)
		}
	}

	// If the hash is not hit, we return nil
	// but not a non-nil []byte with 0 length.
	if !ok {
		return nil
	}

	// Or, the hash we hit, and we make payload
	// as non-nil and do deep copy.
	payload = make([]byte, len(result.Payload))
	copy(payload, result.Payload)

	// It's safe to use `return` but not `return payload`,
	// but I want to point out this and show the difference
	// with `if !ok {return nil}`.
	return payload
}
