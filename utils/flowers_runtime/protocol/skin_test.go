package protocol

import "testing"

func TestSkinValidateAllowsEmptyImageData(t *testing.T) {
	skin := Skin{SkinImageWidth: 256, SkinImageHeight: 256}
	if err := skin.validate(); err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
}

func TestSkinValidateRejectsInvalidNonEmptyImageData(t *testing.T) {
	skin := Skin{SkinImageWidth: 256, SkinImageHeight: 256, SkinData: []byte{0}}
	if err := skin.validate(); err == nil {
		t.Fatal("validate returned nil, want error")
	}
}
