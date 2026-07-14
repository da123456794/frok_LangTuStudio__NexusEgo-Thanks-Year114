package base

import (
	"bytes"
	"encoding/gob"
	"fmt"

	"github.com/google/uuid"
)

func GenerateUUID() uuid.UUID {
	for {
		uniqueID, err := uuid.NewUUID()
		if err != nil {
			continue
		}
		return uniqueID
	}
}

func DeepCopy(source any, destination any, register func()) error {
	register()
	var buffer bytes.Buffer

	if err := gob.NewEncoder(&buffer).Encode(source); err != nil {
		return fmt.Errorf("DeepCopy: %v", err)
	}
	if err := gob.NewDecoder(&buffer).Decode(destination); err != nil {
		return fmt.Errorf("DeepCopy: %v", err)
	}
	return nil
}
