package blocks

import (
	"bytes"
	_ "embed"
	"io"

	"nexus/utils/mirror/blocks/block_set"
	"nexus/utils/mirror/blocks/convertor"

	"github.com/andybalholm/brotli"
)

//go:embed "nemc.br"
var nemcBlockInfoBytes []byte

//go:embed "bedrock_java_to_translate.br"
var toNemcDataLoadBedrockJavaTranslateInfo []byte

//go:embed "specific_legacy_value_to_translate.br"
var toNemcDataLoadSpecificLegacyValuesTranslateInfo []byte

//go:embed "schem_to_translate.br"
var toNemcDataLoadSchemTranslateInfo []byte

func readAndUnpack(bs []byte) string {
	dataBytes, err := io.ReadAll(brotli.NewReader(bytes.NewBuffer(bs)))
	if err != nil {
		panic(err)
	}
	return string(dataBytes)
}

var MC_CURRENT *block_set.BlockSet
var MC_1_20_10 *block_set.BlockSet

// duplicate in future
var NEMC_BLOCK_VERSION = uint32(0)
var NEMC_AIR_RUNTIMEID = uint32(0)
var AIR_RUNTIMEID = uint32(0)

func initNEMCBlocks() {
	bs := block_set.BlockSetFromStringRecords(readAndUnpack(nemcBlockInfoBytes), 0xFFFFFFFF)
	MC_CURRENT = bs
	MC_1_20_10 = bs
	NEMC_BLOCK_VERSION = bs.Version()
	NEMC_AIR_RUNTIMEID = bs.AirRuntimeID()
	AIR_RUNTIMEID = bs.AirRuntimeID()
}

var DefaultAnyToNemcConvertor *convertor.ToNEMCConvertor
var SchemToNemcConvertor *convertor.ToNEMCConvertor

func initConvertor() {
	DefaultAnyToNemcConvertor = MC_CURRENT.CreateEmptyConvertor()
	SchemToNemcConvertor = MC_CURRENT.CreateEmptyConvertor()
	schematicToNemcConvertor := MC_CURRENT.CreateEmptyConvertor()
	mcConvertRecords, err := convertor.ReadRecordsFromString(readAndUnpack(toNemcDataLoadBedrockJavaTranslateInfo))
	if err != nil {
		panic(err)
	}
	specificLegacyValuesConvertRecords, err := convertor.ReadRecordsFromString(readAndUnpack(toNemcDataLoadSpecificLegacyValuesTranslateInfo))
	if err != nil {
		panic(err)
	}
	for _, r := range mcConvertRecords {
		DefaultAnyToNemcConvertor.LoadConvertRecord(r, false, true)
		SchemToNemcConvertor.LoadConvertRecord(r, false, true)
		schematicToNemcConvertor.LoadConvertRecord(r, false, true)
	}
	for _, r := range specificLegacyValuesConvertRecords {
		DefaultAnyToNemcConvertor.LoadConvertRecord(r, true, true)
		SchemToNemcConvertor.LoadConvertRecord(r, true, true)
		schematicToNemcConvertor.LoadConvertRecord(r, true, true)
	}
	schemConvertRecords, err := convertor.ReadRecordsFromString(readAndUnpack(toNemcDataLoadSchemTranslateInfo))
	if err != nil {
		panic(err)
	}
	for _, r := range schemConvertRecords {
		DefaultAnyToNemcConvertor.LoadConvertRecord(r, false, false)
		SchemToNemcConvertor.LoadConvertRecord(r, true, true)
	}
}

var inited bool

func init() {
	initNEMCBlocks()
}

func Init() {
	if inited {
		return
	}
	inited = true
	initConvertor()
}
