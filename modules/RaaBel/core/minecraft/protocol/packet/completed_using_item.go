package packet

import (
	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
)

const (
	UseItemEquipArmour = iota
	UseItemEat
	UseItemAttack
	UseItemConsume
	UseItemThrow
	UseItemShoot
	UseItemPlace
	UseItemFillBottle
	UseItemFillBucket
	UseItemPourBucket
	UseItemUseTool
	UseItemInteract
	UseItemRetrieved
	UseItemDyed
	UseItemTraded
	UseItemBrushingCompleted
	UseItemOpenedVault
)

// CompletedUsingItem is sent by the server to tell the client that it should be done using the item it is
// currently using.
type CompletedUsingItem struct {
	// UsedItemID is the item ID of the item that the client completed using. This should typically be the
	// ID of the item held in the hand.
	UsedItemID int16
	// UseMethod is the method of the using of the item that was completed. It is one of the constants that
	// may be found above.
	UseMethod int32
	// Item is the protocol-extension payload as an ItemInstance when UseMethod = -1.
	Item protocol.ItemInstance
}

// ID ...
func (*CompletedUsingItem) ID() uint32 {
	return IDCompletedUsingItem
}

func (pk *CompletedUsingItem) Marshal(io protocol.IO) {
	io.Int16(&pk.UsedItemID)
	io.Int32(&pk.UseMethod)
	if pk.UseMethod == -1 {
		io.ItemInstance(&pk.Item)
	}
}
