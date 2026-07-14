package blob_hash

import (
	"sync"
	"time"

	blob_hash_cache "github.com/LangTuStudio/Conbit/Conbit/modules/blob_hash/cache"
	blob_hash_interface "github.com/LangTuStudio/Conbit/Conbit/modules/blob_hash/interface"
	blob_hash_packet "github.com/LangTuStudio/Conbit/Conbit/modules/blob_hash/packet"
	"github.com/LangTuStudio/Conbit/nodes/defines"
)

const (
	BlobHashDebug          = false
	BlonHashDiskHitDebug   = false
	BlobHashKeepAliveDebug = false
)

const (
	KeepAliveDeadline              = time.Second * 3
	LoginSequenceDeadline          = time.Second * 60
	WaitingBlobCacheComingDeadline = time.Second * 10
	BlockingDeadline               = time.Second * 30
)

// BaseBlobHashHolder 是区块缓存系统的底层实现
type BaseBlobHashHolder struct {
	// cache 是区块缓存系统持有的 blob hash 缓存数据集
	cache *blob_hash_cache.Cache

	// isServer 指示当前区块缓存系统是否是服务者。
	// 服务者是其他所有客户端的 blob hash 缓存数据集
	// 的最终管理和持有人，它是权威的
	isServer bool
	// serverSpecial 是一系列仅被服务者所使用的字段。
	// 当且仅当 isServer 为真时非空
	serverSpecial *serverSpecial

	// isDiskHolder 指示自身是否是镜像存档的持有人
	isDiskHolder bool
	// 对于服务者而言，diskHolderName 指示镜像存档资源持有者之名。
	// 对于镜像存档资源持有人而言，diskHolderName 指示自己名字。
	// 否则，对于其他的客户端，diskHolderName 为空
	diskHolderName string
	// handler 指示镜像存档资源持有人所使用的处理函数，
	// 它们被用于处理来自服务者的 blob cache 查询和同步请求
	handler mirrorWorldHandler

	// mu 确保各项操作的原子性
	mu *sync.Mutex
	// node 指示该区块缓存系统持有的 Node
	node defines.Node
}

// serverSpecial 是一系列仅被服务者所使用的字段
type serverSpecial struct {
	// 登录序列期间服务器将会发送一些 LevelChunk 以
	// 初始化客户端生成。
	//
	// 然而，由于 Blob hash cache system 是一个 ACK 系统，
	// 而我们需要在登录序列中 ACK 这些区块，
	// 否则客户端将卡死在登录序列。
	//
	// finishLoginSequence 将在登录序列彻底完成后被关闭
	finishLoginSequence chan struct{}
	// finishLoginSequenceDoOnce 确保 finishLoginSequence
	// 至多被关闭一次
	finishLoginSequenceDoOnce *sync.Once

	// pendingRequest 指示目前仍在请求队列中的 Hash
	pendingRequest map[blob_hash_packet.Hash][]blob_hash_packet.HashWithPosition
	// pendingRequestBlocking 允许请求者以阻
	// 塞的方式等待租赁服响应 blob cache 请求
	pendingRequestBlocking map[blob_hash_packet.Hash]map[blob_hash_packet.HashWithPosition]chan struct{}
}

// mirrorWorldHandler 指示镜像存档资源持有人所使用的处理函数，
// 它们被用于处理来自客户端的 blob cache 查询和同步请求
type mirrorWorldHandler struct {
	// handleQueryDiskHashExist ..
	handleQueryDiskHashExist blob_hash_interface.HandleQueryDiskHashExist
	// handleGetDiskHashPayload ..
	handleGetDiskHashPayload blob_hash_interface.HandleGetDiskHashPayload
	// handleRequireSyncHashToDisk ..
	handleRequireSyncHashToDisk blob_hash_interface.HandleRequireSyncHashToDisk
	// handleCleanBlobHashAndApplyToWorld ..
	handleCleanBlobHashAndApplyToWorld blob_hash_interface.HandleCleanBlobHashAndApplyToWorld
	// handleServerDisconnect ..
	handleServerDisconnect blob_hash_interface.HandleServerDisconnect
}
