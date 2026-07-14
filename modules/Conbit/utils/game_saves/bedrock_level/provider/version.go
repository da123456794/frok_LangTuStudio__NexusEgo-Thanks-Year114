package provider

import (
	"strconv"
	"strings"
)

const Version = 10

const currentVersion = "1.21.2"
const currentProtocol = 686

var MinimumCompatibleClientVersion []int32

func init() {
	fullVersion := append(strings.Split(currentVersion, "."), "0", "0")
	for _, v := range fullVersion {
		i, _ := strconv.Atoi(v)
		MinimumCompatibleClientVersion = append(MinimumCompatibleClientVersion, int32(i))
	}
}
