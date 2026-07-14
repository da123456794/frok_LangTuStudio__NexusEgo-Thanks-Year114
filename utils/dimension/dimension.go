package dimension

import (
	"fmt"
	"strconv"
	"strings"
)

// Info describes a dimension by command name and numeric ID.
type Info struct {
	Name string
	ID   int32
}

// Parse resolves a dimension from user input.
// Supported:
// - empty -> overworld
// - overworld/nether/the_end/end/dm
// - numeric id (0/1/2/3)
// - name:id (e.g. dm:3, custom_dim:4)
func Parse(input string) (Info, error) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return Info{Name: "overworld", ID: 0}, nil
	}
	lower := strings.ToLower(raw)

	if idx := strings.LastIndex(lower, ":"); idx > 0 {
		namePart := strings.TrimSpace(lower[:idx])
		idPart := strings.TrimSpace(lower[idx+1:])
		if idPart != "" {
			if id, err := strconv.Atoi(idPart); err == nil {
				if namePart == "" {
					return Info{}, fmt.Errorf("dimension name is empty")
				}
				return Info{Name: namePart, ID: int32(id)}, nil
			}
		}
	}

	name := strings.TrimPrefix(lower, "minecraft:")
	switch name {
	case "overworld", "ow":
		return Info{Name: "overworld", ID: 0}, nil
	case "nether", "the_nether":
		return Info{Name: "nether", ID: 1}, nil
	case "end", "the_end":
		return Info{Name: "the_end", ID: 2}, nil
	case "dm":
		return Info{Name: "dm", ID: 3}, nil
	}

	if id, err := strconv.Atoi(name); err == nil {
		switch id {
		case 0:
			return Info{Name: "overworld", ID: 0}, nil
		case 1:
			return Info{Name: "nether", ID: 1}, nil
		case 2:
			return Info{Name: "the_end", ID: 2}, nil
		case 3:
			return Info{Name: "dm", ID: 3}, nil
		default:
			return Info{}, fmt.Errorf("unknown dimension id %d, use name:id format", id)
		}
	}

	return Info{}, fmt.Errorf("unknown dimension %q", input)
}

// NameFromID returns a default command name for known dimension IDs.
func NameFromID(id int32) string {
	switch id {
	case 0:
		return "overworld"
	case 1:
		return "nether"
	case 2:
		return "the_end"
	case 3:
		return "dm"
	default:
		return ""
	}
}
