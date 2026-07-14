package mapbuilder

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	coreConbit "github.com/LangTuStudio/Conbit/Conbit"
	"github.com/LangTuStudio/Conbit/Conbit/chunks/define"
	"github.com/LangTuStudio/Conbit/minecraft/protocol"
	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
	"github.com/LangTuStudio/Conbit/utils/structure/mc_structure"
	"github.com/google/uuid"

	GameInterface "nexus/utils/api/game_interface"
	resources_control "nexus/utils/api/resources_control"
	clientpkg "nexus/utils/client"
)

// nexusAPI 把 NexusEgo 的主机器人接入点适配为 MapAPI。
type nexusAPI struct {
	client *clientpkg.Client
}

// NewNexusAPI 基于已连接的主机器人 client 创建一个 MapAPI。
func NewNexusAPI(client *clientpkg.Client) MapAPI {
	return &nexusAPI{client: client}
}

func (n *nexusAPI) resources() *resources_control.Resources {
	if n.client == nil {
		return nil
	}
	if r, ok := n.client.Resources.(*resources_control.Resources); ok {
		return r
	}
	return nil
}

func (n *nexusAPI) omega() coreConbit.MicroOmega {
	if n == nil || n.client == nil || n.client.Conn == nil {
		return nil
	}
	provider, ok := n.client.Conn.(interface {
		Omega() coreConbit.MicroOmega
	})
	if !ok {
		return nil
	}
	return provider.Omega()
}

// LockMap 锁定地图。
func (n *nexusAPI) LockMap(mapID int64) error {
	if n.client == nil || n.client.Conn == nil {
		return fmt.Errorf("LockMap: connection not ready")
	}
	return n.client.Conn.WritePacket(&packet.MapCreateLockedCopy{
		OriginalMapID: mapID,
		NewMapID:      mapID,
	})
}

// SendMapPixels 发送地图像素更新。
func (n *nexusAPI) SendMapPixels(mapID int64, pixels []PixelRequest) error {
	if n.client == nil || n.client.Conn == nil {
		return fmt.Errorf("SendMapPixels: connection not ready")
	}
	converted := make([]protocol.PixelRequest, 0, len(pixels))
	for _, p := range pixels {
		converted = append(converted, protocol.PixelRequest{
			Colour: p.Colour,
			Index:  p.Index,
		})
	}
	return n.client.Conn.WritePacket(&packet.MapInfoRequest{
		MapID:        mapID,
		ClientPixels: converted,
	})
}

// GetSubChunksInArea 获取指定区域的子区块数据。
func (n *nexusAPI) GetSubChunksInArea(dimension int32, start, end BlockPos) (*SubChunkResponse, error) {
	if n.client == nil || n.client.Conn == nil {
		return nil, fmt.Errorf("GetSubChunksInArea: connection not ready")
	}
	res := n.resources()
	if res == nil {
		return nil, fmt.Errorf("GetSubChunksInArea: resources not ready")
	}

	minX, maxX := minInt32(start[0], end[0]), maxInt32(start[0], end[0])
	minY, maxY := minInt32(start[1], end[1]), maxInt32(start[1], end[1])
	minZ, maxZ := minInt32(start[2], end[2]), maxInt32(start[2], end[2])

	n.ensureAreaLoaded(minX, minY, minZ, maxX, maxY, maxZ)

	chunkXStart := minX >> 4
	chunkXEnd := maxX >> 4
	chunkZStart := minZ >> 4
	chunkZEnd := maxZ >> 4
	subYStart := minY >> 4
	subYEnd := maxY >> 4

	centerCX := (chunkXStart + chunkXEnd) / 2
	centerCY := (subYStart + subYEnd) / 2
	centerCZ := (chunkZStart + chunkZEnd) / 2

	basePos := protocol.SubChunkPos{centerCX, centerCY, centerCZ}

	offsets := make([]protocol.SubChunkOffset, 0, 64)
	for cx := chunkXStart; cx <= chunkXEnd; cx++ {
		for cz := chunkZStart; cz <= chunkZEnd; cz++ {
			for cy := subYStart; cy <= subYEnd; cy++ {
				dx := int32(cx - centerCX)
				dy := int32(cy - centerCY)
				dz := int32(cz - centerCZ)
				if dx < -127 || dx > 127 || dy < -127 || dy > 127 || dz < -127 || dz > 127 {
					return nil, fmt.Errorf("GetSubChunksInArea: area too large for one batch")
				}
				offsets = append(offsets, protocol.SubChunkOffset{int8(dx), int8(dy), int8(dz)})
			}
		}
	}
	if len(offsets) == 0 {
		return &SubChunkResponse{Position: SubChunkPos{centerCX, centerCY, centerCZ}}, nil
	}
	if len(offsets) > 256 {
		return nil, fmt.Errorf("GetSubChunksInArea: too many subchunks (%d) in one request", len(offsets))
	}

	request := &packet.SubChunkRequest{
		Dimension: dimension,
		Position:  basePos,
		Offsets:   offsets,
	}

	listenerID, packets := res.Listener.CreateNewListen([]uint32{packet.IDSubChunk}, 8)
	defer res.Listener.StopAndDestroy(listenerID)

	if err := n.client.Conn.WritePacket(request); err != nil {
		return nil, err
	}

	timer := time.NewTimer(8 * time.Second)
	defer timer.Stop()

	for {
		select {
		case pk := <-packets:
			sub, ok := pk.(*packet.SubChunk)
			if !ok {
				continue
			}
			if sub.Dimension != request.Dimension {
				continue
			}
			if sub.Position != request.Position {
				continue
			}
			result := &SubChunkResponse{
				Position:        SubChunkPos{sub.Position.X(), sub.Position.Y(), sub.Position.Z()},
				SubChunkEntries: make([]SubChunkEntry, 0, len(sub.SubChunkEntries)),
			}
			for _, e := range sub.SubChunkEntries {
				result.SubChunkEntries = append(result.SubChunkEntries, SubChunkEntry{
					Offset:     SubChunkOffset{e.Offset[0], e.Offset[1], e.Offset[2]},
					Result:     e.Result,
					RawPayload: append([]byte(nil), e.RawPayload...),
				})
			}
			return result, nil
		case <-timer.C:
			return nil, fmt.Errorf("GetSubChunksInArea: timeout")
		}
	}
}

// Dimension 获取机器人当前维度。
func (n *nexusAPI) Dimension() (int32, bool) {
	if n.client == nil {
		return 0, false
	}
	return n.client.DimensionID, true
}

// ConnContext 返回底层连接上下文，用于判断是否断线。
func (n *nexusAPI) ConnContext() context.Context {
	return context.Background()
}

// RequestStructureNBTs 通过 NexusEgo 自带的 structure save / 结构包请求拿完整方块实体 NBT。
// 这条路径经过 NexusEgo 的 ResourcesControl，避免了"未占用结构资源"的 panic。
func (n *nexusAPI) RequestStructureNBTs(start, end BlockPos) (map[[3]int32]map[string]interface{}, error) {
	if n.client == nil || n.client.GameInterface == nil {
		return nil, fmt.Errorf("RequestStructureNBTs: client not ready")
	}
	res := n.resources()
	if res == nil {
		return nil, fmt.Errorf("RequestStructureNBTs: resources not ready")
	}

	gi, ok := n.client.GameInterface.(*GameInterface.GameInterface)
	if !ok || gi == nil {
		return nil, fmt.Errorf("RequestStructureNBTs: GameInterface unavailable")
	}

	minX, maxX := minInt32(start[0], end[0]), maxInt32(start[0], end[0])
	minY, maxY := minInt32(start[1], end[1]), maxInt32(start[1], end[1])
	minZ, maxZ := minInt32(start[2], end[2]), maxInt32(start[2], end[2])
	if err := gi.SendAICommand("gamemode 1", true); err != nil {
		return nil, fmt.Errorf("RequestStructureNBTs prepare gamemode: %v", err)
	}
	n.ensureAreaLoaded(minX, minY, minZ, maxX, maxY, maxZ)

	// 第一步：先用 structure save 把区域备份成结构（必须，否则 export request 拿不到数据）
	uniqueID, err := gi.BackupStructure(GameInterface.MCStructure{
		BeginX: int(minX), BeginY: int(minY), BeginZ: int(minZ),
		SizeX: int(maxX - minX + 1),
		SizeY: int(maxY - minY + 1),
		SizeZ: int(maxZ - minZ + 1),
	})
	if err != nil {
		return nil, fmt.Errorf("RequestStructureNBTs save: %v", err)
	}
	defer gi.DeleteStructure(uniqueID)
	if err := gi.AwaitChangesGeneral(); err != nil {
		return nil, fmt.Errorf("RequestStructureNBTs wait save: %v", err)
	}
	structureName := uuidToSafeString(uniqueID)
	structureNames := uniqueStrings(structureName, uuidToFilteredString(uniqueID))

	// 第二步：占用结构资源 + 发请求 + 拿响应
	holder := res.Structure.Occupy()
	defer res.Structure.Release(holder)

	requestErrs := make([]error, 0, len(structureNames))
	for _, name := range structureNames {
		resp, err := gi.SendStructureRequestWithResponse(n.newStructureTemplateDataRequest(name, minX, minY, minZ, maxX, maxY, maxZ))
		if err != nil {
			requestErrs = append(requestErrs, fmt.Errorf("name=%q: %v", name, err))
			continue
		}
		if !resp.Success {
			requestErrs = append(requestErrs, fmt.Errorf("name=%q: server returned fail", name))
			continue
		}
		return decodeStructureNBTs(resp.StructureTemplate)
	}

	ConbitErrs := make([]error, 0, len(structureNames))
	for _, name := range structureNames {
		blockEntities, err := n.requestStructureNBTsViaConbit(name, minX, minY, minZ, maxX, maxY, maxZ)
		if err == nil && len(blockEntities) > 0 {
			return blockEntities, nil
		}
		if err != nil {
			ConbitErrs = append(ConbitErrs, fmt.Errorf("name=%q: %v", name, err))
		} else {
			ConbitErrs = append(ConbitErrs, fmt.Errorf("name=%q: returned 0 NBTs", name))
		}
	}
	return nil, fmt.Errorf("RequestStructureNBTs request: %v; Conbit request: %v", joinErrors(requestErrs), joinErrors(ConbitErrs))
}

func (n *nexusAPI) newStructureTemplateDataRequest(structureName string, minX, minY, minZ, maxX, maxY, maxZ int32) *packet.StructureTemplateDataRequest {
	return &packet.StructureTemplateDataRequest{
		StructureName: structureName,
		Position:      protocol.BlockPos{minX, minY, minZ},
		Settings: protocol.StructureSettings{
			PaletteName:               "default",
			IgnoreEntities:            true,
			IgnoreBlocks:              false,
			Size:                      protocol.BlockPos{maxX - minX + 1, maxY - minY + 1, maxZ - minZ + 1},
			Offset:                    protocol.BlockPos{0, 0, 0},
			LastEditingPlayerUniqueID: 0,
			Rotation:                  0,
			Mirror:                    0,
			Integrity:                 100,
			Seed:                      0,
			AllowNonTickingChunks:     false,
		},
		RequestType: packet.StructureTemplateRequestExportFromSave,
	}
}

func (n *nexusAPI) ensureAreaLoaded(minX, minY, minZ, maxX, maxY, maxZ int32) {
	if n == nil || n.client == nil || n.client.GameInterface == nil {
		return
	}
	centerX := (minX + maxX) / 2
	centerY := (minY + maxY) / 2
	centerZ := (minZ + maxZ) / 2
	_ = n.client.GameInterface.SendWSCommand(fmt.Sprintf("tp @s %d %d %d", centerX, centerY+2, centerZ))
	time.Sleep(2 * time.Second)
}

func decodeStructureNBTs(template map[string]interface{}) (map[[3]int32]map[string]interface{}, error) {
	structure := &mc_structure.StructureContent{}
	if err := structure.FromNBT(template); err != nil {
		return nil, fmt.Errorf("RequestStructureNBTs decode FromNBT: %v", err)
	}
	decoded := structure.Decode()

	out := make(map[[3]int32]map[string]interface{})
	for cubePos, nbt := range decoded.NBTsInAbsolutePos() {
		key := [3]int32{int32(cubePos.X()), int32(cubePos.Y()), int32(cubePos.Z())}
		out[key] = nbt
	}
	return out, nil
}

func (n *nexusAPI) requestStructureNBTsViaConbit(structureName string, minX, minY, minZ, maxX, maxY, maxZ int32) (map[[3]int32]map[string]interface{}, error) {
	omega := n.omega()
	if omega == nil || omega.GetLowLevelAreaRequester() == nil {
		return nil, fmt.Errorf("Conbit area requester unavailable")
	}
	if structureName == "" {
		return nil, fmt.Errorf("Conbit structure request: empty structure name")
	}
	sizeX, sizeY, sizeZ := maxX-minX+1, maxY-minY+1, maxZ-minZ+1
	if sizeX <= 0 || sizeY <= 0 || sizeZ <= 0 {
		return nil, fmt.Errorf("invalid structure size")
	}

	resp, err := omega.GetLowLevelAreaRequester().
		LowLevelRequestStructure(
			define.CubePos{int(minX), int(minY), int(minZ)},
			define.CubePos{int(sizeX), int(sizeY), int(sizeZ)},
			structureName,
		).
		SetTimeout(8 * time.Second).
		BlockGetResult()
	if err != nil {
		return nil, fmt.Errorf("Conbit structure request: %v", err)
	}
	if resp == nil {
		return nil, fmt.Errorf("Conbit structure request: empty response")
	}
	decoded, err := resp.Decode()
	if err != nil {
		return nil, fmt.Errorf("Conbit structure decode: %v", err)
	}

	out := make(map[[3]int32]map[string]interface{})
	for cubePos, nbt := range decoded.NBTsInAbsolutePos() {
		key := [3]int32{int32(cubePos.X()), int32(cubePos.Y()), int32(cubePos.Z())}
		out[key] = nbt
	}
	return out, nil
}

// uuidToSafeString 把 uuid 转成 NexusEgo 备份结构使用的安全字符串（与 GameInterface 内部一致）。
func uuidToSafeString(id uuid.UUID) string {
	s := id.String()
	return strings.ReplaceAll(s, "-", "")
}

func uuidToFilteredString(id uuid.UUID) string {
	s := id.String()
	for key, value := range GameInterface.StringUUIDReplaceMap {
		s = strings.ReplaceAll(s, key, value)
	}
	return s
}

func uniqueStrings(values ...string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func joinErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	parts := make([]string, 0, len(errs))
	for _, err := range errs {
		if err == nil {
			continue
		}
		parts = append(parts, err.Error())
	}
	return errors.New(strings.Join(parts, "; "))
}

func minInt32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

func maxInt32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

