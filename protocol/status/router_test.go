package status

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net"
	"testing"

	"github.com/RoselleMC/limbgo/internal/protocol/wire"
)

func TestRouterStatusPing(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	errCh := make(chan error, 1)
	go func() {
		errCh <- Router{Description: "test limbo"}.ServeConn(context.Background(), serverConn, nil)
	}()

	if err := writeHandshake(clientConn, 767, "localhost", 25565, stateStatus); err != nil {
		t.Fatalf("write handshake: %v", err)
	}
	if err := wire.WritePacket(clientConn, wire.Packet{ID: 0}); err != nil {
		t.Fatalf("write status request: %v", err)
	}

	reader := bufio.NewReader(clientConn)
	responsePacket, err := wire.ReadPacket(reader, 0)
	if err != nil {
		t.Fatalf("read status response: %v", err)
	}
	if responsePacket.ID != 0 {
		t.Fatalf("status response packet id = %d, want 0", responsePacket.ID)
	}

	responseText, err := wire.ReadString(bytes.NewReader(responsePacket.Data), 32767)
	if err != nil {
		t.Fatalf("read response json: %v", err)
	}
	var response statusResponse
	if err := json.Unmarshal([]byte(responseText), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if response.Description.Text != "test limbo" {
		t.Fatalf("description = %q, want %q", response.Description.Text, "test limbo")
	}

	var ping bytes.Buffer
	if err := wire.WriteLong(&ping, 42); err != nil {
		t.Fatalf("write ping payload: %v", err)
	}
	if err := wire.WritePacket(clientConn, wire.Packet{ID: 1, Data: ping.Bytes()}); err != nil {
		t.Fatalf("write ping: %v", err)
	}
	pong, err := wire.ReadPacket(reader, 0)
	if err != nil {
		t.Fatalf("read pong: %v", err)
	}
	got, err := wire.ReadLong(bytes.NewReader(pong.Data))
	if err != nil {
		t.Fatalf("read pong payload: %v", err)
	}
	if got != 42 {
		t.Fatalf("pong payload = %d, want 42", got)
	}

	if err := <-errCh; err != nil {
		t.Fatalf("router error: %v", err)
	}
}

func writeHandshake(conn net.Conn, protocol int32, address string, port uint16, nextState int32) error {
	var data bytes.Buffer
	if err := wire.WriteVarInt(&data, protocol); err != nil {
		return err
	}
	if err := wire.WriteString(&data, address); err != nil {
		return err
	}
	if err := data.WriteByte(byte(port >> 8)); err != nil {
		return err
	}
	if err := data.WriteByte(byte(port)); err != nil {
		return err
	}
	if err := wire.WriteVarInt(&data, nextState); err != nil {
		return err
	}
	return wire.WritePacket(conn, wire.Packet{ID: 0, Data: data.Bytes()})
}
