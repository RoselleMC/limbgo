package status

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"

	"github.com/RoselleMC/limbgo"
	"github.com/RoselleMC/limbgo/internal/protocol/wire"
)

const (
	stateStatus = 1
	stateLogin  = 2
)

// Router implements the stable handshake and server-list status path.
//
// Login is deliberately rejected until the generated play-state adapters are
// implemented. This makes the standalone binary observable without pretending to
// support chunk rendering yet.
type Router struct {
	Description string
	MaxPlayers  int
}

// ServeConn implements limbgo.ProtocolRouter.
func (r Router) ServeConn(_ context.Context, conn net.Conn, _ limbgo.SessionServices) error {
	reader := bufio.NewReader(conn)
	handshake, err := wire.ReadPacket(reader, 0)
	if err != nil {
		return err
	}
	if handshake.ID != 0 {
		return fmt.Errorf("expected handshake packet 0, got %d", handshake.ID)
	}

	info, err := readHandshake(handshake.Data)
	if err != nil {
		return err
	}

	switch info.NextState {
	case stateStatus:
		return r.serveStatus(conn, reader, info.ProtocolVersion)
	case stateLogin:
		return writeLoginDisconnect(conn, "limbgo play-state protocol adapters are not implemented yet")
	default:
		return fmt.Errorf("unknown handshake next state %d", info.NextState)
	}
}

func (r Router) serveStatus(conn net.Conn, reader *bufio.Reader, protocol int32) error {
	req, err := wire.ReadPacket(reader, 0)
	if err != nil {
		return err
	}
	if req.ID != 0 {
		return fmt.Errorf("expected status request packet 0, got %d", req.ID)
	}

	description := r.Description
	if description == "" {
		description = "limbgo"
	}
	maxPlayers := r.MaxPlayers
	if maxPlayers <= 0 {
		maxPlayers = 1
	}

	payload, err := json.Marshal(statusResponse{
		Version: statusVersion{Name: "limbgo", Protocol: protocol},
		Players: statusPlayers{
			Max:    maxPlayers,
			Online: 0,
		},
		Description: textComponent{Text: description},
	})
	if err != nil {
		return err
	}

	var data bytes.Buffer
	if err := wire.WriteString(&data, string(payload)); err != nil {
		return err
	}
	if err := wire.WritePacket(conn, wire.Packet{ID: 0, Data: data.Bytes()}); err != nil {
		return err
	}

	ping, err := wire.ReadPacket(reader, 0)
	if err != nil {
		return err
	}
	if ping.ID != 1 {
		return fmt.Errorf("expected ping packet 1, got %d", ping.ID)
	}
	return wire.WritePacket(conn, wire.Packet{ID: 1, Data: ping.Data})
}

type handshakeInfo struct {
	ProtocolVersion int32
	Address         string
	Port            uint16
	NextState       int32
}

func readHandshake(data []byte) (handshakeInfo, error) {
	body := bytes.NewReader(data)
	protocol, err := wire.ReadVarInt(body)
	if err != nil {
		return handshakeInfo{}, err
	}
	address, err := wire.ReadString(body, 255)
	if err != nil {
		return handshakeInfo{}, err
	}
	port, err := wire.ReadUnsignedShort(body)
	if err != nil {
		return handshakeInfo{}, err
	}
	nextState, err := wire.ReadVarInt(body)
	if err != nil {
		return handshakeInfo{}, err
	}
	return handshakeInfo{
		ProtocolVersion: protocol,
		Address:         address,
		Port:            port,
		NextState:       nextState,
	}, nil
}

func writeLoginDisconnect(conn net.Conn, message string) error {
	payload, err := json.Marshal(textComponent{Text: message})
	if err != nil {
		return err
	}
	var data bytes.Buffer
	if err := wire.WriteString(&data, string(payload)); err != nil {
		return err
	}
	return wire.WritePacket(conn, wire.Packet{ID: 0, Data: data.Bytes()})
}

type statusResponse struct {
	Version     statusVersion `json:"version"`
	Players     statusPlayers `json:"players"`
	Description textComponent `json:"description"`
}

type statusVersion struct {
	Name     string `json:"name"`
	Protocol int32  `json:"protocol"`
}

type statusPlayers struct {
	Max    int `json:"max"`
	Online int `json:"online"`
}

type textComponent struct {
	Text string `json:"text"`
}
