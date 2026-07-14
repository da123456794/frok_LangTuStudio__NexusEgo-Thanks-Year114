package GameInterface

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
)

type TargetQueryingInfo struct {
	Dimension byte
	Position  [3]float32
	UniqueId  string
	YRot      float32
}

// ParseTargetQueryingInfo parses the output of the querytarget command and returns the decoded targets.
// It now supports both traditional OutputMessages payloads and DataSet-based responses.
func (g *GameInterface) ParseTargetQueryingInfo(pk packet.CommandOutput) ([]TargetQueryingInfo, error) {
	candidates := collectTargetPayloads(pk)
	var decoded []interface{}
	var decodeErr error
	for _, raw := range candidates {
		if raw == "" {
			continue
		}
		decoded, decodeErr = decodeTargetQueryPayload(raw)
		if decodeErr == nil {
			break
		}
	}
	if decodeErr != nil || decoded == nil {
		return []TargetQueryingInfo{}, fmt.Errorf("ParseTargetQueryingInfo: could not parse query target data")
	}

	res := []TargetQueryingInfo{}
	for _, value := range decoded {
		val, normal := value.(map[string]interface{})
		if !normal {
			return []TargetQueryingInfo{}, fmt.Errorf("ParseTargetQueryingInfo: Could not convert value into map[string]interface{}; value = %#v", value)
		}

		dimension, ok := floatFromAny(val["dimension"])
		if !ok {
			return []TargetQueryingInfo{}, fmt.Errorf("ParseTargetQueryingInfo: Crashed in val[\"dimension\"]; val = %#v", val)
		}
		newStruct := TargetQueryingInfo{Dimension: byte(dimension)}

		positionVal, ok := val["position"]
		if !ok {
			return []TargetQueryingInfo{}, fmt.Errorf("ParseTargetQueryingInfo: Crashed in val[\"position\"]; val = %#v", val)
		}
		position, normal := positionVal.(map[string]interface{})
		if !normal {
			return []TargetQueryingInfo{}, fmt.Errorf("ParseTargetQueryingInfo: Crashed in val[\"position\"]; val = %#v", val)
		}

		x, ok := floatFromAny(position["x"])
		if !ok {
			return []TargetQueryingInfo{}, fmt.Errorf("ParseTargetQueryingInfo: Crashed in val[\"position\"][\"x\"]; val[\"position\"] = %#v", position)
		}
		y, ok := floatFromAny(position["y"])
		if !ok {
			return []TargetQueryingInfo{}, fmt.Errorf("ParseTargetQueryingInfo: Crashed in val[\"position\"][\"y\"]; val[\"position\"] = %#v", position)
		}
		z, ok := floatFromAny(position["z"])
		if !ok {
			return []TargetQueryingInfo{}, fmt.Errorf("ParseTargetQueryingInfo: Crashed in val[\"position\"][\"z\"]; val[\"position\"] = %#v", position)
		}
		newStruct.Position = [3]float32{float32(x), float32(y), float32(z)}

		uniqueId, normal := val["uniqueId"].(string)
		if !normal {
			return []TargetQueryingInfo{}, fmt.Errorf("ParseTargetQueryingInfo: Crashed in val[\"uniqueId\"]; val = %#v", val)
		}
		newStruct.UniqueId = uniqueId

		yRot, ok := floatFromAny(val["yRot"])
		if !ok {
			return []TargetQueryingInfo{}, fmt.Errorf("ParseTargetQueryingInfo: Crashed in val[\"yRot\"]; val = %#v", val)
		}
		newStruct.YRot = float32(yRot)

		res = append(res, newStruct)
	}
	return res, nil
}

func decodeTargetQueryPayload(raw string) ([]interface{}, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []interface{}{}, nil
	}

	if arr, err := decodeTargetArray(raw); err == nil {
		return arr, nil
	}

	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &obj); err == nil {
		if arr, ok := pickTargetArray(obj); ok {
			return arr, nil
		}
	}

	if arr, ok := decodeTargetFromLoose(raw); ok {
		return arr, nil
	}

	return nil, fmt.Errorf("could not parse query target data")
}

func decodeTargetArray(data string) ([]interface{}, error) {
	var arr []interface{}
	if err := json.Unmarshal([]byte(data), &arr); err != nil {
		return nil, err
	}
	return arr, nil
}

func pickTargetArray(obj map[string]interface{}) ([]interface{}, bool) {
	for _, key := range []string{"targetData", "鐩爣鏁版嵁", "data", "details"} {
		if v, ok := obj[key]; ok {
			switch data := v.(type) {
			case []interface{}:
				return data, true
			case string:
				if arr, ok := decodeTargetString(data); ok {
					return arr, true
				}
			}
		}
	}
	return nil, false
}

func decodeTargetFromLoose(raw string) ([]interface{}, bool) {
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start >= 0 && end > start {
		if arr, err := decodeTargetArray(raw[start : end+1]); err == nil {
			return arr, true
		}
	}
	return nil, false
}

func decodeTargetString(data string) ([]interface{}, bool) {
	data = strings.TrimSpace(data)
	if data == "" {
		return []interface{}{}, true
	}
	if arr, err := decodeTargetArray(data); err == nil {
		return arr, true
	}

	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(data), &obj); err == nil {
		if arr, ok := pickTargetArray(obj); ok {
			return arr, true
		}
	}

	if arr, ok := decodeTargetFromLoose(data); ok {
		return arr, true
	}

	return nil, false
}

func floatFromAny(v interface{}) (float64, bool) {
	switch num := v.(type) {
	case float64:
		return num, true
	case json.Number:
		f, err := num.Float64()
		if err != nil {
			return 0, false
		}
		return f, true
	case int:
		return float64(num), true
	case int32:
		return float64(num), true
	case int64:
		return float64(num), true
	default:
		return 0, false
	}
}

// ParseTeleportCoordinates extracts coordinates from a tp command output.
func (g *GameInterface) ParseTeleportCoordinates(pk packet.CommandOutput) ([3]float32, error) {
	nums := []float64{}
	for _, msg := range pk.OutputMessages {
		for _, p := range msg.Parameters {
			nums = append(nums, extractNumbers(p)...)
		}
	}
	if len(nums) < 3 {
		return [3]float32{}, fmt.Errorf("ParseTeleportCoordinates: insufficient coordinates")
	}
	return [3]float32{float32(nums[0]), float32(nums[1]), float32(nums[2])}, nil
}

func extractNumbers(s string) []float64 {
	out := []float64{}
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == ',' || r == '|' || r == ':' || r == ';'
	})
	for _, f := range fields {
		if f == "" {
			continue
		}
		if n, err := strconv.ParseFloat(f, 64); err == nil {
			out = append(out, n)
		}
	}
	return out
}

func collectTargetPayloads(pk packet.CommandOutput) []string {
	seen := map[string]struct{}{}
	add := func(s string, list *[]string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		*list = append(*list, s)
	}

	payloads := []string{}
	add(pk.DataSet, &payloads)

	var allParams []string
	for _, msg := range pk.OutputMessages {
		for _, p := range msg.Parameters {
			add(p, &payloads)
			if strings.TrimSpace(p) != "" {
				allParams = append(allParams, strings.TrimSpace(p))
			}
		}
	}

	if len(allParams) > 1 {
		add(strings.Join(allParams, ""), &payloads)
		add(strings.Join(allParams, " "), &payloads)
	}

	return payloads
}

