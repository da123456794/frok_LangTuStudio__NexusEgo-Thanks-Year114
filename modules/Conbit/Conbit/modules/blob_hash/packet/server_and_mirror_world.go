package blob_hash_packet

import "github.com/LangTuStudio/Conbit/minecraft/protocol"

// ------------------------- KeepAlive -------------------------

type KeepAlive struct {
	UUID string
}

func (pk *KeepAlive) Name() string {
	return "blob-hash-keep-alive"
}

func (pk *KeepAlive) Marshal(io protocol.IO) {
	io.StringUTF(&pk.UUID)
}

// ------------------------- ServerDisconnected -------------------------

type ServerDisconnected struct {
	MirrorWorldHolderName string
}

func (pk *ServerDisconnected) Name() string {
	return "blob-hash-server-disconnected"
}

func (pk *ServerDisconnected) Marshal(io protocol.IO) {
	io.StringUTF(&pk.MirrorWorldHolderName)
}

// ------------------------- QueryDiskHashExist -------------------------

// QueryDiskHashExist ..
type QueryDiskHashExist struct {
	Hashes []HashWithPosition
}

func (pk *QueryDiskHashExist) Name() string {
	return "blob-hash-query-disk-hash-exist"
}

func (pk *QueryDiskHashExist) Marshal(io protocol.IO) {
	protocol.SliceUint16Length(io, &pk.Hashes)
}

// QueryDiskHashExistResponse ..
type QueryDiskHashExistResponse struct {
	HolderName string
	States     []bool
}

func (pk *QueryDiskHashExistResponse) Name() string {
	return "blob-hash-query-disk-hash-exist-response"
}

func (pk *QueryDiskHashExistResponse) Marshal(io protocol.IO) {
	io.StringUTF(&pk.HolderName)
	protocol.FuncSliceUint16Length(io, &pk.States, io.Bool)
}

// ------------------------- GetDiskHashPayload -------------------------

// GetDiskHashPayload ..
type GetDiskHashPayload struct {
	Hashes []HashWithPosition
}

func (pk *GetDiskHashPayload) Name() string {
	return "blob-hash-get-disk-hash-payload"
}

func (pk *GetDiskHashPayload) Marshal(io protocol.IO) {
	protocol.SliceUint16Length(io, &pk.Hashes)
}

// GetDiskHashPayloadResponse ..
type GetDiskHashPayloadResponse struct {
	HolderName string
	Payload    []PayloadByHash
}

func (pk *GetDiskHashPayloadResponse) Name() string {
	return "blob-hash-get-disk-hash-payload-response"
}

func (pk *GetDiskHashPayloadResponse) Marshal(io protocol.IO) {
	io.StringUTF(&pk.HolderName)
	protocol.SliceUint16Length(io, &pk.Payload)
}

// ------------------------- RequireSyncHashToDisk -------------------------

// RequireSyncHashToDisk ..
type RequireSyncHashToDisk struct {
	Payload []PayloadByHash
}

func (pk *RequireSyncHashToDisk) Name() string {
	return "blob-hash-require-sync-hash-to-disk"
}

func (pk *RequireSyncHashToDisk) Marshal(io protocol.IO) {
	protocol.SliceUint16Length(io, &pk.Payload)
}

// ------------------------- End -------------------------
