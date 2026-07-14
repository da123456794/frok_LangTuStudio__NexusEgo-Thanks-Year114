package item_stack_transaction

import (
	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/LangTuStudio/RaaBel/game_control/resources_control"
)

// responseMapping ..
type responseMapping struct {
	mapping resources_control.ItemStackResponseMapping
}

// newResponseMapping ..
func newResponseMapping() *responseMapping {
	return &responseMapping{mapping: make(resources_control.ItemStackResponseMapping)}
}

// bind ..
func (r *responseMapping) bind(
	windowName resources_control.WindowName,
	container protocol.FullContainerName,
) {
	dynamicContainerID, hasDynamicContainerID := container.DynamicContainerID.Value()
	r.mapping[resources_control.ContainerNameKey{
		ContainerID:           container.ContainerID,
		DynamicContainerID:    dynamicContainerID,
		HasDynamicContainerID: hasDynamicContainerID,
	}] = windowName
}
