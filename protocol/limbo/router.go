package limbo

import (
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strings"

	"github.com/RoselleMC/limbgo"
	"github.com/RoselleMC/limbgo/internal/protocol/wire"
	"github.com/RoselleMC/limbgo/protocol/packetid"
	"github.com/RoselleMC/limbgo/protocol/registrydata"
)

const (
	stateStatus = 1
	stateLogin  = 2
)

// Router is the main Minecraft limbo protocol router.
//
// Status is protocol-neutral. Play support is intentionally version-adapted:
// legacy adapters cover protocol 47 (Minecraft 1.8.x) and protocol 340
// (Minecraft 1.12.2), while modern adapters are selected from the configured
// ModernProtocols table.
type Router struct {
	Description     string
	MaxPlayers      int
	ModernProtocols *ModernProtocols
	RegistryData    *registrydata.Data
}

// ServeConn implements limbgo.ProtocolRouter.
func (r Router) ServeConn(ctx context.Context, conn net.Conn, services limbgo.SessionServices) error {
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
		return r.serveLogin(ctx, conn, reader, services, info)
	default:
		return fmt.Errorf("unknown handshake next state %d", info.NextState)
	}
}

func (r Router) serveLogin(ctx context.Context, conn net.Conn, reader *bufio.Reader, services limbgo.SessionServices, info handshakeInfo) error {
	loginStart, err := wire.ReadPacket(reader, 0)
	if err != nil {
		return err
	}

	loginPacketProtocol := info.ProtocolVersion
	var cfg modernProtocolConfig
	hasModernConfig := false
	if info.ProtocolVersion != protocol47 && info.ProtocolVersion != protocol340 {
		modernProtocols, err := r.modernProtocols()
		if err != nil {
			return err
		}
		if loaded, ok := modernProtocols.configFor(info.ProtocolVersion); ok {
			cfg = loaded
			hasModernConfig = true
			loginPacketProtocol = cfg.packetProtocol()
		}
	}

	loginStartID, ok := packetid.ID(loginPacketProtocol, packetid.StateLogin, packetid.ToServer, "login_start")
	if !ok || loginStart.ID != loginStartID {
		return fmt.Errorf("expected login_start packet %d, got %d", loginStartID, loginStart.ID)
	}

	username, err := readLoginStartUsername(loginStart.Data)
	if err != nil {
		return err
	}
	player := limbgo.Player{
		Name:            username,
		UUID:            offlineUUID(username),
		ProtocolVersion: int(info.ProtocolVersion),
		RemoteAddr:      conn.RemoteAddr(),
		Properties:      map[string]string{},
	}

	switch info.ProtocolVersion {
	case protocol47:
		return serveProtocol47(ctx, conn, services, player)
	case protocol340:
		return serveProtocol340(ctx, conn, services, player)
	default:
		if hasModernConfig {
			registryData, err := r.registryData()
			if err != nil {
				return err
			}
			if cfg.preConfiguration {
				return serveModernPreConfigurationProtocol(ctx, conn, services, player, cfg, registryData)
			}
			return serveModernProtocol(ctx, conn, reader, services, player, cfg, registryData)
		}
		return writeLoginDisconnect(conn, info.ProtocolVersion, "limbgo play support currently implements protocols "+r.supportedPlayProtocols())
	}
}

func (r Router) modernProtocols() (*ModernProtocols, error) {
	if r.ModernProtocols != nil {
		return r.ModernProtocols, nil
	}
	return DefaultModernProtocols()
}

func (r Router) registryData() (*registrydata.Data, error) {
	if r.RegistryData != nil {
		return r.RegistryData, nil
	}
	return registrydata.Default()
}

func (r Router) supportedPlayProtocols() string {
	modernProtocols, err := r.modernProtocols()
	if err != nil {
		return "47 and 340"
	}
	protocols := append([]int32{protocol47, protocol340}, modernProtocols.supportedProtocols()...)
	sort.Slice(protocols, func(i, j int) bool {
		return protocols[i] < protocols[j]
	})
	return formatProtocolRanges(protocols)
}

func formatProtocolRanges(protocols []int32) string {
	if len(protocols) == 0 {
		return ""
	}
	var parts []string
	start := protocols[0]
	prev := protocols[0]
	for _, protocol := range protocols[1:] {
		if protocol == prev || protocol == prev+1 {
			prev = protocol
			continue
		}
		parts = append(parts, formatProtocolRange(start, prev))
		start = protocol
		prev = protocol
	}
	parts = append(parts, formatProtocolRange(start, prev))
	return strings.Join(parts, ", ")
}

func formatProtocolRange(start, end int32) string {
	if start == end {
		return fmt.Sprintf("%d", start)
	}
	return fmt.Sprintf("%d-%d", start, end)
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

func readLoginStartUsername(data []byte) (string, error) {
	username, err := wire.ReadString(bytes.NewReader(data), 16)
	if err != nil {
		return "", err
	}
	if username == "" {
		return "", fmt.Errorf("empty username")
	}
	return username, nil
}

func writeLoginDisconnect(conn net.Conn, protocol int32, message string) error {
	id, ok := packetid.ID(protocol, packetid.StateLogin, packetid.ToClient, "disconnect")
	if !ok {
		id = 0
	}
	payload, err := json.Marshal(textComponent{Text: message})
	if err != nil {
		return err
	}
	var data bytes.Buffer
	if err := wire.WriteString(&data, string(payload)); err != nil {
		return err
	}
	return wire.WritePacket(conn, wire.Packet{ID: id, Data: data.Bytes()})
}

func offlineUUID(username string) string {
	sum := md5.Sum([]byte("OfflinePlayer:" + username))
	sum[6] = (sum[6] & 0x0f) | 0x30
	sum[8] = (sum[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		sum[0:4],
		sum[4:6],
		sum[6:8],
		sum[8:10],
		sum[10:16],
	)
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
