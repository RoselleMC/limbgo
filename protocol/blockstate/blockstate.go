package blockstate

import (
	"strings"

	"github.com/RoselleMC/limbgo"
)

// DefaultState returns the protocol-specific default block-state ID for state.
func DefaultState(protocol int32, state limbgo.BlockState) (uint32, bool) {
	blocks, ok := byProtocol[protocol]
	if !ok {
		return 0, false
	}
	name := strings.TrimPrefix(state.Name, "minecraft:")
	value, ok := blocks[name]
	return value, ok
}
