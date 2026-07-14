package provider

type OlderAbilities struct {
	AttackMobs             bool    `nbt:"attackmobs"`
	AttackPlayers          bool    `nbt:"attackplayers"`
	Build                  bool    `nbt:"build"`
	DoorsAndSwitches       bool    `nbt:"doorsandswitches"`
	FlySpeed               float32 `nbt:"flySpeed"`
	Flying                 bool    `nbt:"flying"`
	InstantBuild           bool    `nbt:"instabuild"`
	Invulnerable           bool    `nbt:"invulnerable"`
	Lightning              bool    `nbt:"lightning"`
	MayFly                 bool    `nbt:"mayfly"`
	Mine                   bool    `nbt:"mine"`
	OP                     bool    `nbt:"op"`
	OpenContainers         bool    `nbt:"opencontainers"`
	PermissionsLevel       int32   `nbt:"permissionsLevel"`
	PlayerPermissionsLevel int32   `nbt:"playerPermissionsLevel"`
	Teleport               bool    `nbt:"teleport"`
	WalkSpeed              float32 `nbt:"walkSpeed"`
}

type OlderData struct {
	BiomeOverride                   string
	CenterMapsToOrigin              bool
	ConfirmedPlatformLockedContent  bool
	Difficulty                      int32
	FlatWorldLayers                 string
	ForceGameType                   bool
	GameType                        int32
	Generator                       int32
	InventoryVersion                string
	LANBroadcast                    bool
	LANBroadcastIntent              bool
	LastPlayed                      int64
	LevelName                       string
	LimitedWorldOriginX             int32
	LimitedWorldOriginY             int32
	LimitedWorldOriginZ             int32
	MinimumCompatibleClientVersion  []int32
	MultiPlayerGame                 bool
	MultiPlayerGameIntent           bool
	NetherScale                     int32
	NetworkVersion                  int32
	Platform                        int32
	PlatformBroadcastIntent         int32
	RandomSeed                      int64
	SpawnV1Villagers                bool
	SpawnX                          int32
	SpawnY                          int32
	SpawnZ                          int32
	StorageVersion                  int32
	Time                            int64
	WorldVersion                    int32
	XBLBroadcastIntent              int32
	Abilities                       OlderAbilities
	BonusChestEnabled               bool
	BonusChestSpawned               bool
	CheatsEnabled                   bool
	CommandBlockOutput              bool
	CommandBlocksEnabled            bool
	CommandsEnabled                 bool
	CurrentTick                     int64
	DoDayLightCycle                 bool
	DayLightCycle                   int32
	DoEntityDrops                   bool
	DoFireTick                      bool
	DoImmediateRespawn              bool
	DoInsomnia                      bool
	DoMobLoot                       bool
	DoMobSpawning                   bool
	DoTileDrops                     bool
	DoWeatherCycle                  bool
	DrowningDamage                  bool
	EduLevel                        bool
	EduOffer                        int32
	EducationFeaturesEnabled        bool
	Experiments                     map[string]interface{}
	ExperimentalGamePlay            bool
	FallDamage                      bool
	FireDamage                      bool
	FreezeDamage                    bool
	FunctionCommandLimit            int32
	HasBeenLoadedInCreative         bool
	HasLockedBehaviourPack          bool
	HasLockedResourcePack           bool
	ImmutableWorld                  bool
	IsCreatedInEditor               bool
	IsExportedFromEditor            bool
	IsFromLockedTemplate            bool
	IsFromWorldTemplate             bool
	IsRandomSeedAllowed             bool
	IsSingleUseWorld                bool
	IsWorldTemplateOptionLocked     bool
	KeepInventory                   bool
	LastOpenedWithVersion           []int32
	LightningLevel                  float32
	LightningTime                   int32
	LimitedWorldDepth               int32
	LimitedWorldWidth               int32
	MaxCommandChainLength           int32
	MobGriefing                     bool
	NaturalRegeneration             bool
	NeteaseEncryptFlag              bool
	NeteaseStrongholdSelectedChunks [][]int64
	PRID                            string
	PVP                             bool
	RainLevel                       float32
	RainTime                        int32
	RandomTickSpeed                 int32
	RequiresCopiedPackRemovalCheck  bool
	RespawnBlocksExplode            bool
	SendCommandFeedback             bool
	ServerChunkTickRange            int32
	ShowBorderEffect                bool
	ShowCoordinates                 bool
	ShowDeathMessages               bool
	ShowTags                        bool
	SpawnMobs                       bool
	SpawnRadius                     int32
	StartWithMapEnabled             bool
	TexturePacksRequired            bool
	TNTExplodes                     bool
	UseMSAGamerTagsOnly             bool
	WorldStartCount                 int64
	WorldPolicies                   map[string]interface{}
}

func OlderDataToLastest(older OlderData) Data {
	data := Data{
		BiomeOverride:                  older.BiomeOverride,
		CenterMapsToOrigin:             older.CenterMapsToOrigin,
		ConfirmedPlatformLockedContent: older.ConfirmedPlatformLockedContent,
		CheatsEnabled:                  older.CheatsEnabled,
		DaylightCycle:                  older.DayLightCycle,
		Difficulty:                     older.Difficulty,
		EduOffer:                       older.EduOffer,
		FlatWorldLayers:                older.FlatWorldLayers,
		ForceGameType:                  older.ForceGameType,
		GameType:                       older.GameType,
		Generator:                      older.Generator,
		InventoryVersion:               older.InventoryVersion,
		LANBroadcast:                   older.LANBroadcast,
		LANBroadcastIntent:             older.LANBroadcastIntent,
		LastPlayed:                     older.LastPlayed,
		LevelName:                      older.LevelName,
		LimitedWorldOriginX:            older.LimitedWorldOriginX,
		LimitedWorldOriginY:            older.LimitedWorldOriginY,
		LimitedWorldOriginZ:            older.LimitedWorldOriginZ,
		LimitedWorldDepth:              older.LimitedWorldDepth,
		LimitedWorldWidth:              older.LimitedWorldWidth,
		MinimumCompatibleClientVersion: older.MinimumCompatibleClientVersion,
		MultiPlayerGame:                older.MultiPlayerGame,
		MultiPlayerGameIntent:          older.MultiPlayerGameIntent,
		NetherScale:                    older.NetherScale,
		NetworkVersion:                 older.NetworkVersion,
		Platform:                       older.Platform,
		PlatformBroadcastIntent:        older.PlatformBroadcastIntent,
		RandomSeed:                     older.RandomSeed,
		ShowTags:                       older.ShowTags,
		SingleUseWorld:                 older.IsSingleUseWorld,
		SpawnX:                         older.SpawnX,
		SpawnY:                         older.SpawnY,
		SpawnZ:                         older.SpawnZ,
		SpawnV1Villagers:               older.SpawnV1Villagers,
		StorageVersion:                 older.StorageVersion,
		Time:                           older.Time,
		XBLBroadcastIntent:             older.XBLBroadcastIntent,
		Abilities: Abilities{
			AttackMobs:             older.Abilities.AttackMobs,
			AttackPlayers:          older.Abilities.AttackPlayers,
			Build:                  older.Abilities.Build,
			Mine:                   older.Abilities.Mine,
			DoorsAndSwitches:       older.Abilities.DoorsAndSwitches,
			FlySpeed:               older.Abilities.FlySpeed,
			Flying:                 older.Abilities.Flying,
			InstantBuild:           older.Abilities.InstantBuild,
			Invulnerable:           older.Abilities.Invulnerable,
			Lightning:              older.Abilities.Lightning,
			MayFly:                 older.Abilities.MayFly,
			OP:                     older.Abilities.OP,
			OpenContainers:         older.Abilities.OpenContainers,
			PermissionsLevel:       older.Abilities.PermissionsLevel,
			PlayerPermissionsLevel: older.Abilities.PlayerPermissionsLevel,
			Teleport:               older.Abilities.Teleport,
			WalkSpeed:              older.Abilities.WalkSpeed,
		},
		BonusChestEnabled:               older.BonusChestEnabled,
		BonusChestSpawned:               older.BonusChestSpawned,
		CommandBlockOutput:              older.CommandBlockOutput,
		CommandBlocksEnabled:            older.CommandBlocksEnabled,
		CommandsEnabled:                 older.CommandsEnabled,
		CurrentTick:                     older.CurrentTick,
		DoDayLightCycle:                 older.DoDayLightCycle,
		DoEntityDrops:                   older.DoEntityDrops,
		DoFireTick:                      older.DoFireTick,
		DoImmediateRespawn:              older.DoImmediateRespawn,
		DoInsomnia:                      older.DoInsomnia,
		DoMobLoot:                       older.DoMobLoot,
		DoMobSpawning:                   older.DoMobSpawning,
		DoTileDrops:                     older.DoTileDrops,
		DoWeatherCycle:                  older.DoWeatherCycle,
		DrowningDamage:                  older.DrowningDamage,
		EduLevel:                        older.EduLevel,
		EducationFeaturesEnabled:        older.EducationFeaturesEnabled,
		ExperimentalGamePlay:            older.ExperimentalGamePlay,
		FallDamage:                      older.FallDamage,
		FireDamage:                      older.FireDamage,
		FunctionCommandLimit:            older.FunctionCommandLimit,
		HasBeenLoadedInCreative:         older.HasBeenLoadedInCreative,
		HasLockedBehaviourPack:          older.HasLockedBehaviourPack,
		HasLockedResourcePack:           older.HasLockedResourcePack,
		ImmutableWorld:                  older.ImmutableWorld,
		IsCreatedInEditor:               older.IsCreatedInEditor,
		IsExportedFromEditor:            older.IsExportedFromEditor,
		IsFromLockedTemplate:            older.IsFromLockedTemplate,
		IsFromWorldTemplate:             older.IsFromWorldTemplate,
		IsWorldTemplateOptionLocked:     older.IsWorldTemplateOptionLocked,
		KeepInventory:                   older.KeepInventory,
		LastOpenedWithVersion:           older.LastOpenedWithVersion,
		LightningLevel:                  older.LightningLevel,
		LightningTime:                   older.LightningTime,
		MaxCommandChainLength:           older.MaxCommandChainLength,
		MobGriefing:                     older.MobGriefing,
		NaturalRegeneration:             older.NaturalRegeneration,
		PRID:                            older.PRID,
		PVP:                             older.PVP,
		RainLevel:                       older.RainLevel,
		RainTime:                        older.RainTime,
		RandomTickSpeed:                 older.RandomTickSpeed,
		RequiresCopiedPackRemovalCheck:  older.RequiresCopiedPackRemovalCheck,
		SendCommandFeedback:             older.SendCommandFeedback,
		ServerChunkTickRange:            older.ServerChunkTickRange,
		ShowCoordinates:                 older.ShowCoordinates,
		ShowDeathMessages:               older.ShowDeathMessages,
		SpawnMobs:                       older.SpawnMobs,
		SpawnRadius:                     older.SpawnRadius,
		StartWithMapEnabled:             older.StartWithMapEnabled,
		TexturePacksRequired:            older.TexturePacksRequired,
		TNTExplodes:                     older.TNTExplodes,
		UseMSAGamerTagsOnly:             older.UseMSAGamerTagsOnly,
		WorldStartCount:                 older.WorldStartCount,
		Experiments:                     older.Experiments,
		FreezeDamage:                    older.FreezeDamage,
		WorldPolicies:                   older.WorldPolicies,
		WorldVersion:                    older.WorldVersion,
		RespawnBlocksExplode:            older.RespawnBlocksExplode,
		ShowBorderEffect:                older.ShowBorderEffect,
		IsRandomSeedAllowed:             older.IsRandomSeedAllowed,
		NeteaseEncryptFlag:              older.NeteaseEncryptFlag,
		NeteaseStrongholdSelectedChunks: older.NeteaseStrongholdSelectedChunks,
	}

	data.BaseGameVersion = defaultBaseGameVersion
	data.Abilities.AttackMobs = true
	data.Abilities.AttackPlayers = true
	data.Abilities.OpenContainers = true
	data.Abilities.VerticalFlySpeed = 1.0
	data.DoDayLightCycle = true
	data.DoEntityDrops = true
	data.DoFireTick = true
	data.DoInsomnia = true
	data.DoMobLoot = true
	data.DoMobSpawning = true
	data.DoTileDrops = true
	data.DoWeatherCycle = true
	data.DrowningDamage = true
	data.FallDamage = true
	data.FireDamage = true
	data.HasBeenLoadedInCreative = true
	data.FreezeDamage = true
	data.LightningLevel = 1.0
	data.MobGriefing = true
	data.PVP = true
	data.RainLevel = 1.0
	data.RespawnBlocksExplode = true
	data.SendCommandFeedback = true
	data.ShowBorderEffect = true
	data.SpawnMobs = true
	data.SpawnRadius = 5
	data.TNTExplodes = true

	return data
}
