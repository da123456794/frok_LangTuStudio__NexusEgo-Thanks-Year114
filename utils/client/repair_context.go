package client

import (
	"nexus/defines"
	"path/filepath"
	"sync"
)

type RepairBounds struct {
	MinX int
	MaxX int
	MinY int
	MaxY int
	MinZ int
	MaxZ int
}

// RepairContext 持有修补模式所需的元数据与运行状态。
type RepairContext struct {
	Enabled            bool
	FilePath           string
	Origin             types.Position
	OuterOrigin        types.Position
	BuildSize          [3]int
	Bounds             RepairBounds
	RegionSize         int
	UseFill            bool
	AutoPlaceDenyBlock bool
	AutoPlaceBorder    bool
	// 仅当进入修补模式后才允许监听聊天指令
	ChatEnabled      bool
	SettingsSnapshot Cdump_Setting
	// 命令方块独立于 No_NBT 的导入开关
	ImportCommandBlock bool
	CommandDataSpeed   int
	RepairQueue        chan RepairJob

	mu            sync.RWMutex
	repairing     bool
	pendingChunks map[[2]int]struct{}
	workerOnce    sync.Once
}

// RepairJob 表示一次修补请求
type RepairJob struct {
	ChunkX              int
	ChunkZ              int
	Chunks              [][2]int
	PlayerName          string
	PlayerPos           types.Position
	ClearBeforeReimport bool
}

// Setup 使用最新的导入信息初始化修补模式。
// filePath 会被转换为绝对路径；buildSize 中的值小于等于 0 时视为单点范围。
func (r *RepairContext) Setup(filePath string, origin types.Position, buildSize [3]int, regionSize int, useFill bool, settings *Cdump_Setting, outerOrigin types.Position, autoPlaceDenyBlock, autoPlaceBorder bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}
	r.FilePath = absPath
	r.Origin = origin
	r.OuterOrigin = outerOrigin
	r.BuildSize = buildSize
	r.RegionSize = regionSize
	r.UseFill = useFill
	r.AutoPlaceDenyBlock = autoPlaceDenyBlock
	r.AutoPlaceBorder = autoPlaceBorder
	if settings != nil {
		r.SettingsSnapshot = *settings
	} else {
		r.SettingsSnapshot = Cdump_Setting{}
	}

	minX, maxX := origin.X, origin.X
	minY, maxY := origin.Y, origin.Y
	minZ, maxZ := origin.Z, origin.Z

	if buildSize[0] > 0 {
		maxX = origin.X + buildSize[0] - 1
	}
	if buildSize[1] > 0 {
		maxY = origin.Y + buildSize[1] - 1
	}
	if buildSize[2] > 0 {
		maxZ = origin.Z + buildSize[2] - 1
	}
	if autoPlaceBorder {
		minX = outerOrigin.X
		minZ = outerOrigin.Z
		if buildSize[0] > 0 {
			maxX = outerOrigin.X + buildSize[0] + 1
		} else {
			maxX = outerOrigin.X
		}
		if buildSize[2] > 0 {
			maxZ = outerOrigin.Z + buildSize[2] + 1
		} else {
			maxZ = outerOrigin.Z
		}
	}
	if autoPlaceDenyBlock {
		minY = outerOrigin.Y
		if buildSize[1] > 0 {
			maxY = outerOrigin.Y + buildSize[1]
		} else {
			maxY = outerOrigin.Y
		}
	}

	r.Bounds = RepairBounds{
		MinX: minX, MaxX: maxX,
		MinY: minY, MaxY: maxY,
		MinZ: minZ, MaxZ: maxZ,
	}
	r.Enabled = true
	r.repairing = false
	r.ChatEnabled = false
	r.pendingChunks = make(map[[2]int]struct{})
	if r.RepairQueue == nil {
		r.RepairQueue = make(chan RepairJob, 64)
	}
}

// InBounds 判断给定坐标是否位于导入范围内。
func (r *RepairContext) InBounds(x, y, z int) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if !r.Enabled {
		return false
	}
	return x >= r.Bounds.MinX && x <= r.Bounds.MaxX &&
		y >= r.Bounds.MinY && y <= r.Bounds.MaxY &&
		z >= r.Bounds.MinZ && z <= r.Bounds.MaxZ
}

// TryLockRepair 标记修补流程正在运行，若已有修补在执行则返回 false。
func (r *RepairContext) TryLockRepair() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.repairing || !r.Enabled {
		return false
	}
	r.repairing = true
	return true
}

// UnlockRepair 结束修补占用。
func (r *RepairContext) UnlockRepair() {
	r.mu.Lock()
	r.repairing = false
	r.mu.Unlock()
}

func (r *RepairContext) TryBeginRepairJob(chunks [][2]int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.Enabled {
		return false
	}
	for _, chunk := range chunks {
		if _, exists := r.pendingChunks[chunk]; exists {
			return false
		}
	}
	for _, chunk := range chunks {
		r.pendingChunks[chunk] = struct{}{}
	}
	return true
}

func (r *RepairContext) FinishRepairJob(chunks [][2]int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, chunk := range chunks {
		delete(r.pendingChunks, chunk)
	}
}

// EnsureRepairQueue 返回修补队列并保证其已初始化
func (r *RepairContext) EnsureRepairQueue() chan RepairJob {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.RepairQueue == nil {
		r.RepairQueue = make(chan RepairJob, 64)
	}
	return r.RepairQueue
}

// StartRepairWorkerOnce 启动修补队列处理协程（仅一次）
func (r *RepairContext) StartRepairWorkerOnce(worker func(<-chan RepairJob)) {
	r.workerOnce.Do(func() {
		go worker(r.EnsureRepairQueue())
	})
}
