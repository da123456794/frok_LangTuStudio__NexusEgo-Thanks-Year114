package blob_hash

import (
	blob_hash_interface "github.com/LangTuStudio/Conbit/Conbit/modules/blob_hash/interface"
	blob_hash_packet "github.com/LangTuStudio/Conbit/Conbit/modules/blob_hash/packet"
)

// BlobHashMirrorWorldSide ..
type BlobHashMirrorWorldSide struct {
	bbhh *BaseBlobHashHolder
}

// SetHandler 根据给定的函数设置处理器，
// 然后镜像存档的持有人便可作为资源中心
// 处理来自服务者的资源请求
func (b *BlobHashMirrorWorldSide) SetHandler(
	handleQueryDiskHashExist blob_hash_interface.HandleQueryDiskHashExist,
	handleGetDiskHashPayload blob_hash_interface.HandleGetDiskHashPayload,
	handleRequireSyncHashToDisk blob_hash_interface.HandleRequireSyncHashToDisk,
	handleCleanBlobHashAndApplyToWorld blob_hash_interface.HandleCleanBlobHashAndApplyToWorld,
	handleServerDisconnect blob_hash_interface.HandleServerDisconnect,
) {
	b.bbhh.handler.handleQueryDiskHashExist = handleQueryDiskHashExist
	b.bbhh.handler.handleGetDiskHashPayload = handleGetDiskHashPayload
	b.bbhh.handler.handleRequireSyncHashToDisk = handleRequireSyncHashToDisk
	b.bbhh.handler.handleCleanBlobHashAndApplyToWorld = handleCleanBlobHashAndApplyToWorld
	b.bbhh.handler.handleServerDisconnect = handleServerDisconnect
}

// onQueryDiskHashExist 处理来自服务者的 blob hash 查询。
// 它被镜像存档的持有人处理，返回命中镜像存档中的 blob hash，
// 但不含二进制数据荷载
func (b *BlobHashMirrorWorldSide) onQueryDiskHashExist(
	pk blob_hash_packet.QueryDiskHashExist,
) blob_hash_packet.QueryDiskHashExistResponse {
	return blob_hash_packet.QueryDiskHashExistResponse{
		HolderName: b.bbhh.diskHolderName,
		States:     b.bbhh.handler.handleQueryDiskHashExist(pk.Hashes),
	}
}

// onGetDiskHashPayload 处理来自服务者的 blob hash 请求。
// 它被镜像存档的持有人处理，返回命中镜像存档中的 blob hash，
// 并且包含二进制数据荷载
func (b *BlobHashMirrorWorldSide) onGetDiskHashPayload(
	pk blob_hash_packet.GetDiskHashPayload,
) blob_hash_packet.GetDiskHashPayloadResponse {
	return blob_hash_packet.GetDiskHashPayloadResponse{
		HolderName: b.bbhh.diskHolderName,
		Payload:    b.bbhh.handler.handleGetDiskHashPayload(pk.Hashes),
	}
}

// onRequireSyncHashToDisk ..
func (b *BlobHashMirrorWorldSide) onRequireSyncHashToDisk(pk blob_hash_packet.RequireSyncHashToDisk) {
	b.bbhh.handler.handleRequireSyncHashToDisk(pk.Payload)
}
