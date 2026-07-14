package blob_hash_packet

import "github.com/LangTuStudio/Conbit/minecraft/protocol"

// ------------------------- SetHolderRequest -------------------------

// SetHolderRequest ..
type SetHolderRequest struct{}

func (pk *SetHolderRequest) Name() string {
	return "blob-hash-set-holder-request"
}

func (pk *SetHolderRequest) Marshal(io protocol.IO) {}

// SetHolderResponse ..
type SetHolderResponse struct {
	SuccessStates bool
	HolderName    string
}

func (pk *SetHolderResponse) Name() string {
	return "blob-hash-set-holder-response"
}

func (pk *SetHolderResponse) Marshal(io protocol.IO) {
	io.Bool(&pk.SuccessStates)
	io.StringUTF(&pk.HolderName)
}

// ------------------------- GetHashPayload -------------------------

// GetHashPayload ..
type GetHashPayload struct {
	Hashes []HashWithPosition
}

func (pk *GetHashPayload) Name() string {
	return "blob-hash-get-hash-payload"
}

func (pk *GetHashPayload) Marshal(io protocol.IO) {
	protocol.SliceUint16Length(io, &pk.Hashes)
}

// GetHashPayloadResponse ..
type GetHashPayloadResponse struct {
	Payload []PayloadByHash
}

func (pk *GetHashPayloadResponse) Name() string {
	return "blob-hash-get-hash-payload-response"
}

// GetHashPayloadResponse ..
func (pk *GetHashPayloadResponse) Marshal(io protocol.IO) {
	protocol.SliceUint16Length(io, &pk.Payload)
}

// ------------------------- ClientQueryDiskHashExist -------------------------

// ClientQueryDiskHashExist ..
type ClientQueryDiskHashExist struct {
	QueryDiskHashExist
}

func (pk *ClientQueryDiskHashExist) Name() string {
	return "blob-hash-client-query-disk-hash-exist"
}

func (pk *ClientQueryDiskHashExist) Marshal(io protocol.IO) {
	protocol.Single(io, &pk.QueryDiskHashExist)
}

// ClientQueryDiskHashExistResponse ..
type ClientQueryDiskHashExistResponse struct {
	States []bool
}

func (pk *ClientQueryDiskHashExistResponse) Name() string {
	return "blob-hash-client-query-disk-hash-exist-response"
}

func (pk *ClientQueryDiskHashExistResponse) Marshal(io protocol.IO) {
	protocol.FuncSliceUint16Length(io, &pk.States, io.Bool)
}

// ------------------------- ClientGetDiskHashPayload -------------------------

// ClientGetDiskHashPayload ..
type ClientGetDiskHashPayload struct {
	GetDiskHashPayload
}

func (pk *ClientGetDiskHashPayload) Name() string {
	return "blob-hash-client-get-disk-hash-payload"
}

func (pk *ClientGetDiskHashPayload) Marshal(io protocol.IO) {
	protocol.Single(io, &pk.GetDiskHashPayload)
}

// ClientGetDiskHashPayloadResponse ..
type ClientGetDiskHashPayloadResponse struct {
	Payload []PayloadByHash
}

func (pk *ClientGetDiskHashPayloadResponse) Name() string {
	return "blob-hash-client-get-disk-hash-payload-response"
}

func (pk *ClientGetDiskHashPayloadResponse) Marshal(io protocol.IO) {
	protocol.SliceUint16Length(io, &pk.Payload)
}

// ------------------------- End -------------------------
