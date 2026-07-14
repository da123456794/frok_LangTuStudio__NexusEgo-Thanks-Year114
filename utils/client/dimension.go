package client

import (
	"fmt"
	"strings"
)

// WrapCommandInDimension prefixes a command with execute-in when needed.
func (c *Client) WrapCommandInDimension(cmd string) string {
	if c == nil {
		return cmd
	}
	dim := strings.TrimSpace(c.CommandDimension)
	if dim == "" || strings.EqualFold(dim, "overworld") || strings.EqualFold(dim, "minecraft:overworld") {
		return cmd
	}
	trimmed := strings.TrimSpace(cmd)
	if strings.HasPrefix(trimmed, "/") {
		trimmed = strings.TrimPrefix(trimmed, "/")
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "execute in ") {
		return cmd
	}
	return fmt.Sprintf("execute in %s run %s", dim, trimmed)
}
