package limbo

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/RoselleMC/limbgo"
	"github.com/RoselleMC/limbgo/dialog"
	"github.com/RoselleMC/limbgo/internal/protocol/wire"
	"github.com/RoselleMC/limbgo/protocol/packetid"
	"go.minekube.com/common/minecraft/component"
)

func TestProtocol47LoginAndChunk(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	services := testServices{
		spawn: limbgo.SpawnTarget{
			World:    "spawn",
			Position: limbgo.Vec3{X: 0, Y: 64, Z: 0},
			GameMode: limbgo.GameModeAdventure,
		},
		world: testWorld(),
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- Router{Description: "limbgo test"}.ServeConn(context.Background(), serverConn, services)
	}()

	if err := writeHandshake(clientConn, protocol47, "localhost", 25565, stateLogin); err != nil {
		t.Fatalf("write handshake: %v", err)
	}
	var loginStart bytes.Buffer
	if err := wire.WriteString(&loginStart, "TestPlayer"); err != nil {
		t.Fatalf("write username: %v", err)
	}
	if err := wire.WritePacket(clientConn, wire.Packet{ID: 0, Data: loginStart.Bytes()}); err != nil {
		t.Fatalf("write login_start: %v", err)
	}

	reader := bufio.NewReader(clientConn)
	assertPacketID(t, reader, protocol47, packetid.StateLogin, "success")
	assertPacketID(t, reader, protocol47, packetid.StatePlay, "login")
	assertPacketID(t, reader, protocol47, packetid.StatePlay, "spawn_position")
	assertPacketID(t, reader, protocol47, packetid.StatePlay, "position")
	chunkPacket := assertPacketID(t, reader, protocol47, packetid.StatePlay, "map_chunk")
	assertFirstChunkBlock47(t, chunkPacket.Data, 1<<4)

	if err := <-errCh; err != nil {
		t.Fatalf("router error: %v", err)
	}
}

func TestProtocol340LoginAndChunk(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	services := testServices{
		spawn: limbgo.SpawnTarget{
			World:    "spawn",
			Position: limbgo.Vec3{X: 0, Y: 64, Z: 0},
			GameMode: limbgo.GameModeAdventure,
		},
		world: testWorld(),
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- Router{Description: "limbgo test"}.ServeConn(context.Background(), serverConn, services)
	}()

	if err := writeHandshake(clientConn, protocol340, "localhost", 25565, stateLogin); err != nil {
		t.Fatalf("write handshake: %v", err)
	}
	var loginStart bytes.Buffer
	if err := wire.WriteString(&loginStart, "TestPlayer"); err != nil {
		t.Fatalf("write username: %v", err)
	}
	if err := wire.WritePacket(clientConn, wire.Packet{ID: 0, Data: loginStart.Bytes()}); err != nil {
		t.Fatalf("write login_start: %v", err)
	}

	reader := bufio.NewReader(clientConn)
	assertPacketID(t, reader, protocol340, packetid.StateLogin, "success")
	assertPacketID(t, reader, protocol340, packetid.StatePlay, "login")
	assertPacketID(t, reader, protocol340, packetid.StatePlay, "spawn_position")
	assertPacketID(t, reader, protocol340, packetid.StatePlay, "position")
	chunkPacket := assertPacketID(t, reader, protocol340, packetid.StatePlay, "map_chunk")
	assertFirstChunkBlock340(t, chunkPacket.Data, 1<<4)

	if err := <-errCh; err != nil {
		t.Fatalf("router error: %v", err)
	}
}

func TestProtocol340CommandEvent(t *testing.T) {
	serverConn, clientConn := net.Pipe()

	got := make(chan string, 1)
	services := testServices{
		spawn: limbgo.SpawnTarget{
			World:    "spawn",
			Position: limbgo.Vec3{X: 0, Y: 64, Z: 0},
			GameMode: limbgo.GameModeAdventure,
		},
		world: testWorld(),
		events: limbgo.PlayerEventHandlerFuncs{
			Command: func(_ context.Context, _ limbgo.PlayerSession, event *limbgo.CommandEvent) error {
				got <- event.Command
				return nil
			},
		},
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- Router{Description: "limbgo test"}.ServeConn(context.Background(), serverConn, services)
	}()

	loginProtocol(t, clientConn, protocol340, false)
	reader := bufio.NewReader(clientConn)
	assertPacketID(t, reader, protocol340, packetid.StateLogin, "success")
	assertPacketID(t, reader, protocol340, packetid.StatePlay, "login")
	assertPacketID(t, reader, protocol340, packetid.StatePlay, "spawn_position")
	assertPacketID(t, reader, protocol340, packetid.StatePlay, "position")
	assertPacketID(t, reader, protocol340, packetid.StatePlay, "map_chunk")

	var command bytes.Buffer
	if err := wire.WriteString(&command, "/hub"); err != nil {
		t.Fatalf("write command: %v", err)
	}
	writeServerboundNamedPacket(t, clientConn, protocol340, packetid.StatePlay, "chat", command.Bytes())

	if command := <-got; command != "hub" {
		t.Fatalf("command event = %q, want hub", command)
	}
	_ = clientConn.Close()
	if err := <-errCh; err != nil {
		t.Fatalf("router error: %v", err)
	}
}

func TestProtocol340JoinResolverCanReturnWorldInstance(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	world := testWorld()
	world.WorldID = "player-world"
	server, err := limbgo.NewServer(limbgo.Config{
		ProtocolRouter: Router{Description: "limbgo test"},
		JoinResolver: limbgo.JoinResolverFunc(func(_ context.Context, player limbgo.Player) (limbgo.JoinTarget, error) {
			if player.Name != "TestPlayer" {
				t.Fatalf("join resolver player = %q, want TestPlayer", player.Name)
			}
			return limbgo.JoinTarget{
				World: world,
				Spawn: limbgo.SpawnTarget{
					Position: limbgo.Vec3{X: 0, Y: 64, Z: 0},
					GameMode: limbgo.GameModeAdventure,
				},
			}, nil
		}),
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- Router{Description: "limbgo test"}.ServeConn(context.Background(), serverConn, server)
	}()

	loginProtocol(t, clientConn, protocol340, false)
	reader := bufio.NewReader(clientConn)
	assertPacketID(t, reader, protocol340, packetid.StateLogin, "success")
	assertPacketID(t, reader, protocol340, packetid.StatePlay, "login")
	assertPacketID(t, reader, protocol340, packetid.StatePlay, "spawn_position")
	assertPacketID(t, reader, protocol340, packetid.StatePlay, "position")
	chunkPacket := assertPacketID(t, reader, protocol340, packetid.StatePlay, "map_chunk")
	assertFirstChunkBlock340(t, chunkPacket.Data, 1<<4)

	if err := <-errCh; err != nil {
		t.Fatalf("router error: %v", err)
	}
}

func TestProtocol757LoginAndChunk(t *testing.T) {
	testModernPreConfigurationLoginAndChunk(t, protocol757)
}

func TestProtocol758LoginAndChunk(t *testing.T) {
	testModernPreConfigurationLoginAndChunk(t, protocol758)
}

func TestProtocol759LoginAndChunk(t *testing.T) {
	testModernPreConfigurationLoginAndChunk(t, protocol759)
}

func TestProtocol760LoginAndChunk(t *testing.T) {
	testModernPreConfigurationLoginAndChunk(t, protocol760)
}

func TestProtocol761LoginAndChunk(t *testing.T) {
	testModernPreConfigurationLoginAndChunk(t, protocol761)
}

func TestProtocol762LoginAndChunk(t *testing.T) {
	testModernPreConfigurationLoginAndChunk(t, protocol762)
}

func TestProtocol763LoginAndChunk(t *testing.T) {
	testModernPreConfigurationLoginAndChunk(t, protocol763)
}

func testModernPreConfigurationLoginAndChunk(t *testing.T, protocol int32) {
	t.Helper()
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	services := testServices{
		spawn: limbgo.SpawnTarget{
			World:    "spawn",
			Position: limbgo.Vec3{X: 0, Y: 64, Z: 0},
			GameMode: limbgo.GameModeAdventure,
		},
		world: testWorld(),
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- Router{Description: "limbgo test"}.ServeConn(context.Background(), serverConn, services)
	}()

	if err := writeHandshake(clientConn, protocol, "localhost", 25565, stateLogin); err != nil {
		t.Fatalf("write handshake: %v", err)
	}
	var loginStart bytes.Buffer
	if err := wire.WriteString(&loginStart, "TestPlayer"); err != nil {
		t.Fatalf("write username: %v", err)
	}
	if err := wire.WriteBool(&loginStart, false); err != nil {
		t.Fatalf("write uuid option: %v", err)
	}
	if err := wire.WritePacket(clientConn, wire.Packet{ID: 0, Data: loginStart.Bytes()}); err != nil {
		t.Fatalf("write login_start: %v", err)
	}

	reader := bufio.NewReader(clientConn)
	assertPacketID(t, reader, protocol, packetid.StateLogin, "success")
	assertPacketID(t, reader, protocol, packetid.StatePlay, "login")
	assertPacketID(t, reader, protocol, packetid.StatePlay, "position")
	chunkPacket := assertPacketID(t, reader, protocol, packetid.StatePlay, "map_chunk")
	assertFirstChunkBlockModern(t, chunkPacket.Data, false, true, 1)

	if err := <-errCh; err != nil {
		t.Fatalf("router error: %v", err)
	}
}

func TestProtocol764LoginConfigurationAndChunk(t *testing.T) {
	testModernLoginConfigurationAndChunk(t, protocol764)
}

func TestProtocol765LoginConfigurationAndChunk(t *testing.T) {
	testModernLoginConfigurationAndChunk(t, protocol765)
}

func TestProtocol766LoginConfigurationAndChunk(t *testing.T) {
	testModernLoginConfigurationAndChunk(t, protocol766)
}

func TestProtocol767LoginConfigurationAndChunk(t *testing.T) {
	testModernLoginConfigurationAndChunk(t, protocol767)
}

func TestProtocol768LoginConfigurationAndChunk(t *testing.T) {
	testModernLoginConfigurationAndChunk(t, protocol768)
}

func TestProtocol769LoginConfigurationAndChunk(t *testing.T) {
	testModernLoginConfigurationAndChunk(t, protocol769)
}

func TestProtocol770LoginConfigurationAndChunk(t *testing.T) {
	testModernLoginConfigurationAndChunk(t, protocol770)
}

func TestProtocol771LoginConfigurationAndChunk(t *testing.T) {
	testModernLoginConfigurationAndChunk(t, protocol771)
}

func TestProtocol772LoginConfigurationAndChunk(t *testing.T) {
	testModernLoginConfigurationAndChunk(t, protocol772)
}

func TestProtocol773LoginConfigurationAndChunk(t *testing.T) {
	testModernLoginConfigurationAndChunk(t, protocol773)
}

func TestProtocol774LoginConfigurationAndChunk(t *testing.T) {
	testModernLoginConfigurationAndChunk(t, protocol774)
}

func TestProtocol775LoginConfigurationAliasAndChunk(t *testing.T) {
	testModernLoginConfigurationAndChunkWithPacketProtocol(t, protocol775, protocol774)
}

func TestProtocol774ChatEventCanSendRichSystemMessage(t *testing.T) {
	serverConn, clientConn := net.Pipe()

	got := make(chan string, 1)
	services := testServices{
		spawn: limbgo.SpawnTarget{
			World:    "spawn",
			Position: limbgo.Vec3{X: 0, Y: 64, Z: 0},
			GameMode: limbgo.GameModeAdventure,
		},
		world: testWorld(),
		events: limbgo.PlayerEventHandlerFuncs{
			Chat: func(ctx context.Context, session limbgo.PlayerSession, event *limbgo.ChatEvent) error {
				got <- event.Message
				return session.SendMessage(ctx, &component.Text{
					Content: "accepted",
					Extra: []component.Component{
						&component.Text{Content: " rich"},
					},
				})
			},
		},
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- Router{Description: "limbgo test"}.ServeConn(context.Background(), serverConn, services)
	}()

	loginProtocol(t, clientConn, protocol774, false)
	reader := bufio.NewReader(clientConn)
	assertPacketID(t, reader, protocol774, packetid.StateLogin, "success")
	writeServerboundNamedPacket(t, clientConn, protocol774, packetid.StateLogin, "login_acknowledged", nil)
	for i := 0; i < 4; i++ {
		assertPacketID(t, reader, protocol774, packetid.StateConfiguration, "registry_data")
	}
	assertPacketID(t, reader, protocol774, packetid.StateConfiguration, "tags")
	assertPacketID(t, reader, protocol774, packetid.StateConfiguration, "finish_configuration")
	writeServerboundNamedPacket(t, clientConn, protocol774, packetid.StateConfiguration, "finish_configuration", nil)
	assertPacketID(t, reader, protocol774, packetid.StatePlay, "login")
	assertPacketID(t, reader, protocol774, packetid.StatePlay, "position")
	assertPacketID(t, reader, protocol774, packetid.StatePlay, "chunk_batch_start")
	assertPacketID(t, reader, protocol774, packetid.StatePlay, "map_chunk")
	assertPacketID(t, reader, protocol774, packetid.StatePlay, "chunk_batch_finished")

	var message bytes.Buffer
	if err := wire.WriteString(&message, "hello"); err != nil {
		t.Fatalf("write chat message: %v", err)
	}
	writeServerboundNamedPacket(t, clientConn, protocol774, packetid.StatePlay, "chat_message", message.Bytes())
	if message := <-got; message != "hello" {
		t.Fatalf("chat event = %q, want hello", message)
	}
	assertPacketID(t, reader, protocol774, packetid.StatePlay, "system_chat")

	_ = clientConn.Close()
	if err := <-errCh; err != nil {
		t.Fatalf("router error: %v", err)
	}
}

func TestProtocol774DialogAPIAndClickEvent(t *testing.T) {
	serverConn, clientConn := net.Pipe()

	gotClick := make(chan limbgo.DialogClickEvent, 1)
	services := testServices{
		spawn: limbgo.SpawnTarget{
			World:    "spawn",
			Position: limbgo.Vec3{X: 0, Y: 64, Z: 0},
			GameMode: limbgo.GameModeAdventure,
		},
		world: testWorld(),
		events: limbgo.PlayerEventHandlerFuncs{
			Chat: func(ctx context.Context, session limbgo.PlayerSession, event *limbgo.ChatEvent) error {
				if err := session.ShowDialog(ctx, dialog.Notice(dialog.Common{
					Title: &component.Text{
						Content: "Welcome",
						Extra: []component.Component{
							&component.Text{Content: " rich"},
						},
					},
					Body: []dialog.Raw{
						dialog.PlainMessage(&component.Text{Content: "Choose an action"}, 220),
					},
					Inputs: []dialog.Raw{
						dialog.TextInput("name", &component.Text{Content: "Name"}, dialog.TextInputOptions{
							Initial:   "Steve",
							MaxLength: 32,
						}),
						dialog.NumberRangeInput("level", &component.Text{Content: "Level"}, dialog.NumberRangeOptions{
							Start:   1,
							End:     10,
							Initial: dialog.Float(4.5),
							Step:    dialog.Float(0.5),
						}),
					},
					CanCloseWithEscape: dialog.Bool(true),
					Pause:              dialog.Bool(false),
					AfterAction:        dialog.AfterActionWaitForResponse,
				}, dialog.ActionButton{
					Label:   &component.Text{Content: "Submit"},
					Tooltip: &component.Text{Content: "Send rich payload"},
					Action:  dialog.DynamicCustom("limbgo:submit", dialog.Raw{"source": "test"}),
				})); err != nil {
					return err
				}
				return session.ClearDialog(ctx)
			},
			DialogClick: func(_ context.Context, _ limbgo.PlayerSession, event *limbgo.DialogClickEvent) error {
				gotClick <- *event
				return nil
			},
		},
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- Router{Description: "limbgo test"}.ServeConn(context.Background(), serverConn, services)
	}()

	loginProtocol(t, clientConn, protocol774, false)
	reader := bufio.NewReader(clientConn)
	assertPacketID(t, reader, protocol774, packetid.StateLogin, "success")
	writeServerboundNamedPacket(t, clientConn, protocol774, packetid.StateLogin, "login_acknowledged", nil)
	for i := 0; i < 4; i++ {
		assertPacketID(t, reader, protocol774, packetid.StateConfiguration, "registry_data")
	}
	assertPacketID(t, reader, protocol774, packetid.StateConfiguration, "tags")
	assertPacketID(t, reader, protocol774, packetid.StateConfiguration, "finish_configuration")
	writeServerboundNamedPacket(t, clientConn, protocol774, packetid.StateConfiguration, "finish_configuration", nil)
	assertPacketID(t, reader, protocol774, packetid.StatePlay, "login")
	assertPacketID(t, reader, protocol774, packetid.StatePlay, "position")
	assertPacketID(t, reader, protocol774, packetid.StatePlay, "chunk_batch_start")
	assertPacketID(t, reader, protocol774, packetid.StatePlay, "map_chunk")
	assertPacketID(t, reader, protocol774, packetid.StatePlay, "chunk_batch_finished")

	var message bytes.Buffer
	if err := wire.WriteString(&message, "open"); err != nil {
		t.Fatalf("write chat message: %v", err)
	}
	writeServerboundNamedPacket(t, clientConn, protocol774, packetid.StatePlay, "chat_message", message.Bytes())
	dialogPacket := assertPacketID(t, reader, protocol774, packetid.StatePlay, "show_dialog")
	assertInlineDialogNBT(t, dialogPacket.Data)
	assertPacketID(t, reader, protocol774, packetid.StatePlay, "clear_dialog")

	var click bytes.Buffer
	if err := wire.WriteString(&click, "limbgo:submit"); err != nil {
		t.Fatalf("write custom click id: %v", err)
	}
	if err := wire.WriteBool(&click, false); err != nil {
		t.Fatalf("write custom click payload option: %v", err)
	}
	writeServerboundNamedPacket(t, clientConn, protocol774, packetid.StatePlay, "custom_click_action", click.Bytes())
	clickEvent := <-gotClick
	if clickEvent.ID != "limbgo:submit" {
		t.Fatalf("dialog click id = %q, want limbgo:submit", clickEvent.ID)
	}
	if clickEvent.Protocol != int(protocol774) {
		t.Fatalf("dialog click protocol = %d, want %d", clickEvent.Protocol, protocol774)
	}
	if len(clickEvent.Payload) != 0 {
		t.Fatalf("dialog click payload len = %d, want 0", len(clickEvent.Payload))
	}

	_ = clientConn.Close()
	if err := <-errCh; err != nil {
		t.Fatalf("router error: %v", err)
	}
}

func TestProtocol775DialogPacketAlias(t *testing.T) {
	serverConn, clientConn := net.Pipe()

	errCh := make(chan error, 1)
	go func() {
		adapter := playAdapter{protocol: protocol775, packetProtocol: protocol774}
		if err := writeShowDialog(serverConn, adapter, dialog.Notice(dialog.Common{
			Title: dialog.Text("Alias"),
		}, dialog.Button(dialog.Text("OK"), dialog.Custom("limbgo:ok", nil)))); err != nil {
			errCh <- err
			return
		}
		errCh <- writeClearDialog(serverConn, adapter)
	}()

	reader := bufio.NewReader(clientConn)
	dialogPacket := assertPacketID(t, reader, protocol774, packetid.StatePlay, "show_dialog")
	assertInlineDialogNBT(t, dialogPacket.Data)
	assertPacketID(t, reader, protocol774, packetid.StatePlay, "clear_dialog")
	_ = clientConn.Close()
	if err := <-errCh; err != nil {
		t.Fatalf("dialog alias write: %v", err)
	}
}

func TestProtocol774ConfiguredWorldTime(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	world := testWorld()
	timeOfDay := int64(18000)
	world.WorldDimension = limbgo.DimensionPreset(limbgo.DimensionNether, 256)
	world.WorldDimension.TimeOfDay = &timeOfDay
	world.WorldDimension.WorldAge = 42
	services := testServices{
		spawn: limbgo.SpawnTarget{
			World:    "spawn",
			Position: limbgo.Vec3{X: 0, Y: 64, Z: 0},
			GameMode: limbgo.GameModeAdventure,
		},
		world: world,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- Router{Description: "limbgo test"}.ServeConn(context.Background(), serverConn, services)
	}()

	loginProtocol(t, clientConn, protocol774, false)
	reader := bufio.NewReader(clientConn)
	assertPacketID(t, reader, protocol774, packetid.StateLogin, "success")
	writeServerboundNamedPacket(t, clientConn, protocol774, packetid.StateLogin, "login_acknowledged", nil)
	for i := 0; i < 4; i++ {
		assertPacketID(t, reader, protocol774, packetid.StateConfiguration, "registry_data")
	}
	assertPacketID(t, reader, protocol774, packetid.StateConfiguration, "tags")
	assertPacketID(t, reader, protocol774, packetid.StateConfiguration, "finish_configuration")
	writeServerboundNamedPacket(t, clientConn, protocol774, packetid.StateConfiguration, "finish_configuration", nil)
	assertPacketID(t, reader, protocol774, packetid.StatePlay, "login")
	assertPacketID(t, reader, protocol774, packetid.StatePlay, "position")
	timePacket := assertPacketID(t, reader, protocol774, packetid.StatePlay, "update_time")
	assertUpdateTime(t, timePacket.Data, 42, 18000)
	assertPacketID(t, reader, protocol774, packetid.StatePlay, "chunk_batch_start")
	assertPacketID(t, reader, protocol774, packetid.StatePlay, "map_chunk")
	assertPacketID(t, reader, protocol774, packetid.StatePlay, "chunk_batch_finished")

	if err := <-errCh; err != nil {
		t.Fatalf("router error: %v", err)
	}
}

func TestLoadModernProtocolsBytesAndSupportedList(t *testing.T) {
	protocols, err := LoadModernProtocolsBytes([]byte(`{"999":{"packet_id_protocol":774,"data_protocol":774,"position_v2":true}}`))
	if err != nil {
		t.Fatalf("load modern protocols: %v", err)
	}
	cfg, ok := protocols.configFor(999)
	if !ok {
		t.Fatalf("protocol 999 not loaded")
	}
	if !cfg.positionV2 {
		t.Fatalf("protocol 999 position_v2 not applied")
	}
	if cfg.packetProtocol() != protocol774 {
		t.Fatalf("protocol 999 packet protocol = %d, want %d", cfg.packetProtocol(), protocol774)
	}
	if cfg.dataProtocolID() != protocol774 {
		t.Fatalf("protocol 999 data protocol = %d, want %d", cfg.dataProtocolID(), protocol774)
	}
	got := Router{ModernProtocols: protocols}.supportedPlayProtocols()
	for _, want := range []string{"47", "340", "999"} {
		if !strings.Contains(got, want) {
			t.Fatalf("supported protocols %q missing %s", got, want)
		}
	}
}

func testModernLoginConfigurationAndChunk(t *testing.T, protocol int32) {
	t.Helper()
	testModernLoginConfigurationAndChunkWithPacketProtocol(t, protocol, protocol)
}

func testModernLoginConfigurationAndChunkWithPacketProtocol(t *testing.T, protocol int32, packetProtocol int32) {
	t.Helper()
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	services := testServices{
		spawn: limbgo.SpawnTarget{
			World:    "spawn",
			Position: limbgo.Vec3{X: 0, Y: 64, Z: 0},
			GameMode: limbgo.GameModeAdventure,
		},
		world: testWorld(),
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- Router{Description: "limbgo test"}.ServeConn(context.Background(), serverConn, services)
	}()

	if err := writeHandshake(clientConn, protocol, "localhost", 25565, stateLogin); err != nil {
		t.Fatalf("write handshake: %v", err)
	}
	var loginStart bytes.Buffer
	if err := wire.WriteString(&loginStart, "TestPlayer"); err != nil {
		t.Fatalf("write username: %v", err)
	}
	if err := wire.WritePacket(clientConn, wire.Packet{ID: 0, Data: loginStart.Bytes()}); err != nil {
		t.Fatalf("write login_start: %v", err)
	}

	reader := bufio.NewReader(clientConn)
	assertPacketID(t, reader, packetProtocol, packetid.StateLogin, "success")
	writeServerboundNamedPacket(t, clientConn, packetProtocol, packetid.StateLogin, "login_acknowledged", nil)

	registryPacketCount := 4
	if protocol < protocol766 {
		registryPacketCount = 1
	}
	for i := 0; i < registryPacketCount; i++ {
		assertPacketID(t, reader, packetProtocol, packetid.StateConfiguration, "registry_data")
	}
	assertPacketID(t, reader, packetProtocol, packetid.StateConfiguration, "tags")
	assertPacketID(t, reader, packetProtocol, packetid.StateConfiguration, "finish_configuration")
	writeServerboundNamedPacket(t, clientConn, packetProtocol, packetid.StateConfiguration, "finish_configuration", nil)

	assertPacketID(t, reader, packetProtocol, packetid.StatePlay, "login")
	assertPacketID(t, reader, packetProtocol, packetid.StatePlay, "position")
	assertPacketID(t, reader, packetProtocol, packetid.StatePlay, "chunk_batch_start")
	chunkPacket := assertPacketID(t, reader, packetProtocol, packetid.StatePlay, "map_chunk")
	assertFirstChunkBlockModern(t, chunkPacket.Data, protocol >= protocol770, false, 1)
	assertPacketID(t, reader, packetProtocol, packetid.StatePlay, "chunk_batch_finished")

	if err := <-errCh; err != nil {
		t.Fatalf("router error: %v", err)
	}
}

func TestProtocol47StatusStillWorks(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	errCh := make(chan error, 1)
	go func() {
		errCh <- Router{Description: "limbgo test"}.ServeConn(context.Background(), serverConn, nil)
	}()

	if err := writeHandshake(clientConn, protocol47, "localhost", 25565, stateStatus); err != nil {
		t.Fatalf("write handshake: %v", err)
	}
	if err := wire.WritePacket(clientConn, wire.Packet{ID: 0}); err != nil {
		t.Fatalf("write status request: %v", err)
	}

	reader := bufio.NewReader(clientConn)
	response, err := wire.ReadPacket(reader, 0)
	if err != nil {
		t.Fatalf("read status response: %v", err)
	}
	if response.ID != 0 {
		t.Fatalf("status response id = %d, want 0", response.ID)
	}
	var ping bytes.Buffer
	if err := wire.WriteLong(&ping, 7); err != nil {
		t.Fatalf("write ping payload: %v", err)
	}
	if err := wire.WritePacket(clientConn, wire.Packet{ID: 1, Data: ping.Bytes()}); err != nil {
		t.Fatalf("write ping: %v", err)
	}
	pong, err := wire.ReadPacket(reader, 0)
	if err != nil {
		t.Fatalf("read pong: %v", err)
	}
	if pong.ID != 1 {
		t.Fatalf("pong id = %d, want 1", pong.ID)
	}

	if err := <-errCh; err != nil {
		t.Fatalf("router error: %v", err)
	}
}

func TestProtocol774SessionControlAPI(t *testing.T) {
	serverConn, clientConn := net.Pipe()

	gotCapabilities := make(chan limbgo.SessionCapabilities, 1)
	services := testServices{
		spawn: limbgo.SpawnTarget{
			World:    "spawn",
			Position: limbgo.Vec3{X: 0, Y: 64, Z: 0},
			GameMode: limbgo.GameModeAdventure,
		},
		world: testWorld(),
		events: limbgo.PlayerEventHandlerFuncs{
			Chat: func(ctx context.Context, session limbgo.PlayerSession, event *limbgo.ChatEvent) error {
				gotCapabilities <- session.Capabilities()
				if err := session.StoreCookie(ctx, "authman:transfer", []byte("grant-token")); err != nil {
					return err
				}
				if err := session.Transfer(ctx, "velocity.internal", 25566); err != nil {
					return err
				}
				return session.Disconnect(ctx, &component.Text{Content: "done"})
			},
		},
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- Router{Description: "limbgo test"}.ServeConn(context.Background(), serverConn, services)
	}()

	loginProtocol(t, clientConn, protocol774, false)
	reader := bufio.NewReader(clientConn)
	completeModernJoin(t, clientConn, reader, protocol774, protocol774)

	var message bytes.Buffer
	if err := wire.WriteString(&message, "auth ok"); err != nil {
		t.Fatalf("write chat message: %v", err)
	}
	writeServerboundNamedPacket(t, clientConn, protocol774, packetid.StatePlay, "chat_message", message.Bytes())

	caps := <-gotCapabilities
	if !caps.StoreCookie || !caps.Transfer || !caps.Dialog || !caps.Disconnect || !caps.SystemMessage {
		t.Fatalf("capabilities = %+v", caps)
	}
	cookiePacket := assertPacketID(t, reader, protocol774, packetid.StatePlay, "store_cookie")
	assertStoreCookiePacket(t, cookiePacket.Data, "authman:transfer", []byte("grant-token"))
	transferPacket := assertPacketID(t, reader, protocol774, packetid.StatePlay, "transfer")
	assertTransferPacket(t, transferPacket.Data, "velocity.internal", 25566)
	disconnectPacket := assertPacketID(t, reader, protocol774, packetid.StatePlay, "kick_disconnect")
	if err := skipAnonymousNBT(bytes.NewReader(disconnectPacket.Data)); err != nil {
		t.Fatalf("disconnect reason nbt: %v", err)
	}

	if err := <-errCh; err != nil {
		t.Fatalf("router error: %v", err)
	}
}

func TestProtocol340SessionControlCapabilities(t *testing.T) {
	session := &playSession{adapter: newPlayAdapter(protocol340)}
	caps := session.Capabilities()
	if caps.StoreCookie || caps.Transfer || caps.Dialog {
		t.Fatalf("legacy capabilities = %+v", caps)
	}
	if !caps.Disconnect || !caps.SystemMessage {
		t.Fatalf("legacy capabilities missing baseline support: %+v", caps)
	}
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	session.conn = serverConn
	if err := session.StoreCookie(context.Background(), "authman:transfer", []byte("grant")); !errors.Is(err, limbgo.ErrUnsupportedCapability) {
		t.Fatalf("store cookie error = %v, want unsupported capability", err)
	}
	if err := session.Transfer(context.Background(), "velocity.internal", 25566); !errors.Is(err, limbgo.ErrUnsupportedCapability) {
		t.Fatalf("transfer error = %v, want unsupported capability", err)
	}
}

func completeModernJoin(t *testing.T, conn net.Conn, reader *bufio.Reader, protocol int32, packetProtocol int32) {
	t.Helper()
	assertPacketID(t, reader, packetProtocol, packetid.StateLogin, "success")
	writeServerboundNamedPacket(t, conn, packetProtocol, packetid.StateLogin, "login_acknowledged", nil)
	registryPacketCount := 4
	if protocol < protocol766 {
		registryPacketCount = 1
	}
	for i := 0; i < registryPacketCount; i++ {
		assertPacketID(t, reader, packetProtocol, packetid.StateConfiguration, "registry_data")
	}
	assertPacketID(t, reader, packetProtocol, packetid.StateConfiguration, "tags")
	assertPacketID(t, reader, packetProtocol, packetid.StateConfiguration, "finish_configuration")
	writeServerboundNamedPacket(t, conn, packetProtocol, packetid.StateConfiguration, "finish_configuration", nil)
	assertPacketID(t, reader, packetProtocol, packetid.StatePlay, "login")
	assertPacketID(t, reader, packetProtocol, packetid.StatePlay, "position")
	assertPacketID(t, reader, packetProtocol, packetid.StatePlay, "chunk_batch_start")
	assertPacketID(t, reader, packetProtocol, packetid.StatePlay, "map_chunk")
	assertPacketID(t, reader, packetProtocol, packetid.StatePlay, "chunk_batch_finished")
}

func assertStoreCookiePacket(t *testing.T, data []byte, wantKey string, wantValue []byte) {
	t.Helper()
	reader := bytes.NewReader(data)
	key, err := wire.ReadString(reader, 32767)
	if err != nil {
		t.Fatalf("read cookie key: %v", err)
	}
	if key != wantKey {
		t.Fatalf("cookie key = %q, want %q", key, wantKey)
	}
	length, err := wire.ReadVarInt(reader)
	if err != nil {
		t.Fatalf("read cookie value length: %v", err)
	}
	if length < 0 {
		t.Fatalf("cookie value length = %d", length)
	}
	value := make([]byte, int(length))
	if _, err := reader.Read(value); err != nil {
		t.Fatalf("read cookie value: %v", err)
	}
	if !bytes.Equal(value, wantValue) {
		t.Fatalf("cookie value = %q, want %q", value, wantValue)
	}
	if reader.Len() != 0 {
		t.Fatalf("store_cookie has %d trailing bytes", reader.Len())
	}
}

func assertTransferPacket(t *testing.T, data []byte, wantHost string, wantPort int32) {
	t.Helper()
	reader := bytes.NewReader(data)
	host, err := wire.ReadString(reader, 32767)
	if err != nil {
		t.Fatalf("read transfer host: %v", err)
	}
	if host != wantHost {
		t.Fatalf("transfer host = %q, want %q", host, wantHost)
	}
	port, err := wire.ReadVarInt(reader)
	if err != nil {
		t.Fatalf("read transfer port: %v", err)
	}
	if port != wantPort {
		t.Fatalf("transfer port = %d, want %d", port, wantPort)
	}
	if reader.Len() != 0 {
		t.Fatalf("transfer has %d trailing bytes", reader.Len())
	}
}

func assertPacketID(t *testing.T, reader *bufio.Reader, protocol int32, state packetid.State, name string) wire.Packet {
	t.Helper()
	packet, err := wire.ReadPacket(reader, 0)
	if err != nil {
		t.Fatalf("read %s packet: %v", name, err)
	}
	want, ok := packetid.ID(protocol, state, packetid.ToClient, name)
	if !ok {
		t.Fatalf("missing generated packet id for %s", name)
	}
	if packet.ID != want {
		t.Fatalf("packet %s id = %#x, want %#x", name, packet.ID, want)
	}
	return packet
}

func assertInlineDialogNBT(t *testing.T, data []byte) {
	t.Helper()
	reader := bytes.NewReader(data)
	holder, err := wire.ReadVarInt(reader)
	if err != nil {
		t.Fatalf("read dialog holder: %v", err)
	}
	if holder != 0 {
		t.Fatalf("dialog holder = %d, want inline holder 0", holder)
	}
	if err := skipAnonymousNBT(reader); err != nil {
		t.Fatalf("skip dialog nbt: %v", err)
	}
	if reader.Len() != 0 {
		t.Fatalf("dialog packet has %d trailing bytes", reader.Len())
	}
}

func assertUpdateTime(t *testing.T, data []byte, wantAge, wantTime int64) {
	t.Helper()
	reader := bytes.NewReader(data)
	age, err := wire.ReadLong(reader)
	if err != nil {
		t.Fatalf("read world age: %v", err)
	}
	timeOfDay, err := wire.ReadLong(reader)
	if err != nil {
		t.Fatalf("read time of day: %v", err)
	}
	if age != wantAge || timeOfDay != wantTime {
		t.Fatalf("update_time = age %d time %d, want age %d time %d", age, timeOfDay, wantAge, wantTime)
	}
	if reader.Len() != 0 {
		t.Fatalf("update_time has %d trailing bytes", reader.Len())
	}
}

func assertFirstChunkBlock47(t *testing.T, data []byte, want uint16) {
	t.Helper()
	reader := bytes.NewReader(data)
	if _, err := readInt32(reader); err != nil {
		t.Fatalf("read chunk x: %v", err)
	}
	if _, err := readInt32(reader); err != nil {
		t.Fatalf("read chunk z: %v", err)
	}
	if _, err := reader.ReadByte(); err != nil {
		t.Fatalf("read ground-up flag: %v", err)
	}
	mask, err := readUint16(reader)
	if err != nil {
		t.Fatalf("read section mask: %v", err)
	}
	if mask != 1 {
		t.Fatalf("section mask = %#x, want 0x1", mask)
	}
	size, err := wire.ReadVarInt(reader)
	if err != nil {
		t.Fatalf("read chunk data size: %v", err)
	}
	if size <= 0 || int(size) > reader.Len() {
		t.Fatalf("invalid chunk data size %d with remaining %d", size, reader.Len())
	}
	var first [2]byte
	if _, err := reader.Read(first[:]); err != nil {
		t.Fatalf("read first block: %v", err)
	}
	if got := binary.BigEndian.Uint16(first[:]); got != want {
		t.Fatalf("first block state = %#x, want %#x", got, want)
	}
}

func assertFirstChunkBlock340(t *testing.T, data []byte, want uint32) {
	t.Helper()
	reader := bytes.NewReader(data)
	if _, err := readInt32(reader); err != nil {
		t.Fatalf("read chunk x: %v", err)
	}
	if _, err := readInt32(reader); err != nil {
		t.Fatalf("read chunk z: %v", err)
	}
	if _, err := reader.ReadByte(); err != nil {
		t.Fatalf("read ground-up flag: %v", err)
	}
	mask, err := wire.ReadVarInt(reader)
	if err != nil {
		t.Fatalf("read section mask: %v", err)
	}
	if mask != 1 {
		t.Fatalf("section mask = %#x, want 0x1", mask)
	}
	size, err := wire.ReadVarInt(reader)
	if err != nil {
		t.Fatalf("read chunk data size: %v", err)
	}
	chunkData := make([]byte, size)
	if _, err := reader.Read(chunkData); err != nil {
		t.Fatalf("read chunk data: %v", err)
	}
	section := bytes.NewReader(chunkData)
	bitsPerBlock, err := section.ReadByte()
	if err != nil {
		t.Fatalf("read bits per block: %v", err)
	}
	if bitsPerBlock != 4 {
		t.Fatalf("bits per block = %d, want 4", bitsPerBlock)
	}
	paletteLen, err := wire.ReadVarInt(section)
	if err != nil {
		t.Fatalf("read palette len: %v", err)
	}
	if paletteLen < 2 {
		t.Fatalf("palette len = %d, want at least 2", paletteLen)
	}
	palette := make([]uint32, paletteLen)
	for i := range palette {
		value, err := wire.ReadVarInt(section)
		if err != nil {
			t.Fatalf("read palette entry %d: %v", i, err)
		}
		palette[i] = uint32(value)
	}
	dataLen, err := wire.ReadVarInt(section)
	if err != nil {
		t.Fatalf("read long array len: %v", err)
	}
	if dataLen <= 0 {
		t.Fatalf("long array len = %d", dataLen)
	}
	firstLong, err := wire.ReadLong(section)
	if err != nil {
		t.Fatalf("read first packed long: %v", err)
	}
	firstPaletteIndex := uint64(firstLong) & 0xf
	if int(firstPaletteIndex) >= len(palette) {
		t.Fatalf("first palette index %d outside palette %+v", firstPaletteIndex, palette)
	}
	if got := palette[firstPaletteIndex]; got != want {
		t.Fatalf("first block state = %#x, want %#x (palette %+v)", got, want, palette)
	}
}

func assertFirstChunkBlockModern(t *testing.T, data []byte, heightmapArray bool, heightmapNamed bool, want uint32) {
	t.Helper()
	reader := bytes.NewReader(data)
	if _, err := readInt32(reader); err != nil {
		t.Fatalf("read chunk x: %v", err)
	}
	if _, err := readInt32(reader); err != nil {
		t.Fatalf("read chunk z: %v", err)
	}
	if heightmapArray {
		count, err := wire.ReadVarInt(reader)
		if err != nil {
			t.Fatalf("read heightmap count: %v", err)
		}
		for i := int32(0); i < count; i++ {
			if _, err := wire.ReadVarInt(reader); err != nil {
				t.Fatalf("read heightmap type: %v", err)
			}
			values, err := wire.ReadVarInt(reader)
			if err != nil {
				t.Fatalf("read heightmap values len: %v", err)
			}
			for j := int32(0); j < values; j++ {
				if _, err := wire.ReadLong(reader); err != nil {
					t.Fatalf("read heightmap long: %v", err)
				}
			}
		}
	} else {
		var err error
		if heightmapNamed {
			err = skipNamedNBT(reader)
		} else {
			err = skipAnonymousNBT(reader)
		}
		if err != nil {
			t.Fatalf("skip heightmaps: %v", err)
		}
	}
	size, err := wire.ReadVarInt(reader)
	if err != nil {
		t.Fatalf("read chunk data size: %v", err)
	}
	chunkData := make([]byte, size)
	if _, err := reader.Read(chunkData); err != nil {
		t.Fatalf("read chunk data: %v", err)
	}
	section := bytes.NewReader(chunkData)
	if _, err := readUint16(section); err != nil {
		t.Fatalf("read non-air count: %v", err)
	}
	bitsPerBlock, err := section.ReadByte()
	if err != nil {
		t.Fatalf("read bits per block: %v", err)
	}
	if bitsPerBlock != 4 {
		t.Fatalf("bits per block = %d, want 4", bitsPerBlock)
	}
	paletteLen, err := wire.ReadVarInt(section)
	if err != nil {
		t.Fatalf("read palette len: %v", err)
	}
	palette := make([]uint32, paletteLen)
	for i := range palette {
		value, err := wire.ReadVarInt(section)
		if err != nil {
			t.Fatalf("read palette entry %d: %v", i, err)
		}
		palette[i] = uint32(value)
	}
	dataLen, err := wire.ReadVarInt(section)
	if err != nil {
		t.Fatalf("read long array len: %v", err)
	}
	if dataLen <= 0 {
		t.Fatalf("long array len = %d", dataLen)
	}
	firstLong, err := wire.ReadLong(section)
	if err != nil {
		t.Fatalf("read first packed long: %v", err)
	}
	firstPaletteIndex := uint64(firstLong) & 0xf
	if int(firstPaletteIndex) >= len(palette) {
		t.Fatalf("first palette index %d outside palette %+v", firstPaletteIndex, palette)
	}
	if got := palette[firstPaletteIndex]; got != want {
		t.Fatalf("first block state = %#x, want %#x (palette %+v)", got, want, palette)
	}
}

func skipAnonymousNBT(reader *bytes.Reader) error {
	tag, err := reader.ReadByte()
	if err != nil {
		return err
	}
	return skipNBTPayload(reader, tag)
}

func skipNamedNBT(reader *bytes.Reader) error {
	tag, err := reader.ReadByte()
	if err != nil {
		return err
	}
	nameLen, err := readUint16(reader)
	if err != nil {
		return err
	}
	if _, err := reader.Seek(int64(nameLen), 1); err != nil {
		return err
	}
	return skipNBTPayload(reader, tag)
}

func skipNBTPayload(reader *bytes.Reader, tag byte) error {
	switch tag {
	case 0:
		return nil
	case 1:
		_, err := reader.ReadByte()
		return err
	case 2:
		_, err := readUint16(reader)
		return err
	case 3:
		_, err := readInt32(reader)
		return err
	case 4:
		_, err := wire.ReadLong(reader)
		return err
	case 5, 6:
		skip := 4
		if tag == 6 {
			skip = 8
		}
		_, err := reader.Seek(int64(skip), 1)
		return err
	case 8:
		length, err := readUint16(reader)
		if err != nil {
			return err
		}
		_, err = reader.Seek(int64(length), 1)
		return err
	case 9:
		child, err := reader.ReadByte()
		if err != nil {
			return err
		}
		count, err := readInt32(reader)
		if err != nil {
			return err
		}
		for i := int32(0); i < count; i++ {
			if err := skipNBTPayload(reader, child); err != nil {
				return err
			}
		}
		return nil
	case 10:
		for {
			child, err := reader.ReadByte()
			if err != nil {
				return err
			}
			if child == 0 {
				return nil
			}
			nameLen, err := readUint16(reader)
			if err != nil {
				return err
			}
			if _, err := reader.Seek(int64(nameLen), 1); err != nil {
				return err
			}
			if err := skipNBTPayload(reader, child); err != nil {
				return err
			}
		}
	case 12:
		count, err := readInt32(reader)
		if err != nil {
			return err
		}
		_, err = reader.Seek(int64(count)*8, 1)
		return err
	default:
		return nil
	}
}

func readInt32(r *bytes.Reader) (int32, error) {
	var buf [4]byte
	if _, err := r.Read(buf[:]); err != nil {
		return 0, err
	}
	return int32(binary.BigEndian.Uint32(buf[:])), nil
}

func readUint16(r *bytes.Reader) (uint16, error) {
	var buf [2]byte
	if _, err := r.Read(buf[:]); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(buf[:]), nil
}

func writeServerboundNamedPacket(t *testing.T, conn net.Conn, protocol int32, state packetid.State, name string, data []byte) {
	t.Helper()
	id, ok := packetid.ID(protocol, state, packetid.ToServer, name)
	if !ok {
		t.Fatalf("missing serverbound packet id for protocol=%d state=%s name=%s", protocol, state, name)
	}
	if err := wire.WritePacket(conn, wire.Packet{ID: id, Data: data}); err != nil {
		t.Fatalf("write serverbound %s: %v", name, err)
	}
}

func loginProtocol(t *testing.T, conn net.Conn, protocol int32, uuidOption bool) {
	t.Helper()
	if err := writeHandshake(conn, protocol, "localhost", 25565, stateLogin); err != nil {
		t.Fatalf("write handshake: %v", err)
	}
	var loginStart bytes.Buffer
	if err := wire.WriteString(&loginStart, "TestPlayer"); err != nil {
		t.Fatalf("write username: %v", err)
	}
	if uuidOption {
		if err := wire.WriteBool(&loginStart, false); err != nil {
			t.Fatalf("write uuid option: %v", err)
		}
	}
	if err := wire.WritePacket(conn, wire.Packet{ID: 0, Data: loginStart.Bytes()}); err != nil {
		t.Fatalf("write login_start: %v", err)
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
	if err := wire.WriteUnsignedShort(&data, port); err != nil {
		return err
	}
	if err := wire.WriteVarInt(&data, nextState); err != nil {
		return err
	}
	return wire.WritePacket(conn, wire.Packet{ID: 0, Data: data.Bytes()})
}

func testWorld() *limbgo.MemoryWorld {
	blocks := make([]uint32, 16*16*16)
	blocks[0] = 1
	return &limbgo.MemoryWorld{
		WorldID: "spawn",
		WorldDimension: limbgo.Dimension{
			Name:        "minecraft:overworld",
			MinY:        0,
			Height:      256,
			Natural:     true,
			HasSkylight: true,
		},
		Palette: []limbgo.BlockState{
			{Name: "minecraft:air"},
			{Name: "minecraft:stone"},
		},
		Chunks: map[limbgo.ChunkPos]limbgo.Chunk{
			{X: 0, Z: 0}: {
				X:    0,
				Z:    0,
				MinY: 0,
				Sections: []limbgo.ChunkSection{
					{Y: 0, BlockStateIDs: blocks},
				},
			},
		},
	}
}

type testServices struct {
	spawn  limbgo.SpawnTarget
	world  limbgo.World
	events limbgo.PlayerEventHandler
}

func (s testServices) ResolveSpawn(context.Context, limbgo.Player) (limbgo.SpawnTarget, error) {
	return s.spawn, nil
}

func (s testServices) World(context.Context, string) (limbgo.World, error) {
	return s.world, nil
}

func (s testServices) Events() limbgo.PlayerEventHandler {
	return s.events
}
