package structure

import (
	"errors"
	"fmt"
	"os"

	"github.com/TriM-Organization/bedrock-world-operator/chunk"
	bwo_define "github.com/TriM-Organization/bedrock-world-operator/define"
	"github.com/TriM-Organization/bedrock-world-operator/world"

	"github.com/Yeah114/WaterStructure/define"
)

var ErrReaderNotImplemented = errors.New("璇诲彇鍣ㄦ湭瀹炵幇")

type BaseReader struct {
}

func (BaseReader) ID() uint8 {
	return 0
}

func (BaseReader) Name() string {
	return "BaseReader"
}

func (BaseReader) GetOffsetPos() define.Offset {
	panic(ErrReaderNotImplemented)
}

func (BaseReader) SetOffsetPos(define.Offset) {
	panic(ErrReaderNotImplemented)
}

func (BaseReader) GetSize() define.Size {
	panic(ErrReaderNotImplemented)
}

func (BaseReader) GetChunks([]define.ChunkPos) (map[define.ChunkPos]*chunk.Chunk, error) {
	return nil, ErrReaderNotImplemented
}

func (BaseReader) GetChunksNBT([]define.ChunkPos) (map[define.ChunkPos]map[define.BlockPos]map[string]any, error) {
	return nil, ErrReaderNotImplemented
}

func (BaseReader) FromFile(file *os.File) error {
	return ErrReaderNotImplemented
}

func (BaseReader) FromMCWorld(
	world *world.BedrockWorld,
	target *os.File,
	point1BlockPos define.BlockPos,
	point2BlockPos define.BlockPos,
	startCallback func(int),
	progressCallback func(),
) error {
	return ErrReaderNotImplemented
}

func (BaseReader) CountNonAirBlocks() (int, error) {
	return 0, ErrReaderNotImplemented
}

func (BaseReader) Close() error {
	return ErrReaderNotImplemented
}

func (b BaseReader) ToMCWorld(
	bedrockWorld *world.BedrockWorld,
	startSubChunkPos define.SubChunkPos,
	startCallback func(int),
	progressCallback func(),
) error {
	return convertReaderToMCWorld(b, bedrockWorld, startSubChunkPos, startCallback, progressCallback)
}

func convertReaderToMCWorld(
	reader Structure,
	bedrockWorld *world.BedrockWorld,
	startSubChunkPos bwo_define.SubChunkPos,
	startCallback func(int),
	progressCallback func(),
) error {
	if reader == nil {
		return ErrReaderNotImplemented
	}
	if bedrockWorld == nil {
		return errors.New("bedrock 涓栫晫涓?nil")
	}

	size := reader.GetSize()
	xCount := size.GetChunkXCount()
	zCount := size.GetChunkZCount()
	totalChunks := xCount * zCount

	if startCallback != nil {
		startCallback(totalChunks)
	}
	if totalChunks == 0 {
		return nil
	}

	chunkOffsetX := startSubChunkPos.X()
	chunkOffsetZ := startSubChunkPos.Z()
	blockYOffset := startSubChunkPos.Y() * 16

	const batchSize = 8
	return forEachReaderChunkBatch(xCount, zCount, batchSize, func(batch []define.ChunkPos) error {
		chunks, err := reader.GetChunks(batch)
		if err != nil {
			return fmt.Errorf("鑾峰彇鍖哄潡澶辫触: %w", err)
		}
		for _, pos := range batch {
			chunkData, ok := chunks[pos]
			if ok && chunkData != nil {
				chunkData.Compact()
				targetPos := bwo_define.ChunkPos{
					pos.X() + chunkOffsetX,
					pos.Z() + chunkOffsetZ,
				}
				if err := bedrockWorld.SaveChunk(bwo_define.DimensionIDOverworld, targetPos, chunkData); err != nil {
					return fmt.Errorf("淇濆瓨鍖哄潡 %v 澶辫触: %w", targetPos, err)
				}
			}
			if progressCallback != nil {
				progressCallback()
			}
		}

		chunkNBTs, err := reader.GetChunksNBT(batch)
		if err != nil {
			return fmt.Errorf("鑾峰彇鍖哄潡 NBT 澶辫触: %w", err)
		}
		for cpos, blockMap := range chunkNBTs {
			if len(blockMap) == 0 {
				continue
			}

			list := make([]map[string]any, 0, len(blockMap))
			absChunkX := (cpos.X() + chunkOffsetX) * 16
			absChunkZ := (cpos.Z() + chunkOffsetZ) * 16
			for bpos, nbtData := range blockMap {
				if nbtData == nil {
					continue
				}

				entry := make(map[string]any, len(nbtData)+3)
				for key, value := range nbtData {
					entry[key] = value
				}
				entry["x"] = absChunkX + bpos.X()
				entry["y"] = blockYOffset + bpos.Y() + 64
				entry["z"] = absChunkZ + bpos.Z()
				list = append(list, entry)
			}
			if len(list) == 0 {
				continue
			}

			targetPos := bwo_define.ChunkPos{
				cpos.X() + chunkOffsetX,
				cpos.Z() + chunkOffsetZ,
			}
			if err := bedrockWorld.SaveNBT(bwo_define.DimensionIDOverworld, targetPos, list); err != nil {
				return fmt.Errorf("淇濆瓨鍖哄潡 NBT %v 澶辫触: %w", targetPos, err)
			}
		}
		return nil
	})
}

func forEachReaderChunkBatch(xCount, zCount, batchSize int, fn func([]define.ChunkPos) error) error {
	if batchSize <= 0 {
		batchSize = 1
	}

	batch := make([]define.ChunkPos, 0, batchSize)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		current := make([]define.ChunkPos, len(batch))
		copy(current, batch)
		batch = batch[:0]
		return fn(current)
	}

	for x := 0; x < xCount; x++ {
		for z := 0; z < zCount; z++ {
			batch = append(batch, define.ChunkPos{int32(x), int32(z)})
			if len(batch) == batchSize {
				if err := flush(); err != nil {
					return err
				}
			}
		}
	}
	return flush()
}

func ConvertReaderToMCWorld(
	reader Structure,
	bedrockWorld *world.BedrockWorld,
	startSubChunkPos bwo_define.SubChunkPos,
	startCallback func(int),
	progressCallback func(),
) error {
	return convertReaderToMCWorld(reader, bedrockWorld, startSubChunkPos, startCallback, progressCallback)
}
