package global

import (
	"nexus/utils/mirror"
)

type ChunkWriteFn func(chunk *mirror.ChunkData)
