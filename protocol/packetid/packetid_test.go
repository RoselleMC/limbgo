package packetid

import "testing"

func TestKeyPacketIDs(t *testing.T) {
	cases := []struct {
		protocol  int32
		state     State
		direction Direction
		name      string
		want      int32
	}{
		{protocol: 47, state: StatePlay, direction: ToClient, name: "map_chunk", want: 0x21},
		{protocol: 340, state: StatePlay, direction: ToClient, name: "map_chunk", want: 0x20},
		{protocol: 766, state: StateConfiguration, direction: ToClient, name: "registry_data", want: 0x07},
		{protocol: 766, state: StateConfiguration, direction: ToClient, name: "finish_configuration", want: 0x03},
		{protocol: 766, state: StateLogin, direction: ToServer, name: "login_acknowledged", want: 0x03},
		{protocol: 766, state: StatePlay, direction: ToClient, name: "chunk_batch_start", want: 0x0d},
		{protocol: 766, state: StatePlay, direction: ToClient, name: "map_chunk", want: 0x27},
		{protocol: 766, state: StatePlay, direction: ToClient, name: "chunk_batch_finished", want: 0x0c},
		{protocol: 767, state: StatePlay, direction: ToClient, name: "map_chunk", want: 0x27},
		{protocol: 768, state: StatePlay, direction: ToClient, name: "map_chunk", want: 0x28},
		{protocol: 768, state: StatePlay, direction: ToClient, name: "position", want: 0x42},
		{protocol: 769, state: StatePlay, direction: ToClient, name: "map_chunk", want: 0x28},
		{protocol: 769, state: StatePlay, direction: ToClient, name: "position", want: 0x42},
		{protocol: 770, state: StatePlay, direction: ToClient, name: "map_chunk", want: 0x27},
		{protocol: 770, state: StatePlay, direction: ToClient, name: "position", want: 0x41},
		{protocol: 771, state: StatePlay, direction: ToClient, name: "map_chunk", want: 0x27},
		{protocol: 772, state: StatePlay, direction: ToClient, name: "map_chunk", want: 0x27},
		{protocol: 773, state: StatePlay, direction: ToClient, name: "map_chunk", want: 0x2c},
		{protocol: 773, state: StatePlay, direction: ToClient, name: "position", want: 0x46},
		{protocol: 774, state: StatePlay, direction: ToClient, name: "map_chunk", want: 0x2c},
		{protocol: 774, state: StatePlay, direction: ToClient, name: "position", want: 0x46},
		{protocol: 774, state: StatePlay, direction: ToClient, name: "chunk_batch_start", want: 0x0c},
		{protocol: 774, state: StatePlay, direction: ToClient, name: "chunk_batch_finished", want: 0x0b},
	}

	for _, tt := range cases {
		got, ok := ID(tt.protocol, tt.state, tt.direction, tt.name)
		if !ok {
			t.Fatalf("missing packet id for protocol=%d state=%s direction=%s name=%s", tt.protocol, tt.state, tt.direction, tt.name)
		}
		if got != tt.want {
			t.Fatalf("packet id for protocol=%d state=%s direction=%s name=%s = %#x, want %#x", tt.protocol, tt.state, tt.direction, tt.name, got, tt.want)
		}
	}
}

func TestUnknownProtocol(t *testing.T) {
	if _, ok := Lookup(-1); ok {
		t.Fatal("unexpected packet mapping for unknown protocol")
	}
}
