package blockstate

import (
	"strings"

	"github.com/RoselleMC/limbgo"
)

// DefaultState returns the protocol-specific default block-state ID for state.
func DefaultState(protocol int32, state limbgo.BlockState) (uint32, bool) {
	blocks, ok := byProtocol[protocol]
	if !ok {
		alias, aliasOK := aliasProtocol(protocol)
		if !aliasOK {
			return 0, false
		}
		blocks, ok = byProtocol[alias]
	}
	if !ok {
		return 0, false
	}
	name := strings.TrimPrefix(state.Name, "minecraft:")
	value, ok := blocks[name]
	return value, ok
}

func aliasProtocol(protocol int32) (int32, bool) {
	switch protocol {
	case 775:
		return 774, true
	default:
		return 0, false
	}
}
