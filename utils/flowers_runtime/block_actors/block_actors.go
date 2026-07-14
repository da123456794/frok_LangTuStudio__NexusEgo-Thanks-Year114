package block_actors

import "nexus/utils/flowers_runtime/protocol"

// 描述方块实体的通用接口
type BlockActors interface {
	ID() string             // 返回该方块实体的ID
	Marshal(io protocol.IO) // 解码或编码为二进制的平铺型NBT
}

// 以下列出了各个方块实体的ID
const (
	IDChiseledBookshelf = "ChiseledBookshelf"
	IDCraftingTable     = "CraftingTable"
	IDDayLightDetector  = "DaylightDetector"
	IDEndPortal         = "EndPortal"
	IDSculkCatalyst     = "SculkCatalyst"
	IDSporeBlossom      = "SporeBlossom"
	IDTrialSpawner      = "TrialSpawner"
	IDVault             = "Vault"

	IDBrushableBlock = "BrushableBlock"
	IDDecoratedPot   = "DecoratedPot"

	IDBanner           = "Banner"
	IDBeacon           = "Beacon"
	IDBed              = "Bed"
	IDBeehive          = "Beehive"
	IDBell             = "Bell"
	IDBrewingStand     = "BrewingStand"
	IDCampfire         = "Campfire"
	IDCommandBlock     = "CommandBlock"
	IDComparator       = "Comparator"
	IDConduit          = "Conduit"
	IDCauldron         = "Cauldron"
	IDEnchantTable     = "EnchantTable"
	IDEndGateway       = "EndGateway"
	IDFlowerPot        = "FlowerPot"
	IDHopper           = "Hopper"
	IDJigsawBlock      = "JigsawBlock"
	IDJukebox          = "Jukebox"
	IDLectern          = "Lectern"
	IDLodestone        = "Lodestone"
	IDMobSpawner       = "MobSpawner"
	IDMovingBlock      = "MovingBlock"
	IDNeteaseContainer = "NeteaseContainer"
	IDNetherReactor    = "NetherReactor"
	IDMusic            = "Music"
	IDPistonArm        = "PistonArm"
	IDSkull            = "Skull"
	IDStructureBlock   = "StructureBlock"

	IDSign        = "Sign"
	IDHangingSign = "HangingSign"

	IDSculkSensor           = "SculkSensor"
	IDCalibratedSculkSensor = "CalibratedSculkSensor"
	IDSculkShrieker         = "SculkShrieker"

	IDFurnace      = "Furnace"
	IDBlastFurnace = "BlastFurnace"
	IDSmoker       = "Smoker"

	IDChest      = "Chest"
	IDBarrel     = "Barrel"
	IDEnderChest = "EnderChest"
	IDShulkerBox = "ShulkerBox"

	IDDispenser = "Dispenser"
	IDDropper   = "Dropper"
	IDCrafter   = "Crafter"

	IDItemFrame     = "ItemFrame"
	IDGlowItemFrame = "GlowItemFrame"

	IDChemistryTable = "ChemistryTable"
	IDModBlock       = "ModBlock"
)

func NewPool() map[string]BlockActors {
	return map[string]BlockActors{
		IDBanner:                &Banner{},
		IDBarrel:                &Barrel{},
		IDBeacon:                &Beacon{},
		IDBed:                   &Bed{},
		IDBeehive:               &Beehive{},
		IDBell:                  &Bell{},
		IDBlastFurnace:          &BlastFurnace{},
		IDBrewingStand:          &BrewingStand{},
		IDBrushableBlock:        &BrushableBlock{},
		IDCalibratedSculkSensor: &CalibratedSculkSensor{},
		IDCampfire:              &Campfire{},
		IDCauldron:              &Cauldron{},
		IDChemistryTable:        &ChemistryTable{},
		IDChest:                 &Chest{},
		IDChiseledBookshelf:     &ChiseledBookshelf{},
		IDCommandBlock:          &CommandBlock{},
		IDComparator:            &Comparator{},
		IDConduit:               &Conduit{},
		IDCrafter:               &Crafter{},
		IDCraftingTable:         &CraftingTable{},
		IDDayLightDetector:      &DayLightDetector{},
		IDDecoratedPot:          &DecoratedPot{},
		IDDispenser:             &Dispenser{},
		IDDropper:               &Dropper{},
		IDEnchantTable:          &EnchantTable{},
		IDEndPortal:             &EndPortal{},
		IDEnderChest:            &EnderChest{},
		IDEndGateway:            &EndGateway{},
		IDFlowerPot:             &FlowerPot{},
		IDFurnace:               &Furnace{},
		IDGlowItemFrame:         &GlowItemFrame{},
		IDHangingSign:           &HangingSign{},
		IDHopper:                &Hopper{},
		IDItemFrame:             &ItemFrame{},
		IDJigsawBlock:           &JigsawBlock{},
		IDJukebox:               &Jukebox{},
		IDLectern:               &Lectern{},
		IDLodestone:             &Lodestone{},
		IDMobSpawner:            &MobSpawner{},
		IDModBlock:              &ModBlock{},
		IDMovingBlock:           &MovingBlock{},
		IDNetherReactor:         &NetherReactor{},
		IDMusic:                 &Music{},
		IDPistonArm:             &PistonArm{},
		IDSculkCatalyst:         &SculkCatalyst{},
		IDSculkSensor:           &SculkSensor{},
		IDSculkShrieker:         &SculkShrieker{},
		IDShulkerBox:            &ShulkerBox{},
		IDSign:                  &Sign{},
		IDSkull:                 &Skull{},
		IDSmoker:                &Smoker{},
		IDSporeBlossom:          &SporeBlossom{},
		IDStructureBlock:        &StructureBlock{},
		IDTrialSpawner:          &TrialSpawner{},
		IDVault:                 &Vault{},
	}
}
