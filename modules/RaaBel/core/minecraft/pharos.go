/*
PhoenixBuilder specific.
Author: CoozillaX
*/
package minecraft

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
)

// pharosResponse ..
type pharosResponse struct {
	Harbor struct {
		Servers map[string]map[string][][]string `json:"servers"`
		Warning string                           `json:"warning"`
	} `json:"harbor"`
}

// getPharosSpeedUpIP ..
func getPharosSpeedUpIP(addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", errors.New("failed to parse addr")
	}
	respReader, err := http.DefaultClient.Get(fmt.Sprintf("https://dual.pharos.netease.com/api/v1/harbor/mcrealms?ips=%s", addr))
	if err != nil {
		return "", err
	}
	resp := &pharosResponse{}
	if err := json.NewDecoder(respReader.Body).Decode(resp); err != nil {
		return "", err
	}
	if resp.Harbor.Warning != "" {
		return "", errors.New(resp.Harbor.Warning)
	}
	if ports, ok := resp.Harbor.Servers[host]; ok {
		if proxyList, ok := ports[port]; ok && len(proxyList) > 0 {
			randomIndex := rand.Intn(len(proxyList))
			proxyInfo := proxyList[randomIndex]
			if len(proxyInfo) >= 2 {
				speedUpAddr := net.JoinHostPort(proxyInfo[0], proxyInfo[1])
				return speedUpAddr, nil
			}
		}
	}
	return "", errors.New("no speedup nodes found for the given address")
}
