package blob_hash

import (
	"sync"
	"time"

	"github.com/LangTuStudio/Conbit/Conbit"
	blob_hash_cache "github.com/LangTuStudio/Conbit/Conbit/modules/blob_hash/cache"
	blob_hash_interface "github.com/LangTuStudio/Conbit/Conbit/modules/blob_hash/interface"
	blob_hash_packet "github.com/LangTuStudio/Conbit/Conbit/modules/blob_hash/packet"
	"github.com/LangTuStudio/Conbit/nodes/defines"
)

// BlobHashHolder 统合基于以 BaseBlobHashHolder
// 为底层的 BlobHashMirrorWorldSide, BlobHashServerSide 和
// BlobHashClientSide，然后实现了它们的共有函数
type BlobHashHolder struct {
	bbhh *BaseBlobHashHolder
}

// IsServer 返回自身是否是 blob cache 缓存数据集的权威持有人(服务者)
func (b *BlobHashHolder) IsServer() bool {
	return b.bbhh.isServer
}

// IsDiskHolder 返回当前是否是镜像存档资源的持有人
func (b *BlobHashHolder) IsDiskHolder() bool {
	return b.bbhh.isDiskHolder
}

// LoadBlobCache 从底层缓存数据集检索 hash 所指示的数据负载。
// 如果不存在，则返回 nil
func (b *BlobHashHolder) LoadBlobCache(hash uint64) []byte {
	return b.bbhh.cache.Load(hash)
}

// UpdateBlobCache 将 hash 与 payLoad 关联起来，
// 并将它们放入底层的缓存数据集合。
//
// 保证 UpdateBlobCache 在返回值后，UpdateBlobCache
// 的调用者可以安全的修改 payload 所指示的切片。
//
// UpdateBlobCache 的调用者有责任校验来自服务器的 hash
// 是否与 xxhash.Sum64(payload) 等价。
//
// 如果 UpdateBlobCache 返回假，则说明使用者发送了不正确的
// (hash, payload) 对，此时底层 blob hash cache 缓存数据
// 集将拒绝接受并不发生任何变化。
//
// 作为一种特殊情况，长度为 0 的数据荷载不会被接受，
// UpdateBlobCache 会永远为它返回假
func (b *BlobHashHolder) UpdateBlobCache(hash uint64, payload []byte) bool {
	return b.bbhh.cache.Update(hash, payload)
}

// AsServerSide 返回针对服务者实现的函数。
// 非服务者调用 AsServerSide 将得到空值。
// 可以使用 b.IsServer() 确定当前结点是否是服务者
func (b *BlobHashHolder) AsServerSide() blob_hash_interface.BlobHashServerSide {
	if !b.bbhh.isServer {
		return nil
	}
	return &BlobHashServerSide{bbhh: b.bbhh}
}

// AsMirrorWorldSide 返回针对镜像存档资源持有者实现的函数，
// 无论当前的使用者是否是镜像存档的持有人
func (b *BlobHashHolder) AsMirrorWorldSide() blob_hash_interface.BlobHashMirrorWorldSide {
	return &BlobHashMirrorWorldSide{bbhh: b.bbhh}
}

// WaitingLoginSequenceDown 阻塞并等待登录序列完成。
// 如果当前不是服务者，则无论如何都将立即返回值
func (b *BlobHashHolder) WaitingLoginSequenceDown() {
	if !b.bbhh.isServer {
		return
	}

	timer := time.NewTimer(LoginSequenceDeadline)
	defer timer.Stop()

	select {
	case <-b.bbhh.serverSpecial.finishLoginSequence:
	case <-timer.C:
		panic("WaitingLoginSequenceDown: Login sequence take too much time and result as login failed")
	}
}

// QueryDiskHashExist 向镜像存档持有人发起检索请求，
// 目的仅在于检索 hashes 是否命中镜像存档中的存储
func (b *BlobHashHolder) QueryDiskHashExist(hashes []blob_hash_packet.HashWithPosition) (
	hit []blob_hash_packet.HashWithPosition,
	miss []blob_hash_packet.HashWithPosition,
) {
	if b.bbhh.isServer {
		s := BlobHashServerSide{bbhh: b.bbhh}
		return s.QueryDiskHashExist(hashes)
	}
	m := BlobHashClientSide{bbhh: b.bbhh}
	return m.QueryDiskHashExist(hashes)
}

// GetDiskHashPayload 从镜像存档持有人请求
// hashes 的二进制数据荷载。
// hit 指示命中的部分，miss 指示未命中的部分
func (b *BlobHashHolder) GetDiskHashPayload(hashes []blob_hash_packet.HashWithPosition) (
	hit []blob_hash_packet.PayloadByHash,
	miss []blob_hash_packet.HashWithPosition,
) {
	if b.bbhh.isServer {
		s := BlobHashServerSide{bbhh: b.bbhh}
		return s.GetDiskHashPayload(hashes)
	}
	m := BlobHashClientSide{bbhh: b.bbhh}
	return m.GetDiskHashPayload(hashes)
}

// RequireSyncHashToDisk 命令镜像存档持有人
// 将 payload 指示的 (hash, payload) 储存到镜像存档中。
//
// 它只允许由服务者调用，以确保镜像资源持有人的存档一定与服务器同步。
// 任何非服务器者调用 RequireSyncHashToDisk 不会产生任何效果
func (b *BlobHashHolder) RequireSyncHashToDisk(payLoad []blob_hash_packet.PayloadByHash) {
	if !b.bbhh.isServer {
		return
	}
	s := BlobHashServerSide{bbhh: b.bbhh}
	s.RequireSyncHashToDisk(payLoad)
}

// SetHolderRequest 请求服务者将客户端设置为镜像存档的持有人。
// 一旦成功便不可撤销，持有人必须伴随服务者的存在而始终存在
func (b *BlobHashHolder) SetHolderRequest() bool {
	m := BlobHashClientSide{bbhh: b.bbhh}
	return m.SetHolderRequest()
}

// BlobHashHolder 从服务者获取 hashes 对应的数据荷载。
// 底层缓存集合会因此同时更新
func (b *BlobHashHolder) GetHashPayload(hashes []blob_hash_packet.HashWithPosition) (
	mapping map[blob_hash_packet.HashWithPosition][]byte,
) {
	m := BlobHashClientSide{bbhh: b.bbhh}
	return m.GetHashPayload(hashes)
}

// NewBlobHashHolder 为使用者
// 创建一个区块系统缓存持有器。
// maxHashAllow 指示底层最多可持有的缓存数量。
//
// 如果 isServer 为真，
// 则使当前使用者是接入点，
// 并且其将作为自身和其他所有终结点的
// blob hash 缓存数据集的权威持有人。
//
// 缓存数据集的权威持有人将作为服务者为
// 其他接入点和终结点提供 blob hash
// 查询和同步服务。
//
// 我们将权威持有人称为服务者，
// 而相对的，其他使用者便是客户端。
// 特别地，服务者亦可作为一种特殊的客户端。
//
// uq 持有机器人信息；
// interact 是交互实现；
// react 是侦听实现；
// node 是传入的接入点或终结点的结点
func NewBlobHashHolder(
	maxHashAllow int,
	isServer bool,
	uq Conbit.MicroUQHolder, interact Conbit.InteractCore, react Conbit.ReactCore,
	node defines.Node,
) *BlobHashHolder {
	bbhh := &BaseBlobHashHolder{
		isServer:      isServer,
		serverSpecial: nil,
		cache:         blob_hash_cache.NewCache(maxHashAllow),
		handler: mirrorWorldHandler{
			handleQueryDiskHashExist:           nil,
			handleGetDiskHashPayload:           nil,
			handleRequireSyncHashToDisk:        nil,
			handleCleanBlobHashAndApplyToWorld: nil,
			handleServerDisconnect:             nil,
		},
		isDiskHolder:   false,
		diskHolderName: "",
		mu:             new(sync.Mutex),
		node:           node,
	}

	if bbhh.isServer {
		bbhh.serverSpecial = &serverSpecial{
			finishLoginSequence:       make(chan struct{}),
			finishLoginSequenceDoOnce: new(sync.Once),
			pendingRequest:            make(map[blob_hash_packet.Hash][]blob_hash_packet.HashWithPosition),
			pendingRequestBlocking:    make(map[blob_hash_packet.Hash]map[blob_hash_packet.HashWithPosition]chan struct{}),
		}
		s := BlobHashServerSide{bbhh: bbhh}
		s.registerListener(react, interact)
	}

	return &BlobHashHolder{bbhh: bbhh}
}
