package NBTAssigner

import (
	"fmt"
	GameInterface "nexus/utils/api/game_interface"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/LangTuStudio/Conbit/minecraft/protocol"
	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
)

func (s *Structure_block) Decode() error {
	return nil
}

func (s *Structure_block) WriteData() error {
	gameInterface := s.BlockEntity.Interface.(*GameInterface.GameInterface)
	// 放置结构体方块
	err := gameInterface.SetBlock(s.BlockEntity.AdditionalData.Position, s.BlockEntity.Block.Name, s.BlockEntity.AdditionalData.BlockStates)
	if err != nil {
		return fmt.Errorf("WriteData: %v", err)
	}
	// 检查结构体信息
	if _, ok := s.BlockEntity.Block.NBT["structureName"]; !ok {
		return nil
	}
	structureName := s.BlockEntity.Block.NBT["structureName"].(string)
	if _, ok := s.BlockEntity.Block.NBT["dataField"]; !ok {
		return nil
	}
	dataField := s.BlockEntity.Block.NBT["dataField"].(string)
	if _, ok := s.BlockEntity.Block.NBT["includePlayers"]; !ok {
		return nil
	}
	includePlayers := s.BlockEntity.Block.NBT["includePlayers"].(int)
	var bool_includePlayers bool
	if includePlayers == 1 {
		bool_includePlayers = true
	} else {
		bool_includePlayers = false
	}

	if _, ok := s.BlockEntity.Block.NBT["showBoundingBox"]; !ok {
		return nil
	}
	showBoundingBox := s.BlockEntity.Block.NBT["showBoundingBox"].(int)
	var bool_showBoundingBox bool
	if showBoundingBox == 1 {
		bool_showBoundingBox = true
	} else {
		bool_showBoundingBox = false
	}

	if _, ok := s.BlockEntity.Block.NBT["data"]; !ok {
		return nil
	}
	StructureBlockType := s.BlockEntity.Block.NBT["data"].(int)

	if _, ok := s.BlockEntity.Block.NBT["redstoneSaveMode"]; !ok {
		return nil
	}
	RedstoneSaveMode := s.BlockEntity.Block.NBT["redstoneSaveMode"].(int)

	if _, ok := s.BlockEntity.Block.NBT["showBoundingBox"]; !ok {
		return nil
	}

	// 解析结构体的设置
	// pass

	// 写入结构体信息
	err = gameInterface.WritePacket(&packet.StructureBlockUpdate{
		Position:           protocol.BlockPos(s.BlockEntity.AdditionalData.Position),
		StructureName:      structureName,
		DataField:          dataField,
		IncludePlayers:     bool_includePlayers,
		ShowBoundingBox:    bool_showBoundingBox,
		StructureBlockType: int32(StructureBlockType),
		Settings: protocol.StructureSettings{
			PaletteName: s.BlockEntity.Block.NBT["structureName"].(string),
			IgnoreEntities: func() bool {
				if s.BlockEntity.Block.NBT["ignoreEntities"].(int) == 1 {
					return true
				} else {
					return false
				}
			}(),
			IgnoreBlocks: func() bool {
				if s.BlockEntity.Block.NBT["removeBlocks"].(int) == 1 {
					return true
				} else {
					return false
				}
			}(),
			Size: protocol.BlockPos{
				int32(s.BlockEntity.Block.NBT["xStructureSize"].(int)),
				int32(s.BlockEntity.Block.NBT["yStructureSize"].(int)),
				int32(s.BlockEntity.Block.NBT["zStructureSize"].(int)),
			},
			Offset: protocol.BlockPos{
				int32(s.BlockEntity.Block.NBT["xStructureOffset"].(int)),
				int32(s.BlockEntity.Block.NBT["yStructureOffset"].(int)),
				int32(s.BlockEntity.Block.NBT["zStructureOffset"].(int)),
			},
			LastEditingPlayerUniqueID: gameInterface.ClientInfo.EntityUniqueID,
			Rotation:                  byte(s.BlockEntity.Block.NBT["rotation"].(int)),
			Mirror:                    byte(s.BlockEntity.Block.NBT["mirror"].(int)),
			Seed:                      uint32(s.BlockEntity.Block.NBT["seed"].(int)),
		},
		RedstoneSaveMode: int32(RedstoneSaveMode),
		ShouldTrigger:    true,
		Waterlogged:      true,
	})
	if err != nil {
		return fmt.Errorf("WriteData: %v", err)
	}
	return nil
}

func (s *StructureBlock) Decode() error {
	var normal bool
	var animationMode byte
	var animationSeconds float32
	var data int32
	var dataField string
	var ignoreEntities bool
	var includePlayers bool
	var integrity float32
	var mirror byte
	var redstoneSaveMode int32
	var removeBlocks bool
	var rotation byte
	var seed int64
	var showBoundingBox bool
	var structureName string
	var xStructureOffset int32
	var xStructureSize int32
	var yStructureOffset int32
	var yStructureSize int32
	var zStructureOffset int32
	var zStructureSize int32

	if value, ok := s.BlockEntity.Block.NBT["animationMode"]; ok {
		animationMode, normal = value.(byte)
		if !normal {
			return fmt.Errorf("Decode: Crashed at s.BlockEntity.Block.NBT[\"animationMode\"]; s.BlockEntity.Block.NBT = %#v", s.BlockEntity.Block.NBT)
		}
	}
	if value, ok := s.BlockEntity.Block.NBT["animationSeconds"]; ok {
		animationSeconds, normal = value.(float32)
		if !normal {
			return fmt.Errorf("Decode: Crashed at s.BlockEntity.Block.NBT[\"animationSeconds\"]; s.BlockEntity.Block.NBT = %#v", s.BlockEntity.Block.NBT)
		}
	}
	if value, ok := s.BlockEntity.Block.NBT["data"]; ok {
		data, normal = value.(int32)
		if !normal {
			return fmt.Errorf("Decode: Crashed at s.BlockEntity.Block.NBT[\"data\"]; s.BlockEntity.Block.NBT = %#v", s.BlockEntity.Block.NBT)
		}
	}
	if value, ok := s.BlockEntity.Block.NBT["dataField"]; ok {
		dataField, normal = value.(string)
		if !normal {
			return fmt.Errorf("Decode: Crashed at s.BlockEntity.Block.NBT[\"dataField\"]; s.BlockEntity.Block.NBT = %#v", s.BlockEntity.Block.NBT)
		}
	}
	if value, ok := s.BlockEntity.Block.NBT["ignoreEntities"]; ok {
		got, ok := value.(byte)
		if !ok {
			return fmt.Errorf("Decode: Crashed at s.BlockEntity.Block.NBT[\"ignoreEntities\"]; s.BlockEntity.Block.NBT = %#v", s.BlockEntity.Block.NBT)
		}
		ignoreEntities = got != 0
	}
	if value, ok := s.BlockEntity.Block.NBT["includePlayers"]; ok {
		got, ok := value.(byte)
		if !ok {
			return fmt.Errorf("Decode: Crashed at s.BlockEntity.Block.NBT[\"includePlayers\"]; s.BlockEntity.Block.NBT = %#v", s.BlockEntity.Block.NBT)
		}
		includePlayers = got != 0
	}
	if value, ok := s.BlockEntity.Block.NBT["integrity"]; ok {
		integrity, normal = value.(float32)
		if !normal {
			return fmt.Errorf("Decode: Crashed at s.BlockEntity.Block.NBT[\"integrity\"]; s.BlockEntity.Block.NBT = %#v", s.BlockEntity.Block.NBT)
		}
	}
	if value, ok := s.BlockEntity.Block.NBT["mirror"]; ok {
		mirror, normal = value.(byte)
		if !normal {
			return fmt.Errorf("Decode: Crashed at s.BlockEntity.Block.NBT[\"mirror\"]; s.BlockEntity.Block.NBT = %#v", s.BlockEntity.Block.NBT)
		}
	}
	if value, ok := s.BlockEntity.Block.NBT["redstoneSaveMode"]; ok {
		redstoneSaveMode, normal = value.(int32)
		if !normal {
			return fmt.Errorf("Decode: Crashed at s.BlockEntity.Block.NBT[\"redstoneSaveMode\"]; s.BlockEntity.Block.NBT = %#v", s.BlockEntity.Block.NBT)
		}
	}
	if value, ok := s.BlockEntity.Block.NBT["removeBlocks"]; ok {
		got, ok := value.(byte)
		if !ok {
			return fmt.Errorf("Decode: Crashed at s.BlockEntity.Block.NBT[\"removeBlocks\"]; s.BlockEntity.Block.NBT = %#v", s.BlockEntity.Block.NBT)
		}
		removeBlocks = got != 0
	}
	if value, ok := s.BlockEntity.Block.NBT["rotation"]; ok {
		rotation, normal = value.(byte)
		if !normal {
			return fmt.Errorf("Decode: Crashed at s.BlockEntity.Block.NBT[\"rotation\"]; s.BlockEntity.Block.NBT = %#v", s.BlockEntity.Block.NBT)
		}
	}
	if value, ok := s.BlockEntity.Block.NBT["seed"]; ok {
		seed, normal = value.(int64)
		if !normal {
			return fmt.Errorf("Decode: Crashed at s.BlockEntity.Block.NBT[\"seed\"]; s.BlockEntity.Block.NBT = %#v", s.BlockEntity.Block.NBT)
		}
	}
	if value, ok := s.BlockEntity.Block.NBT["showBoundingBox"]; ok {
		got, ok := value.(byte)
		if !ok {
			return fmt.Errorf("Decode: Crashed at s.BlockEntity.Block.NBT[\"showBoundingBox\"]; s.BlockEntity.Block.NBT = %#v", s.BlockEntity.Block.NBT)
		}
		showBoundingBox = got != 0
	}
	if value, ok := s.BlockEntity.Block.NBT["structureName"]; ok {
		structureName, normal = value.(string)
		if !normal {
			return fmt.Errorf("Decode: Crashed at s.BlockEntity.Block.NBT[\"structureName\"]; s.BlockEntity.Block.NBT = %#v", s.BlockEntity.Block.NBT)
		}
	}
	if value, ok := s.BlockEntity.Block.NBT["xStructureOffset"]; ok {
		xStructureOffset, normal = value.(int32)
		if !normal {
			return fmt.Errorf("Decode: Crashed at s.BlockEntity.Block.NBT[\"xStructureOffset\"]; s.BlockEntity.Block.NBT = %#v", s.BlockEntity.Block.NBT)
		}
	}
	if value, ok := s.BlockEntity.Block.NBT["xStructureSize"]; ok {
		xStructureSize, normal = value.(int32)
		if !normal {
			return fmt.Errorf("Decode: Crashed at s.BlockEntity.Block.NBT[\"xStructureSize\"]; s.BlockEntity.Block.NBT = %#v", s.BlockEntity.Block.NBT)
		}
	}
	if value, ok := s.BlockEntity.Block.NBT["yStructureOffset"]; ok {
		yStructureOffset, normal = value.(int32)
		if !normal {
			return fmt.Errorf("Decode: Crashed at s.BlockEntity.Block.NBT[\"yStructureOffset\"]; s.BlockEntity.Block.NBT = %#v", s.BlockEntity.Block.NBT)
		}
	}
	if value, ok := s.BlockEntity.Block.NBT["yStructureSize"]; ok {
		yStructureSize, normal = value.(int32)
		if !normal {
			return fmt.Errorf("Decode: Crashed at s.BlockEntity.Block.NBT[\"yStructureSize\"]; s.BlockEntity.Block.NBT = %#v", s.BlockEntity.Block.NBT)
		}
	}
	if value, ok := s.BlockEntity.Block.NBT["zStructureOffset"]; ok {
		zStructureOffset, normal = value.(int32)
		if !normal {
			return fmt.Errorf("Decode: Crashed at s.BlockEntity.Block.NBT[\"zStructureOffset\"]; s.BlockEntity.Block.NBT = %#v", s.BlockEntity.Block.NBT)
		}
	}
	if value, ok := s.BlockEntity.Block.NBT["zStructureSize"]; ok {
		zStructureSize, normal = value.(int32)
		if !normal {
			return fmt.Errorf("Decode: Crashed at s.BlockEntity.Block.NBT[\"zStructureSize\"]; s.BlockEntity.Block.NBT = %#v", s.BlockEntity.Block.NBT)
		}
	}

	s.StructureBlockData = StructureBlockData{
		AnimationMode:    animationMode,
		AnimationSeconds: animationSeconds,
		Data:             data,
		DataField:        dataField,
		IgnoreEntities:   ignoreEntities,
		IncludePlayers:   includePlayers,
		Integrity:        integrity,
		Mirror:           mirror,
		RedstoneSaveMode: redstoneSaveMode,
		RemoveBlocks:     removeBlocks,
		Rotation:         rotation,
		Seed:             seed,
		ShowBoundingBox:  showBoundingBox,
		StructureName:    structureName,
		XStructureOffset: xStructureOffset,
		XStructureSize:   xStructureSize,
		YStructureOffset: yStructureOffset,
		YStructureSize:   yStructureSize,
		ZStructureOffset: zStructureOffset,
		ZStructureSize:   zStructureSize,
	}
	return nil
}

func (s *StructureBlock) WriteData() error {
	api := s.BlockEntity.Interface.(*GameInterface.GameInterface)
	if s.BlockEntity.AdditionalData.FastMode {
		if err := api.SetBlockAsync(s.BlockEntity.AdditionalData.Position, s.BlockEntity.Block.Name, s.BlockEntity.AdditionalData.BlockStates); err != nil {
			return fmt.Errorf("WriteData: %v", err)
		}
	} else {
		if err := s.BlockEntity.Interface.SetBlock(s.BlockEntity.AdditionalData.Position, s.BlockEntity.Block.Name, s.BlockEntity.AdditionalData.BlockStates); err != nil {
			return fmt.Errorf("WriteData: %v", err)
		}
	}

	api.WritePacket(&packet.StructureBlockUpdate{
		Position:           s.BlockEntity.AdditionalData.Position,
		StructureName:      s.StructureBlockData.StructureName,
		DataField:          s.StructureBlockData.DataField,
		IncludePlayers:     s.StructureBlockData.IncludePlayers,
		ShowBoundingBox:    s.StructureBlockData.ShowBoundingBox,
		StructureBlockType: s.StructureBlockData.Data,
		Settings: protocol.StructureSettings{
			PaletteName:           "default",
			IgnoreEntities:        s.StructureBlockData.IgnoreEntities,
			IgnoreBlocks:          s.StructureBlockData.RemoveBlocks,
			AllowNonTickingChunks: true,
			Size: [3]int32{
				s.StructureBlockData.XStructureSize,
				s.StructureBlockData.YStructureSize,
				s.StructureBlockData.ZStructureSize,
			},
			Offset: [3]int32{
				s.StructureBlockData.XStructureOffset,
				s.StructureBlockData.YStructureOffset,
				s.StructureBlockData.ZStructureOffset,
			},
			LastEditingPlayerUniqueID: api.ClientInfo.EntityUniqueID,
			Rotation:                  s.StructureBlockData.Rotation,
			Mirror:                    s.StructureBlockData.Mirror,
			AnimationMode:             s.StructureBlockData.AnimationMode,
			AnimationDuration:         s.StructureBlockData.AnimationSeconds,
			Integrity:                 s.StructureBlockData.Integrity,
			Seed:                      uint32(s.StructureBlockData.Seed),
			Pivot: mgl32.Vec3{
				(float32(s.StructureBlockData.XStructureSize) - 1) / 2,
				(float32(s.StructureBlockData.YStructureSize) - 1) / 2,
				(float32(s.StructureBlockData.ZStructureSize) - 1) / 2,
			},
		},
		RedstoneSaveMode: s.StructureBlockData.RedstoneSaveMode,
		ShouldTrigger:    false,
		Waterlogged:      false,
	})
	return nil
}

