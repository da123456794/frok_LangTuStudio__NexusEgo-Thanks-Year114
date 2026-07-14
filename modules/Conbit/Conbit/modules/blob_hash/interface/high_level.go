package blob_hash_interface

import blob_hash_packet "github.com/LangTuStudio/Conbit/Conbit/modules/blob_hash/packet"

// BlobHashHighLevelGeneral 是一系列可同时由 服务者/客户端 调用的查询用函数
type BlobHashHighLevelGeneral interface {
	// QueryDiskHashExist 向镜像存档持有人发起检索请求，
	// 目的仅在于检索 hashes 是否命中镜像存档中的存储
	QueryDiskHashExist(hashes []blob_hash_packet.HashWithPosition) (
		hit []blob_hash_packet.HashWithPosition,
		miss []blob_hash_packet.HashWithPosition,
	)
	// GetDiskHashPayload 从镜像存档持有人请求
	// hashes 的二进制数据荷载。
	// hit 指示命中的部分，miss 指示未命中的部分
	GetDiskHashPayload(hashes []blob_hash_packet.HashWithPosition) (
		hit []blob_hash_packet.PayloadByHash,
		miss []blob_hash_packet.HashWithPosition,
	)
}

// BlobHashHighLevelServerSideSpecial 定义了一些只能由服务者使用的函数，
// 目的皆在确保镜像存档资源的持有人持有的镜像存档一定与租赁服是同步的
type BlobHashHighLevelServerSideSpecial interface {
	// RequireSyncHashToDisk 命令镜像存档持有人
	// 将 payload 指示的 (hash, payload) 储存到镜像存档中
	RequireSyncHashToDisk(payLoad []blob_hash_packet.PayloadByHash)
}

// BlobHashServerSide ..
type BlobHashServerSide interface {
	BlobHashHighLevelGeneral
	BlobHashHighLevelServerSideSpecial
}
