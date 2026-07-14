package item_stack_operation

import "github.com/LangTuStudio/RaaBel/core/minecraft/protocol"

// MakingRuntime 是在将各个已实现的物品操作
// 内联为标准的物品堆栈操作请求时的运行时数据
type MakingRuntime any

// MoveRuntime 是将物品移动操作内联为物品堆栈操作请求的运行时结构体
type MoveRuntime struct {
	MoveSrcContainer      protocol.FullContainerName
	MoveSrcStackNetworkID int32
	MoveDstContainer      protocol.FullContainerName
	MoveDstStackNetworkID int32
}

// SwapRuntime 是将物品交换操作内联为物品堆栈操作请求的运行时结构体
type SwapRuntime struct {
	SwapSrcContainer      protocol.FullContainerName
	SwapSrcStackNetworkID int32
	SwapDstContainer      protocol.FullContainerName
	SwapDstStackNetworkID int32
}

// DropRuntime 是将物品丢弃内联为物品堆栈操作请求的运行时结构体
type DropRuntime struct {
	DropSrcContainer      protocol.FullContainerName
	DropSrcStackNetworkID int32
	Randomly              bool
}

// DropRuntime 是将物品从快捷栏丢出操作内联为物品堆栈操作请求的运行时结构体
type DropHotbarRuntime struct {
	DropSrcStackNetworkID int32
	Randomly              bool
}

// CreativeItemRuntime 是将物品从创造物品栏获取操作内联为物品堆栈操作请求的运行时结构体
type CreativeItemRuntime struct {
	RequestID             int32
	DstContainer          protocol.FullContainerName
	DstItemStackID        int32
	CreativeItemNetworkID uint32
}

// RenamingRuntime 是将铁砧重命名操作内联为物品堆栈操作请求的运行时结构体
type RenamingRuntime struct {
	RequestID               int32
	ItemCount               uint8
	SrcContainerID          byte
	SrcStackNetworkID       int32
	AnvilSlotStackNetworkID int32
}

// CraftingRuntime 是将工作台操作内联为物品堆栈操作请求的运行时结构体
type CraftingRuntime struct {
	RequestID            int32
	Consumes             []CraftingConsume
	ResultStackNetworkID int32
}

// LoomingRuntime 是将织布机操作内联为物品堆栈操作请求的运行时结构体
type LoomingRuntime struct {
	RequestID int32

	LoomPatternStackNetworkID    int32
	MovePatternSrcContainerID    byte
	MovePatternSrcStackNetworkID int32

	LoomBannerStackNetworkID    int32
	MoveBannerSrcContainerID    byte
	MoveBannerSrcStackNetworkID int32

	LoomDyeStackNetworkID    int32
	MoveDyeSrcContainerID    byte
	MoveDyeSrcStackNetworkID int32
}

// TrimmingRuntime 是将锻造台纹饰操作内联为物品堆栈操作请求的运行时结构体
type TrimmingRuntime struct {
	RequestID       int32
	RecipeNetworkID uint32

	TrimItemStackNetworkID        int32
	MoveTrimItemSrcContainerID    byte
	MoveTrimItemSrcStackNetworkID int32

	MaterialStackNetworkID        int32
	MoveMaterialSrcContainerID    byte
	MoveMaterialSrcStackNetworkID int32

	TemplateStackNetworkID        int32
	MoveTemplateSrcContainerID    byte
	MoveTemplateSrcStackNetworkID int32
}

// BeaconPaymentRuntime 是将信标支付操作内联为物品堆栈操作请求的运行时结构体
type BeaconPaymentRuntime struct {
	RequestID                    int32
	BeaconInputStackNetworkID    int32
	MovePaymentSrcContainerID    byte
	MovePaymentSrcStackNetworkID int32
}
