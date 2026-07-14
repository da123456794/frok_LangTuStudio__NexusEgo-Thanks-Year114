package packet

const (
	IDLogin = iota + 1
	IDPlayStatus
	IDServerToClientHandshake
	IDClientToServerHandshake
	IDDisconnect        //b 5
	IDResourcePacksInfo //b 6
	IDResourcePackStack //b 7
	IDResourcePackClientResponse
	IDText //b 9
	IDSetTime
	IDStartGame //b 11
	IDAddPlayer //b 12
	IDAddActor  //b 13
	IDRemoveActor
	IDAddItemActor
	_
	IDTakeItemActor
	IDMoveActorAbsolute
	IDMovePlayer
	IDPassengerJump
	IDUpdateBlock
	IDAddPainting
	IDTickSync
	IDLevelSoundEventV1 // Netease: new packet
	IDLevelEvent
	IDBlockEvent
	IDActorEvent
	IDMobEffect
	IDUpdateAttributes
	IDInventoryTransaction
	IDMobEquipment
	IDMobArmourEquipment
	IDInteract
	IDBlockPickRequest
	IDActorPickRequest
	IDPlayerAction
	_
	IDHurtArmour
	IDSetActorData
	IDSetActorMotion
	IDSetActorLink
	IDSetHealth
	IDSetSpawnPosition
	IDAnimate
	IDRespawn
	IDContainerOpen
	IDContainerClose
	IDPlayerHotBar
	IDInventoryContent
	IDInventorySlot
	IDContainerSetData
	IDCraftingData //b 52
	_
	IDGUIDataPickItem
	IDAdventureSettings // Netease: missing
	IDBlockActorData
	IDPlayerInput
	IDLevelChunk
	IDSetCommandsEnabled
	IDSetDifficulty
	IDChangeDimension
	IDSetPlayerGameType
	IDPlayerList //b 63
	IDSimpleEvent
	IDEvent
	IDSpawnExperienceOrb
	IDClientBoundMapItemData
	IDMapInfoRequest
	IDRequestChunkRadius
	IDChunkRadiusUpdated
	IDItemFrameDropItem
	IDGameRulesChanged
	IDCamera
	IDBossEvent
	IDShowCredits
	IDAvailableCommands
	IDCommandRequest
	IDCommandBlockUpdate
	IDCommandOutput
	IDUpdateTrade
	IDUpdateEquip
	IDResourcePackDataInfo
	IDResourcePackChunkData
	IDResourcePackChunkRequest
	IDTransfer
	IDPlaySound
	IDStopSound
	IDSetTitle
	IDAddBehaviourTree
	IDStructureBlockUpdate
	IDShowStoreOffer
	IDPurchaseReceipt
	IDPlayerSkin
	IDSubClientLogin
	IDAutomationClientConnect
	IDSetLastHurtBy
	IDBookEdit
	IDNPCRequest
	IDPhotoTransfer
	IDModalFormRequest
	IDModalFormResponse
	IDServerSettingsRequest
	IDServerSettingsResponse
	IDShowProfile
	IDSetDefaultGameType
	IDRemoveObjective
	IDSetDisplayObjective
	IDSetScore
	IDLabTable
	IDUpdateBlockSynced
	IDMoveActorDelta
	IDSetScoreboardIdentity
	IDSetLocalPlayerAsInitialised
	IDUpdateSoftEnum
	IDNetworkStackLatency
	_
	IDScriptCustomEvent // Netease: missing
	IDSpawnParticleEffect
	IDAvailableActorIdentifiers
	IDLevelSoundEventV2 // Netease: new packet
	IDNetworkChunkPublisherUpdate
	IDBiomeDefinitionList
	IDLevelSoundEvent
	IDLevelEventGeneric
	IDLecternUpdate
	_
	_
	_
	IDClientCacheStatus
	IDOnScreenTextureAnimation // Netease: 131 -> 130
	IDMapCreateLockedCopy      // Netease: 130 -> 131
	IDStructureTemplateDataRequest
	IDStructureTemplateDataResponse
	_
	IDClientCacheBlobStatus
	IDClientCacheMissResponse
	IDEducationSettings
	IDEmote
	IDMultiPlayerSettings
	IDSettingsCommand
	IDAnvilDamage
	IDCompletedUsingItem
	IDNetworkSettings
	IDPlayerAuthInput
	IDCreativeContent
	IDPlayerEnchantOptions
	IDItemStackRequest
	IDItemStackResponse
	IDPlayerArmourDamage
	IDCodeBuilder
	IDUpdatePlayerGameType //b 151
	IDEmoteList
	IDPositionTrackingDBServerBroadcast
	IDPositionTrackingDBClientRequest
	IDDebugInfo
	IDPacketViolationWarning
	IDMotionPredictionHints
	IDAnimateEntity
	IDCameraShake
	IDPlayerFog
	IDCorrectPlayerMovePrediction
	IDItemComponent
	IDFilterText
	IDClientBoundDebugRenderer
	IDSyncActorProperty
	IDAddVolumeEntity
	IDRemoveVolumeEntity
	IDSimulationType
	IDNPCDialogue
	IDEducationResourceURI
	IDCreatePhoto
	IDUpdateSubChunkBlocks
	IDPhotoInfoRequest // Netease: missing
	IDSubChunk
	IDSubChunkRequest
	IDClientStartItemCooldown
	IDScriptMessage
	IDCodeBuilderSource
	IDTickingAreasLoadStatus
	IDDimensionData
	IDAgentAction
	IDChangeMobProperty
	IDLessonProgress
	IDRequestAbility
	IDRequestPermissions
	IDToastRequest
	IDUpdateAbilities //b 187
	IDUpdateAdventureSettings
	IDDeathInfo
	IDEditorNetwork
	IDFeatureRegistry
	IDServerStats
	IDRequestNetworkSettings
	IDGameTestRequest
	IDGameTestResults
	IDUpdateClientInputLocks
	IDClientCheatAbility // Netease: missing
	IDCameraPresets
	IDUnlockedRecipes
	IDPyRpc                                     // Netease: new packet
	IDChangeModel                               // Netease: new packet
	IDStoreBuySucc                              // Netease: new packet
	IDNeteaseJson                               // Netease: new packet
	IDChangeModelTexture                        // Netease: new packet
	IDChangeModelOffset                         // Netease: new packet
	IDChangeModelBind                           // Netease: new packet
	IDHungerAttr                                // Netease: new packet
	IDSetDimensionLocalTime                     // Netease: new packet
	IDWithdrawFurnaceXp                         // Netease: new packet
	IDSetDimensionLocalWeather                  // Netease: new packet
	IDCustomV1                      = iota + 13 // Netease: new packet
	IDCombine                                   // Netease: new packet
	IDVConnection                               // Netease: new packet
	IDTransport                                 // Netease: new packet
	IDCustomV2                                  // Netease: new packet
	IDConfirmSkin                               // Netease: new packet
	IDTransportNoCompress                       // Netease: new packet
	IDMobEffectV2                               // Netease: new packet
	IDMobBlockActorChanged                      // Netease: new packet
	IDChangeActorMotion                         // Netease: new packet
	IDAnimateEmoteEntity                        // Netease: new packet
	IDChangeBiome                               // Netease: new packet
	IDUpdateBiome                               // Netease: new packet
	IDSyncSkin                                  // Netease: new packet
	IDCameraInstruction             = iota + 76 // Netease: 301 -> 300
	IDCompressedBiomeDefinitionList             // Netease: 302 -> 301
	IDTrimData                                  // Netease: 303 -> 302
	IDOpenSign                                  // Netease: 304 -> 303
	IDAgentAnimation                            // Netease: 305 -> 304
	IDRefreshEntitlements
	IDPlayerToggleCrafterSlotRequest
	IDSetPlayerInventoryOptions
	IDSetHud
	IDAwardAchievement
	IDClientBoundCloseForm
	IDUnused311
	IDServerBoundLoadingScreen = 312
	IDJigsawStructureData      = 313
	IDCurrentStructureFeature  = 314
	IDServerBoundDiagnostics   = 315

	// 1.21.90 additions on top of the NetEase fork.
	IDCameraAimAssist              = 316
	IDContainerRegistryCleanup     = 317
	IDMovementEffect               = 318
	IDCameraAimAssistPresets       = 320
	IDClientCameraAimAssist        = 321
	IDClientMovementPredictionSync = 322
	IDUpdateClientOptions          = 323
	IDPlayerVideoCapture           = 324
	IDPlayerUpdateEntityOverrides  = 325
	IDPlayerLocation               = 326
	IDClientBoundControlSchemeSet  = 327
	IDServerScriptDebugDrawer      = 328
)

const IDItemRegistry = IDItemComponent
