package game_interface

import (
	"fmt"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol/packet"
	"github.com/LangTuStudio/RaaBel/game_control/resources_control"
)

type ResourcesWrapper struct {
	resources_control.BotInfo
	*resources_control.Resources
}

type GameInterface struct {
	wrapper               *ResourcesWrapper
	commands              *Commands
	structureBackup       *StructureBackup
	querytarget           *Querytarget
	movement              *Movement
	setblock              *SetBlock
	replaceitem           *Replaceitem
	botClick              *BotClick
	itemStackOperation    *ItemStackOperation
	containerOpenAndClose *ContainerOpenAndClose
	itemCopy              *ItemCopy
	itemTransition        *ItemTransition
	structureRequest      *StructureRequest
	subChunkRequest       *SubChunkRequest
	botAction             *BotAction
	specialCommandManager *SpecialCommandManager
	maintainer            Maintainer
}

func NewResourcesWrapper(resources *resources_control.Resources) *ResourcesWrapper {
	return &ResourcesWrapper{
		BotInfo:   resources.BotInfo(),
		Resources: resources,
	}
}

func NewGameInterface(resources *resources_control.Resources, maintainer ...Maintainer) *GameInterface {
	result := new(GameInterface)

	result.wrapper = NewResourcesWrapper(resources)
	result.commands = NewCommands(result.wrapper)
	result.structureBackup = NewStructureBackup(result.commands)
	result.querytarget = NewQuerytarget(result.commands)
	result.movement = NewMovement(result.wrapper, result.querytarget)
	result.setblock = NewSetBlock(result.commands)
	result.replaceitem = NewReplaceitem(result.commands)
	result.botClick = NewBotClick(result.wrapper, result.commands, result.setblock)
	result.itemStackOperation = NewItemStackOperation(result.wrapper)
	result.containerOpenAndClose = NewContainerOpenAndClose(result.wrapper, result.commands, result.botClick)
	result.itemCopy = NewItemCopy(result.containerOpenAndClose, result.commands, result.itemStackOperation, result.structureBackup)
	result.itemTransition = NewItemTransition(result.wrapper, result.itemStackOperation)
	result.structureRequest = NewStructureRequest(result.wrapper)
	result.subChunkRequest = NewSubChunkRequest(result.wrapper)
	result.botAction = NewBotAction(result)
	result.specialCommandManager = NewSpecialCommandManager()
	if len(maintainer) > 0 {
		result.maintainer = maintainer[0]
	}
	if err := result.initMaintainer(); err != nil {
		panic(fmt.Errorf("NewGameInterface: %v", err))
	}

	return result
}

func (g *GameInterface) initMaintainer() error {
	if g.maintainer == nil {
		return nil
	}
	packetID := make([]uint32, 0)
	for key := range g.maintainer.PacketToListen() {
		packetID = append(packetID, key)
	}
	_, err := g.PacketListener().ListenPacket(
		packetID,
		func(p packet.Packet, connCloseErr error) {
			if connCloseErr == nil {
				g.maintainer.HandlePacket(p, g)
			}
		},
	)
	if err != nil {
		return fmt.Errorf("initMaintainer: %v", err)
	}
	return nil
}

func (g *GameInterface) GetBotInfo() resources_control.BotInfo {
	return g.wrapper.BotInfo
}

func (g *GameInterface) PacketListener() *resources_control.PacketListener {
	return g.wrapper.PacketListener()
}

func (g *GameInterface) Resources() *resources_control.Resources {
	return g.wrapper.Resources
}

func (g *GameInterface) Commands() *Commands {
	return g.commands
}

func (g *GameInterface) StructureBackup() *StructureBackup {
	return g.structureBackup
}

func (g *GameInterface) Querytarget() *Querytarget {
	return g.querytarget
}

func (g *GameInterface) Movement() *Movement {
	return g.movement
}

func (g *GameInterface) SetBlock() *SetBlock {
	return g.setblock
}

func (g *GameInterface) Replaceitem() *Replaceitem {
	return g.replaceitem
}

func (g *GameInterface) BotClick() *BotClick {
	return g.botClick
}

func (g *GameInterface) ItemStackOperation() *ItemStackOperation {
	return g.itemStackOperation
}

func (g *GameInterface) ContainerOpenAndClose() *ContainerOpenAndClose {
	return g.containerOpenAndClose
}

func (g *GameInterface) ItemCopy() *ItemCopy {
	return g.itemCopy
}

func (g *GameInterface) ItemTransition() *ItemTransition {
	return g.itemTransition
}

func (g *GameInterface) StructureRequest() *StructureRequest {
	return g.structureRequest
}

func (g *GameInterface) SubChunkRequest() *SubChunkRequest {
	return g.subChunkRequest
}

func (g *GameInterface) BotAction() *BotAction {
	return g.botAction
}

func (g *GameInterface) SpecialCommandManager() *SpecialCommandManager {
	return g.specialCommandManager
}

func (g *GameInterface) Maintainer() Maintainer {
	return g.maintainer
}
