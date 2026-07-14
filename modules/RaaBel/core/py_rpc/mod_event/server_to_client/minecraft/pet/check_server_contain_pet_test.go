package pet

import "testing"

func TestCheckServerContainPetFromGoAcceptsClosePetAddon(t *testing.T) {
	var event CheckServerContainPet
	if err := event.FromGo(map[string]any{"close_pet_addon": true}); err != nil {
		t.Fatalf("FromGo returned error: %v", err)
	}
	if !event.ClosePetAddon {
		t.Fatal("ClosePetAddon = false, want true")
	}
}

func TestCheckServerContainPetFromGoAcceptsEmptyObject(t *testing.T) {
	var event CheckServerContainPet
	if err := event.FromGo(map[string]any{}); err != nil {
		t.Fatalf("FromGo returned error: %v", err)
	}
	if event.ClosePetAddon {
		t.Fatal("ClosePetAddon = true, want false")
	}
}
