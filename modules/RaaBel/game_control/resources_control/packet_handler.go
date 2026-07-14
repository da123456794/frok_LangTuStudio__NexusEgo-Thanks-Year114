package resources_control

import (
	"fmt"
	"time"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol/packet"
	"github.com/LangTuStudio/RaaBel/core/py_rpc"

	"github.com/pterm/pterm"
)

// Block interaction actions were require bot to send
// packet.PlayerAuthInput since nemc 1.20.50.
//
// However, for command teleport, we should handle
// this by send packet.PlayerAuthInput with flag
// packet.InputFlagHandledTeleport, or the server will
// ignore the packet.PlayerAuthInput after bot was
// teleported.
func (r *Resources) handleMovePlayer(p *packet.MovePlayer) {
	if p.EntityRuntimeID == r.BotInfo().EntityRuntimeID && p.Mode == packet.MoveModeTeleport {
		inputData := protocol.NewBitset(packet.PlayerAuthInputBitsetSize)
		inputData.Set(packet.InputFlagHandledTeleport)
		_ = r.WritePacket(&packet.PlayerAuthInput{
			InputData: inputData,
			Position:  p.Position,
		})
	}
}

// respawn process
func (r *Resources) handleRespawn(p *packet.Respawn) {
	entityRuntimeID := r.BotInfo().EntityRuntimeID
	_ = r.WritePacket(&packet.Respawn{
		Position:        p.Position,
		State:           packet.RespawnStateClientReadyToSpawn,
		EntityRuntimeID: entityRuntimeID,
	})
	_ = r.WritePacket(&packet.PlayerAction{
		EntityRuntimeID: entityRuntimeID,
		ActionType:      protocol.PlayerActionRespawn,
	})
	for range 5 {
		inputData := protocol.NewBitset(packet.PlayerAuthInputBitsetSize)
		inputData.Set(packet.InputFlagStartFlying)
		_ = r.WritePacket(&packet.PlayerAuthInput{
			InputData: inputData,
			Position:  p.Position,
		})
		time.Sleep(time.Second / 20 * 3)
	}
	/*
		if p.State == packet.RespawnStateSearchingForSpawn {
			entityRuntimeID := r.BotInfo().EntityRuntimeID
			_ = r.WritePacket(&packet.Respawn{
				State:           packet.RespawnStateClientReadyToSpawn,
				EntityRuntimeID: entityRuntimeID,
			})
			_ = r.WritePacket(&packet.PlayerAction{
				EntityRuntimeID: entityRuntimeID,
				ActionType:      protocol.PlayerActionRespawn,
				BlockFace:       -1,
			})
			for range 5 {
				inputData := protocol.NewBitset(packet.PlayerAuthInputBitsetSize)
				inputData.Set(packet.InputFlagStartFlying)
				_ = r.WritePacket(&packet.PlayerAuthInput{
					InputData: inputData,
					Position:  p.Position,
				})
				time.Sleep(time.Second / 20 * 3)
			}
		}
	*/
}

// command request callback
func (r *Resources) handleCommandOutput(p *packet.CommandOutput) {
	r.commands.onCommandOutput(p)
}

// heart beat response (netease pyrpc)
func (r *Resources) handlePyRpc(p *packet.PyRpc) {
	// prepare
	if p.Value == nil {
		return
	}
	// unmarshal
	content, err := py_rpc.Unmarshal(p.Value)
	if err != nil {
		pterm.Warning.Sprintf("handlePyRpc: %v", err)
		return
	}
	// unmarshal
	switch c := content.(type) {
	case *py_rpc.HeartBeat:
		// heart beat to test the device is still alive?
		// it seems that we just need to return it back to the server is OK
		c.Type = py_rpc.ClientToServerHeartBeat
		r.client.Conn().WritePacket(&packet.PyRpc{
			Value:         py_rpc.Marshal(c),
			OperationType: packet.PyRpcOperationTypeSend,
		})
	case *py_rpc.ModEvent:
		// AI command events removed; ignore other mod events
	}
}

// inventory contents(basic)
func (r *Resources) handleInventoryContent(p *packet.InventoryContent) {
	windowName := NewWindowName(WindowID(p.WindowID), 0)
	if p.WindowID == protocol.WindowIDDynamic {
		v, found := p.Container.DynamicContainerID.Value()
		if !found {
			for _, item := range p.Content {
				if item.Stack.NBTData == nil {
					continue
				}
				switch id := item.Stack.NBTData["bundle_id"].(type) {
				case uint8:
					v, found = uint32(id), true
				case int16:
					v, found = uint32(id), true
				case uint16:
					v, found = uint32(id), true
				case int32:
					v, found = uint32(id), true
				case uint32:
					v, found = id, true
				case int64:
					v, found = uint32(id), true
				case uint64:
					v, found = uint32(id), true
				case int:
					v, found = uint32(id), true
				case uint:
					v, found = uint32(id), true
				}
				if found {
					break
				}
			}
		}
		if !found {
			pterm.Warning.Println("handleInventoryContent: dynamic container window does not have a dynamic container ID")
			return
		}
		windowName = NewWindowName(protocol.WindowIDDynamic, DynamicContainerID(v))
	}

	for key, value := range p.Content {
		slotID := SlotID(key)
		r.inventory.setItemStack(windowName, slotID, &value)
	}
}

// inventory contents(for enchant command...)
func (r *Resources) handleInventoryTransaction(p *packet.InventoryTransaction) {
	for _, value := range p.Actions {
		if value.SourceType == protocol.InventoryActionSourceCreative || value.SourceType == protocol.InventoryActionSourceWorld {
			continue
		}

		windowName, slotID := NewWindowName(WindowID(value.WindowID), 0), SlotID(value.InventorySlot)
		r.inventory.setItemStack(windowName, slotID, &value.NewItem)
	}
}

// inventory contents(for chest...) [NOT TEST]
func (r *Resources) handleInventorySlot(p *packet.InventorySlot) {
	windowName := NewWindowName(WindowID(p.WindowID), 0)
	if p.WindowID == protocol.WindowIDDynamic {
		v, found := p.Container.DynamicContainerID.Value()
		if !found && p.NewItem.Stack.NBTData != nil {
			switch id := p.NewItem.Stack.NBTData["bundle_id"].(type) {
			case uint8:
				v, found = uint32(id), true
			case int16:
				v, found = uint32(id), true
			case uint16:
				v, found = uint32(id), true
			case int32:
				v, found = uint32(id), true
			case uint32:
				v, found = id, true
			case int64:
				v, found = uint32(id), true
			case uint64:
				v, found = uint32(id), true
			case int:
				v, found = uint32(id), true
			case uint:
				v, found = uint32(id), true
			}
		}
		if !found {
			pterm.Warning.Println("handleInventorySlot: dynamic container window does not have a dynamic container ID")
			return
		}
		windowName = NewWindowName(protocol.WindowIDDynamic, DynamicContainerID(v))
	}

	slotID := SlotID(p.Slot)
	r.inventory.setItemStack(windowName, slotID, &p.NewItem)
}

// item stack request
func (r *Resources) handleItemStackResponse(p *packet.ItemStackResponse) {
	r.itemStack.mu.Lock()
	defer r.itemStack.mu.Unlock()

	select {
	case <-r.itemStack.ctx.Done():
		return
	default:
	}

	for _, response := range p.Responses {
		requestID := ItemStackRequestID(response.RequestID)
		itemRepeatChecker := make(map[SlotLocation]bool)

		callback, ok := r.itemStack.itemStackCallback[requestID]
		if !ok {
			panic(fmt.Sprintf("handleItemStackResponse: Item stack request with id %d set no callback", response.RequestID))
		}
		delete(r.itemStack.itemStackCallback, requestID)

		containerNameToWindowName, ok := r.itemStack.itemStackMapping[requestID]
		if !ok {
			panic(fmt.Sprintf("handleItemStackResponse: Item stack request with id %d set no container to window name mapping", response.RequestID))
		}
		delete(r.itemStack.itemStackMapping, requestID)

		itemUpdater := r.itemStack.itemStackUpdater[requestID]
		delete(r.itemStack.itemStackUpdater, requestID)

		if response.Status != protocol.ItemStackResponseStatusOK {
			resp := response
			go callback(&resp, nil)
			continue
		}

		for _, containerInfo := range response.ContainerInfo {
			dynamicContainerID, hasDynamicContainerID := containerInfo.Container.DynamicContainerID.Value()
			key := ContainerNameKey{ContainerID: containerInfo.Container.ContainerID, DynamicContainerID: dynamicContainerID, HasDynamicContainerID: hasDynamicContainerID}
			windowName, existed := containerNameToWindowName[key]
			if !existed && containerInfo.Container.ContainerID == protocol.ContainerDynamic && !hasDynamicContainerID {
				for mappingKey, mappingWindowName := range containerNameToWindowName {
					if mappingKey.ContainerID != protocol.ContainerDynamic {
						continue
					}
					if existed {
						panic(fmt.Sprintf("handleItemStackResponse: dynamic container ID is missing and multiple dynamic mappings exist %#v (request id = %d)", containerNameToWindowName, response.RequestID))
					}
					windowName, existed = mappingWindowName, true
				}
			}
			if !existed {
				panic(
					fmt.Sprintf(
						"handleItemStackResponse: Container %#v not existed in underlying container to window name mapping %#v (request id = %d)",
						containerInfo.Container, containerNameToWindowName, response.RequestID,
					),
				)
			}

			for _, slotInfo := range containerInfo.SlotInfo {
				slotID := SlotID(slotInfo.Slot)
				slotLocation := SlotLocation{WindowName: windowName, SlotID: slotID}
				if _, ok := itemRepeatChecker[slotLocation]; ok {
					panic(fmt.Sprintf("handleItemStackResponse: The item at %#v was found duplicates (Should nerver happened)", slotLocation))
				}
				itemRepeatChecker[slotLocation] = true

				item, inventoryExisted := r.inventory.GetItemStack(windowName, slotID)
				if !inventoryExisted {
					panic(
						fmt.Sprintf("handleItemStackResponse: Inventory whose window name is %#v is not existed (request id = %d)",
							windowName, response.RequestID,
						),
					)
				}

				UpdateNetworkItem(item, slotLocation, slotInfo, itemUpdater)
				r.inventory.setItemStack(windowName, slotID, item)
			}
		}

		resp := response
		go callback(&resp, nil)
	}
}

// when a container is opened
func (r *Resources) handleContainerOpen(p *packet.ContainerOpen) {
	r.inventory.createInventory(NewWindowName(WindowID(p.WindowID), 0))
	r.container.onContainerOpen(p)
}

// when a container has been closed
func (r *Resources) handleContainerClose(p *packet.ContainerClose) {
	switch p.WindowID {
	case protocol.WindowIDInventory, protocol.WindowIDOffHand:
	case protocol.WindowIDArmour, protocol.WindowIDUI:
	default:
		r.inventory.deleteInventory(NewWindowName(WindowID(p.WindowID), 0))
	}
	r.container.onContainerClose(p)
}

// when CompletedUsingItem is received
func (r *Resources) handleCompletedUsingItem(p *packet.CompletedUsingItem) {
	r.container.onCompletedUsingItem(p)
}

// 根据收到的数据包更新客户端的资源数据
func (r *Resources) handlePacket(pk packet.Packet) {
	if !r.interceptor.onPacket(&pk) {
		return
	}
	r.processPacket(pk)
}

// handleConnClose ..
func (r *Resources) handleConnClose(err error) {
	r.commands.handleConnClose(err)
	r.itemStack.handleConnClose(err)
	r.container.handleConnClose(err)
	r.listener.handleConnClose(err)
}

func (r *Resources) processPacket(pk packet.Packet) {
	// internal
	switch p := pk.(type) {
	case *packet.MovePlayer:
		r.handleMovePlayer(p)
	case *packet.Respawn:
		r.handleRespawn(p)
	case *packet.CommandOutput:
		r.handleCommandOutput(p)
	case *packet.PyRpc:
		r.handlePyRpc(p)
	case *packet.InventoryContent:
		r.handleInventoryContent(p)
	case *packet.InventoryTransaction:
		r.handleInventoryTransaction(p)
	case *packet.InventorySlot:
		r.handleInventorySlot(p)
	case *packet.ItemStackResponse:
		r.handleItemStackResponse(p)
	case *packet.ContainerOpen:
		r.handleContainerOpen(p)
	case *packet.ContainerClose:
		r.handleContainerClose(p)
	case *packet.ContainerRegistryCleanup:
		for _, container := range p.RemovedContainers {
			if container.ContainerID != protocol.ContainerDynamic {
				continue
			}
			dynamicContainerID, found := container.DynamicContainerID.Value()
			if found {
				r.inventory.deleteInventory(NewWindowName(protocol.WindowIDDynamic, DynamicContainerID(dynamicContainerID)))
				continue
			}
			for _, windowName := range r.inventory.GetAllWindowName() {
				if windowName.WindowID == protocol.WindowIDDynamic {
					r.inventory.deleteInventory(windowName)
				}
			}
		}
	case *packet.CompletedUsingItem:
		r.handleCompletedUsingItem(p)
	case *packet.CreativeContent:
		r.constant.onCreativeContent(p)
	case *packet.AvailableCommands:
		r.constant.onAvailableCommands(p)
	case *packet.CraftingData:
		r.constant.onCraftingData(p)
	case *packet.StructureTemplateDataResponse:
		r.structureRequest.onStructureTemplateDataResponse(p)
	case *packet.SubChunk:
		r.subChunkRequest.onSubChunk(p)
	case *packet.ChunkRadiusUpdated:
		r.subChunkRequest.onChunkRadiusUpdated(p)
	}

	// for UQHolder and other implementations
	r.uqholder.onAnyPacket(pk)
	r.listener.onPacket(pk)
}
