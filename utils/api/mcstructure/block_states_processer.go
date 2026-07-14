package mcstructure

import (
	"fmt"
	"nexus/utils/command_upgrader/mc_command_parser"
	"strings"
)

func MarshalBlockStates(blockStates map[string]interface{}) (string, error) {
	temp := []string{}
	separator := mc_command_parser.BlockStatesDefaultSeparator
	for key, value := range blockStates {
		switch val := value.(type) {
		case string:
			temp = append(temp, fmt.Sprintf(
				"%#v%s%#v", key, separator, val,
			))
			// e.g. "color"="orange"
		case bool:
			if val {
				temp = append(temp, fmt.Sprintf("%#v%strue", key, separator))
			} else {
				temp = append(temp, fmt.Sprintf("%#v%sfalse", key, separator))
			}
			// e.g. "open_bit"=true
		case byte:
			if val == 0 || val == 1 {
				if val == 0 {
					temp = append(temp, fmt.Sprintf("%#v%sfalse", key, separator))
				} else {
					temp = append(temp, fmt.Sprintf("%#v%strue", key, separator))
				}
			} else {
				temp = append(temp, fmt.Sprintf("%#v%s%d", key, separator, val))
			}
			// e.g. "open_bit"=true
		case int32:
			temp = append(temp, fmt.Sprintf("%#v%s%d", key, separator, val))
			// e.g. "facing_direction"=0
		case int:
			temp = append(temp, fmt.Sprintf("%#v%s%d", key, separator, val))
		case int64:
			temp = append(temp, fmt.Sprintf("%#v%s%d", key, separator, val))
		case uint16:
			temp = append(temp, fmt.Sprintf("%#v%s%d", key, separator, val))
		case uint32:
			temp = append(temp, fmt.Sprintf("%#v%s%d", key, separator, val))
		case uint64:
			temp = append(temp, fmt.Sprintf("%#v%s%d", key, separator, val))
		default:
			return "", fmt.Errorf("MarshalBlockStates: Unexpected data type of blockStates[%#v]; blockStates[%#v] = %#v", key, key, value)
		}
	}
	return fmt.Sprintf("[%s]", strings.Join(temp, ",")), nil
}

func UnmarshalBlockStates(blockStates string) (m map[string]interface{}, err error) {
	func() {
		defer func() {
			if errMessage := recover(); errMessage != nil {
				err = fmt.Errorf("UnmarshalBlockStates: %v", errMessage)
			}
		}()
		m = mc_command_parser.ParseBlockStates(blockStates)
	}()
	return
}
