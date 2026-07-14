package define

import (
	"encoding/binary"

	chunk_define "github.com/LangTuStudio/Conbit/Conbit/chunks/define"
)

const (
	KeySubChunkData = '/' // 2f
)

const (
	KeyVersion       = ',' // 2c
	KeyVersionOld    = 'v' // 76
	KeyBlockEntities = '1' // 31
	KeyEntitiesOld   = '2' // 32
	KeyPendingTicks  = '3'
	KeyFinalisation  = '6' // 36
	Key3DData        = '+' // 2b
	Key2DData        = '-' // 2d
	KeyChecksums     = ';' // 3b

	KeyEntityIdentifiers = "digp"
	KeyEntity            = "actorprefix"

	KeyChunkTimeStamp       = 'T'
	KeyDeltaUpdateTimeStamp = "dutsp"
	KeyDeltaUpdate          = "dup"
	KeyBlobHash             = "blobhashprefix"
)

const (
	FinalisationNeedsTicked = iota
	FinalisationNeedsPopulated
	FinalisationGenerated
)

func Index(dm chunk_define.Dimension, position chunk_define.ChunkPos) []byte {
	x, z, dim := uint32(position[0]), uint32(position[1]), uint32(dm)
	b := make([]byte, 12)

	binary.LittleEndian.PutUint32(b, x)
	binary.LittleEndian.PutUint32(b[4:], z)
	if dim == 0 {
		return b[:8]
	}
	binary.LittleEndian.PutUint32(b[8:], dim)
	return b
}

func Sum(dm chunk_define.Dimension, position chunk_define.ChunkPos, p ...byte) []byte {
	return append(Index(dm, position), p...)
}
