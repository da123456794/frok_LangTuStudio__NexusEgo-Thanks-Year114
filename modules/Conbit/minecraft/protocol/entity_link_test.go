package protocol

import (
	"bytes"
	"testing"
)

func TestEntityLinkReadsVehicleAngularVelocity(t *testing.T) {
	original := EntityLink{
		RiddenEntityUniqueID:   42,
		RiderEntityUniqueID:    84,
		Type:                   EntityLinkRider,
		RiderInitiated:         true,
		VehicleAngularVelocity: 1.25,
	}

	buf := bytes.NewBuffer(nil)
	original.Marshal(NewWriter(buf, 0))

	readBuf := bytes.NewBuffer(buf.Bytes())
	var decoded EntityLink
	decoded.Marshal(NewReader(readBuf, 0, false))

	if readBuf.Len() != 0 {
		t.Fatalf("entity link decode left %v unread bytes", readBuf.Len())
	}
	if decoded.VehicleAngularVelocity != original.VehicleAngularVelocity {
		t.Fatalf("unexpected angular velocity: %v", decoded.VehicleAngularVelocity)
	}
}
