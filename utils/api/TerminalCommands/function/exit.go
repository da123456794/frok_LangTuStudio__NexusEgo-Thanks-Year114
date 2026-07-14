package function

import (
	"fmt"
	"os"

	"nexus/utils/client"
)

func Exit(client *client.Client, words []string) bool {
	if client.Conn != nil {
		client.GameInterface.SendCommand(fmt.Sprintf("kick \"%s\" EDotCS Robot Exit", client.Conn.IdentityData().DisplayName))
		client.Conn.Close()
	}
	os.Exit(0)
	return true
}
