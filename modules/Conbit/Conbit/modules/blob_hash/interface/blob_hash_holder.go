package blob_hash_interface

import blob_hash_packet "github.com/LangTuStudio/Conbit/Conbit/modules/blob_hash/packet"

// BlobHashHolderInfo 供使用者查询当前结点 blob cache 信息
type BlobHashHolderInfo interface {
	// IsServer 返回自身是否是 blob cache 缓存数据集的权威持有人(服务者)
	IsServer() bool
	// IsDiskHolder 返回当前是否是镜像存档资源的持有人
	IsDiskHolder() bool
}

// LoginSequence ..
type LoginSequence interface {
	// WaitingLoginSequenceDown 阻塞并等待登录序列完成。
	// 如果当前不是服务者，则无论如何都将立即返回值
	WaitingLoginSequenceDown()
}

// BlobHashHolderAction 指示一些 blob hash 操作
type BlobHashHolderAction interface {
	// LoadBlobCache 从底层缓存数据集检索 hash 所指示的数据负载。
	// 如果不存在，则返回 nil
	LoadBlobCache(hash uint64) []byte
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
	UpdateBlobCache(hash uint64, payload []byte) bool
	// AsServerSide 返回针对服务者实现的函数。
	// 非服务者调用 AsServerSide 将得到空值。
	// 可以使用 b.IsServer() 确定当前结点是否是服务者
	AsServerSide() BlobHashServerSide
	// AsMirrorWorldSide 返回针对镜像存档资源持有者实现的函数，
	// 无论当前的使用者是否是镜像存档的持有人
	AsMirrorWorldSide() BlobHashMirrorWorldSide
	// SetHolderRequest 请求服务者将客户端设置为镜像存档的持有人。
	// 一旦成功便不可撤销，持有人必须伴随服务者的存在而始终存在
	SetHolderRequest() bool
}

// BlobHashHolderHighLevel 是最终提供的，
// 一系列可同时由 服务者/客户端 使用的函数
type BlobHashHolderHighLevel interface {
	BlobHashHighLevelGeneral
	// BlobHashHolder 从服务者获取 hashes 对应的数据荷载。
	// 底层缓存集合会因此同时更新
	GetHashPayload(hashes []blob_hash_packet.HashWithPosition) (mapping map[blob_hash_packet.HashWithPosition][]byte)
}
