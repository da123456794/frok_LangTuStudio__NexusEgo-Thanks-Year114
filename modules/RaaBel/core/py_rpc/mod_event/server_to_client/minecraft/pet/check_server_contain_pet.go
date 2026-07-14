package pet

import "fmt"

type CheckServerContainPet struct {
	ClosePetAddon bool `json:"close_pet_addon"`
}

// Return the event name of c
func (c *CheckServerContainPet) EventName() string {
	return "check_server_contain_pet"
}

// Convert c to go object which only contains go-built-in types
func (c *CheckServerContainPet) MakeGo() (res any) {
	if c.ClosePetAddon {
		return map[string]any{"close_pet_addon": c.ClosePetAddon}
	}
	return map[string]any{}
}

// Sync data to c from obj
func (c *CheckServerContainPet) FromGo(obj any) error {
	object, success := obj.(map[string]any)
	if !success {
		return fmt.Errorf("FromGo: Failed to convert obj to map[string]interface{}; obj = %#v", obj)
	}
	closePetAddon := false
	if value, ok := object["close_pet_addon"]; ok {
		closePetAddon, success = value.(bool)
		if !success {
			return fmt.Errorf(`FromGo: Failed to convert object["close_pet_addon"] to bool; object["close_pet_addon"] = %#v`, value)
		}
	}
	*c = CheckServerContainPet{ClosePetAddon: closePetAddon}
	return nil
}
