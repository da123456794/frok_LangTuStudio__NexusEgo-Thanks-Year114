package ResourcesControl

import (
	"fmt"

	"github.com/LangTuStudio/Conbit/minecraft/protocol"
	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
	newgamedata "github.com/LangTuStudio/Conbit/minecraft_neo/game_data"
)

type BotInfo struct {
	BotName         string
	XUID            string
	EntityUniqueID  int64
	EntityRuntimeID uint64
}

type CommandRequestCallback = command_request_with_response
type Inventories = inventory_contents
type ItemStackOperationManager = item_stack_request_with_response
type ContainerManager = container
type PacketListener = packet_listener

type WindowID = uint32
type SlotID = uint8

type Inventory struct {
	slots map[SlotID]protocol.ItemInstance
}

func NewAirItemStack() *protocol.ItemStack {
	return &protocol.ItemStack{
		ItemType: protocol.ItemType{
			NetworkID:     -1,
			MetadataValue: 0,
		},
		BlockRuntimeID: 0,
		Count:          0,
		NBTData:        make(map[string]any),
	}
}

func NewAirItemInstance() *protocol.ItemInstance {
	return &protocol.ItemInstance{
		StackNetworkID: 0,
		Stack:          *NewAirItemStack(),
	}
}

func (i *Inventory) GetItemStack(slotID SlotID) *protocol.ItemInstance {
	if i == nil {
		return nil
	}
	item, ok := i.slots[slotID]
	if !ok {
		return nil
	}
	copied := item
	return &copied
}

func (i *Inventory) GetAllItemStack() map[SlotID]*protocol.ItemInstance {
	if i == nil {
		return nil
	}
	result := make(map[SlotID]*protocol.ItemInstance, len(i.slots))
	for slotID, item := range i.slots {
		copied := item
		result[slotID] = &copied
	}
	return result
}

func (i *inventory_contents) GetInventory(windowID WindowID) (inventory *Inventory, existed bool) {
	data, err := i.GetInventoryInfo(uint32(windowID))
	if err != nil {
		return nil, false
	}
	slots := make(map[SlotID]protocol.ItemInstance, len(data))
	for slotID, item := range data {
		slots[SlotID(slotID)] = item
	}
	return &Inventory{slots: slots}, true
}

func (i *inventory_contents) GetItemStack(windowID WindowID, slotID SlotID) (item *protocol.ItemInstance, inventoryExisted bool) {
	got, err := i.GetItemStackInfo(uint32(windowID), uint8(slotID))
	if err != nil {
		return nil, false
	}
	copied := got
	return &copied, true
}

func (i *inventory_contents) GetAllItemStack(windowID WindowID) (mapping map[SlotID]*protocol.ItemInstance, inventoryExisted bool) {
	got, err := i.GetInventoryInfo(uint32(windowID))
	if err != nil {
		return nil, false
	}
	result := make(map[SlotID]*protocol.ItemInstance, len(got))
	for slotID, item := range got {
		copied := item
		result[SlotID(slotID)] = &copied
	}
	return result, true
}

func (i *inventory_contents) GetAllWindowID() (result []WindowID) {
	windowIDs := i.ListWindowID()
	result = make([]WindowID, 0, len(windowIDs))
	for _, id := range windowIDs {
		result = append(result, WindowID(id))
	}
	return result
}

func (r *Resources) BindRuntime(writePacket func(packet.Packet) error, info BotInfo, gameData newgamedata.GameData) {
	if r == nil {
		return
	}
	r.writePacket = writePacket
	r.botInfo = info
	if r.constant == nil {
		r.constant = NewConstantPacket()
	}
	r.constant.updateByGameData(gameData)
	r.Inventory.create_new_inventory(uint32(protocol.WindowIDInventory))
}

func (r *Resources) WritePacket(p packet.Packet) error {
	if r == nil || r.writePacket == nil {
		return fmt.Errorf("resources write packet not configured")
	}
	return r.writePacket(p)
}

func (r *Resources) Commands() *CommandRequestCallback {
	if r == nil {
		return nil
	}
	return &r.Command
}

func (r *Resources) Inventories() *Inventories {
	if r == nil {
		return nil
	}
	return &r.Inventory
}

func (r *Resources) ItemStackOperationManager() *ItemStackOperationManager {
	if r == nil {
		return nil
	}
	return &r.ItemStackOperation
}

func (r *Resources) ItemStackOperationCompat() *ItemStackOperationManager {
	if r == nil {
		return nil
	}
	return &r.ItemStackOperation
}

func (r *Resources) ContainerManager() *ContainerManager {
	if r == nil {
		return nil
	}
	return &r.Container
}

func (r *Resources) PacketListener() *PacketListener {
	if r == nil {
		return nil
	}
	return &r.Listener
}

func (r *Resources) ConstantPacket() *ConstantPacket {
	if r == nil {
		return nil
	}
	if r.constant == nil {
		r.constant = NewConstantPacket()
	}
	return r.constant
}

func (r *Resources) BotInfo() BotInfo {
	if r == nil {
		return BotInfo{}
	}
	return r.botInfo
}

