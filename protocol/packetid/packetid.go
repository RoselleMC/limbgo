package packetid

// State is a Minecraft protocol state.
type State string

const (
	StateLogin         State = "login"
	StateConfiguration State = "configuration"
	StatePlay          State = "play"
)

// Direction is a packet direction relative to the server.
type Direction string

const (
	ToClient Direction = "toClient"
	ToServer Direction = "toServer"
)

// Entry is one generated packet ID mapping.
type Entry struct {
	State     State
	Direction Direction
	Name      string
	ID        int32
}

// VersionPackets contains generated packet IDs for one protocol version.
type VersionPackets struct {
	MinecraftVersion string
	Protocol         int32
	Entries          []Entry
}

// Lookup returns packet mappings for protocol.
func Lookup(protocol int32) (VersionPackets, bool) {
	packets, ok := byProtocol[protocol]
	return packets, ok
}

// ID resolves one packet ID.
func ID(protocol int32, state State, direction Direction, name string) (int32, bool) {
	packets, ok := Lookup(protocol)
	if !ok {
		return 0, false
	}
	for _, entry := range packets.Entries {
		if entry.State == state && entry.Direction == direction && entry.Name == name {
			return entry.ID, true
		}
	}
	return 0, false
}
