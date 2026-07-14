package blob_hash

import (
	"fmt"

	blob_hash_packet "github.com/LangTuStudio/Conbit/Conbit/modules/blob_hash/packet"
)

// BlobHashClientSide ..
type BlobHashClientSide struct {
	bbhh *BaseBlobHashHolder
}

// SetHolderRequest 请求服务者将客户端设置为镜像存档的持有人。
// 一旦成功便不可撤销，持有人必须伴随服务者的存在而始终存在
func (b *BlobHashClientSide) SetHolderRequest() bool {
	request := blob_hash_packet.SetHolderRequest{}

	resp, err := CallAPI(
		&request,
		func() *blob_hash_packet.SetHolderResponse {
			return new(blob_hash_packet.SetHolderResponse)
		},
		b.bbhh.node,
	)
	if err != nil {
		return false
	}

	defer func() {
		if BlobHashDebug {
			fmt.Printf("c2s/SetHolderRequest: request = %v, resp = %v\n", request, resp)
		}
	}()

	if resp.SuccessStates {
		b.bbhh.mu.Lock()
		b.bbhh.isDiskHolder = true
		b.bbhh.diskHolderName = resp.HolderName
		b.bbhh.mu.Unlock()
	}
	return resp.SuccessStates
}

// BlobHashHolder 从服务者获取 hashes 对应的数据荷载。
// 底层缓存集合会因此同时更新
func (b *BlobHashClientSide) GetHashPayload(hashes []blob_hash_packet.HashWithPosition) (
	mapping map[blob_hash_packet.HashWithPosition][]byte,
) {
	mapping = make(map[blob_hash_packet.HashWithPosition][]byte)

	request := blob_hash_packet.GetHashPayload{
		Hashes: hashes,
	}
	resp, err := CallAPI(
		&request,
		func() *blob_hash_packet.GetHashPayloadResponse {
			return new(blob_hash_packet.GetHashPayloadResponse)
		},
		b.bbhh.node,
	)
	if err != nil {
		return
	}

	defer func() {
		if BlobHashDebug {
			fixedResp := make([]blob_hash_packet.HashWithPosition, 0)
			for _, value := range resp.Payload {
				fixedResp = append(fixedResp, value.Hash)
			}
			fmt.Printf("c2s/GetHashPayload: request = %v, resp = %v\n", request, fixedResp)
		}
	}()

	for _, value := range resp.Payload {
		if b.bbhh.cache.Update(uint64(value.Hash.Hash), value.Payload) {
			mapping[value.Hash] = value.Payload
		}
	}
	return
}

// QueryDiskHashExist 向镜像存档持有人发起检索请求，
// 目的仅在于检索 hashes 是否命中镜像存档中的存储
func (b *BlobHashClientSide) QueryDiskHashExist(hashes []blob_hash_packet.HashWithPosition) (
	hit []blob_hash_packet.HashWithPosition,
	miss []blob_hash_packet.HashWithPosition,
) {
	request := blob_hash_packet.ClientQueryDiskHashExist{
		QueryDiskHashExist: blob_hash_packet.QueryDiskHashExist{
			Hashes: hashes,
		},
	}
	resp, err := CallAPI(
		&request,
		func() *blob_hash_packet.ClientQueryDiskHashExistResponse {
			return new(blob_hash_packet.ClientQueryDiskHashExistResponse)
		},
		b.bbhh.node,
	)
	if err != nil {
		hit = make([]blob_hash_packet.HashWithPosition, 0)
		miss = append(miss, hashes...)
		return
	}

	defer func() {
		if BlobHashDebug {
			fmt.Printf("s2c/QueryDiskHashExist: request = %v, resp = %v\n", request, resp)
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
func (b *BlobHashClientSide) GetDiskHashPayload(hashes []blob_hash_packet.HashWithPosition) (
	hit []blob_hash_packet.PayloadByHash,
	miss []blob_hash_packet.HashWithPosition,
) {
	request := blob_hash_packet.ClientGetDiskHashPayload{
		GetDiskHashPayload: blob_hash_packet.GetDiskHashPayload{
			Hashes: hashes,
		},
	}
	resp, err := CallAPI(
		&request,
		func() *blob_hash_packet.ClientGetDiskHashPayloadResponse {
			return new(blob_hash_packet.ClientGetDiskHashPayloadResponse)
		},
		b.bbhh.node,
	)
	if err != nil {
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
			fmt.Printf("s2c/GetDiskHashPayload: request = %v, resp = %v\n", request, fixedResp)
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
