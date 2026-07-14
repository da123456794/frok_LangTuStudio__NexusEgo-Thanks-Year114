package convertor

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Yeah114/blocks/describe"
)

// JavaConvertRecord represents a conversion record to Java blocks
type JavaConvertRecord struct {
	Name             string // base name
	SNBTStateOrValue string // either SNBT state string or legacy value number
	JavaBlockName    string
	JavaBlockSNBT    string
}

func (r *JavaConvertRecord) String() string {
	return fmt.Sprintf("%v\n%v\n%v\n%v\n", r.Name, r.SNBTStateOrValue, r.JavaBlockName, r.JavaBlockSNBT)
}

func (r *JavaConvertRecord) GetLegacyValue() (uint16, bool) {
	val, err := strconv.Atoi(r.SNBTStateOrValue)
	if err != nil {
		return 0, false
	}
	return uint16(val), true
}

func ReadJavaRecordsFromString(s string) ([]*JavaConvertRecord, error) {
	records := []*JavaConvertRecord{}
	lines := strings.Split(s, "\n")
	for i := 0; i+3 < len(lines); i += 4 {
		name := strings.TrimSpace(lines[i])
		snbtStateOrValue := strings.TrimSpace(lines[i+1])
		javaName := strings.TrimSpace(lines[i+2])
		javaSNBT := strings.TrimSpace(lines[i+3])
		if name == "" {
			continue
		}
		records = append(records, &JavaConvertRecord{
			Name:             name,
			SNBTStateOrValue: snbtStateOrValue,
			JavaBlockName:    javaName,
			JavaBlockSNBT:    javaSNBT,
		})
	}
	return records, nil
}

func (c *ToJavaConvertor) LoadJavaConvertRecord(r *JavaConvertRecord, overwrite bool, strict bool) {
	javaBlock := describe.NewJavaBlockString(r.JavaBlockName, r.JavaBlockSNBT)

	if val, ok := r.GetLegacyValue(); ok {
		if exist, err := c.AddAnchorByLegacyValue(describe.BlockNameForSearch(r.Name), val, javaBlock, overwrite); err != nil || exist {
			if strict {
				panic(fmt.Errorf("fail to add java translation: %v %v %v", r.Name, val, javaBlock.String()))
			}
		}
	} else {
		props, err := describe.PropsForSearchFromStr(r.SNBTStateOrValue)
		if err != nil {
			panic(err)
		}
		if exist, err := c.AddAnchorByState(describe.BlockNameForSearch(r.Name), props, javaBlock, overwrite); err != nil || exist {
			if strict {
				panic(fmt.Errorf("fail to add java translation: %v %v %v", r.Name, props.InPreciseSNBT(), javaBlock.String()))
			}
		}
	}
}
