package blob_hash_cache

import (
	"sync"
	"time"

	blob_hash_packet "github.com/LangTuStudio/Conbit/Conbit/modules/blob_hash/packet"
)

const GCScheduleTime = time.Second * 30

const (
	MaxHashAllowDefault = MaxHashAllowMiddle
	MaxHashAllowLower   = 512
	MaxHashAllowMiddle  = 1024
	MaxHashAllowHigher  = 2048
)

const (
	BlobHashCacheDebug            = false
	BlobHashCachePrintVerifyError = true
	BlobHashCacheValidTime        = 30
	BlobHashListPopPercent        = 0.25
)

// timeAndPayload ..
type timeAndPayload struct {
	LastReciveUnixTime int64
	Payload            []byte
}

// hashTimePayload ..
type hashTimePayload struct {
	Hash               blob_hash_packet.Hash
	LastReciveUnixTime int64
	Payload            []byte
}

// Cache 是 blob hash 的缓存数据集实现，
// 它通过基于追踪缓存最后更新时间的方式
// 来确保区块正在使用的 blob hash 不会
// 因为垃圾释放而意外删除。
//
// 其内部已经使用了读写锁，
// 因此这将是线程安全的
type Cache struct {
	// mapping 指示每个缓存到相应数据荷载的映射，
	// 它同时会记录每个缓存的最后使用时间
	mapping map[blob_hash_packet.Hash]timeAndPayload
	// maxHashAllow 指示最多可用的缓存数
	maxHashAllow int
	// mu 确保线程安全
	mu *sync.RWMutex
	// gcIsInSchedule 指示是否已经存在已计划的垃圾回收
	gcIsInSchedule bool
	// lastGCTime 是上一次垃圾回收的时间
	lastGCTime time.Time
}
