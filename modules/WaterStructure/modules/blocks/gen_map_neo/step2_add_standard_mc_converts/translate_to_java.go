package step2_add_standard_mc_converts

import (
	"fmt"

	"github.com/Yeah114/blocks/convertor"
	"github.com/Yeah114/blocks/describe"
)

func GenBedrockToJavaTranslateRecords(
	snbtInOut [][4]string,
	c *convertor.ToNEMCConvertor,
) []*convertor.JavaConvertRecord {
	translated := 0
	ignored := 0
	conflicted := 0
	records := []*convertor.JavaConvertRecord{}
	postponeSnbtInOut := [][4]string{}

	for _, s := range snbtInOut {
		inBlockName, inBlockState, outBlockName, outBlockState := s[0], s[1], s[2], s[3]
		inBlockName, inBlockState, outBlockName, outBlockState = AlterInOutSnbtBlock(inBlockName, inBlockState, outBlockName, outBlockState)
		record, ok, notMatched := TryAddJavaConvert(inBlockName, inBlockState, outBlockName, outBlockState, c, false)
		if notMatched {
			_s := s
			postponeSnbtInOut = append(postponeSnbtInOut, _s)
			continue
		}
		if !ok {
			conflicted += 1
			continue
		}
		if record == nil {
			ignored += 1
			continue
		}
		translated += 1
		records = append(records, record)
	}

	for _, s := range postponeSnbtInOut {
		inBlockName, inBlockState, outBlockName, outBlockState := s[0], s[1], s[2], s[3]
		inBlockName, inBlockState, outBlockName, outBlockState = AlterInOutSnbtBlock(inBlockName, inBlockState, outBlockName, outBlockState)
		record, ok, _ := TryAddJavaConvert(inBlockName, inBlockState, outBlockName, outBlockState, c, true)
		if !ok {
			conflicted += 1
			continue
		}
		if record == nil {
			ignored += 1
			continue
		}
		translated += 1
		records = append(records, record)
	}

	fmt.Printf("translated: %v\n", translated)
	fmt.Printf("ignored: %v\n", ignored)
	fmt.Printf("conflicted: %v\n", conflicted)
	return records
}

func TryAddJavaConvert(inBlockName, inBlockState, outBlockName, outBlockState string,
	c *convertor.ToNEMCConvertor,
	mustMatch bool) (record *convertor.JavaConvertRecord, ok bool, notMatched bool) {

	// Input is Bedrock block, find it in our system
	inBlockNameForSearch := describe.BlockNameForSearch(inBlockName)
	inBlockStateForSearch, err := describe.PropsForSearchFromStr(inBlockState)
	if err != nil {
		panic(err)
	}

	// Check if this Bedrock block exists in our NEMC system
	_, found := c.PreciseMatchByState(inBlockNameForSearch, inBlockStateForSearch)
	if !found {
		if !mustMatch {
			return nil, false, true
		}
		// Try fuzzy match
		_, _, found = c.TryBestSearchByState(inBlockNameForSearch, inBlockStateForSearch)
		if !found {
			// This Bedrock block doesn't exist in our system, ignore it
			return nil, true, false
		}
	}

	// Output is Java block
	if outBlockName == "air" || outBlockName == "minecraft:air" {
		return nil, true, false
	}

	// Create the conversion record
	if inBlockState == "" || inBlockState == "{}" {
		inBlockState = "{}"
	}
	if outBlockState == "" || outBlockState == "{}" {
		outBlockState = ""
	}

	return &convertor.JavaConvertRecord{
		Name:             inBlockNameForSearch.BaseName(),
		SNBTStateOrValue: inBlockStateForSearch.InPreciseSNBT(),
		JavaBlockName:    outBlockName,
		JavaBlockSNBT:    outBlockState,
	}, true, false
}
