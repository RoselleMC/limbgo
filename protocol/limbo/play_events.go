package limbo

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"strings"

	"github.com/RoselleMC/limbgo"
	"github.com/RoselleMC/limbgo/dialog"
	"github.com/RoselleMC/limbgo/internal/protocol/wire"
	"github.com/RoselleMC/limbgo/protocol/packetid"
	"go.minekube.com/common/minecraft/component"
	"go.minekube.com/common/minecraft/component/codec"
)

type playAdapter struct {
	protocol       int32
	packetProtocol int32
}

func newPlayAdapter(protocol int32) playAdapter {
	return playAdapter{protocol: protocol, packetProtocol: protocol}
}

func newModernPlayAdapter(cfg modernProtocolConfig) playAdapter {
	return playAdapter{protocol: cfg.protocol, packetProtocol: cfg.packetProtocol()}
}

type playSession struct {
	conn    net.Conn
	player  limbgo.Player
	adapter playAdapter
}

func (s *playSession) Player() limbgo.Player {
	return s.player
}

func (s *playSession) SendMessage(_ context.Context, message component.Component) error {
	return writeSystemMessage(s.conn, s.adapter, message)
}

func (s *playSession) ShowDialog(_ context.Context, dialog dialog.Dialog) error {
	return writeShowDialog(s.conn, s.adapter, dialog)
}

func (s *playSession) ClearDialog(_ context.Context) error {
	return writeClearDialog(s.conn, s.adapter)
}

func servePlayEvents(ctx context.Context, conn net.Conn, reader *bufio.Reader, services limbgo.SessionServices, player limbgo.Player, adapter playAdapter) error {
	handler := services.Events()
	if handler == nil {
		return nil
	}
	session := &playSession{conn: conn, player: player, adapter: adapter}
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		packet, err := wire.ReadPacket(reader, 0)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		if err := handlePlayPacket(ctx, handler, session, packet); err != nil {
			return err
		}
	}
}

func handlePlayPacket(ctx context.Context, handler limbgo.PlayerEventHandler, session *playSession, packet wire.Packet) error {
	adapter := session.adapter
	switch {
	case isServerboundPlayPacket(adapter, packet.ID, "chat"):
		message, err := readFirstPacketString(packet.Data)
		if err != nil {
			return err
		}
		if strings.HasPrefix(message, "/") {
			return handler.HandleCommand(ctx, session, &limbgo.CommandEvent{
				Player:   session.player,
				Command:  strings.TrimPrefix(message, "/"),
				Protocol: int(adapter.protocol),
			})
		}
		return handler.HandleChat(ctx, session, &limbgo.ChatEvent{
			Player:   session.player,
			Message:  message,
			Protocol: int(adapter.protocol),
		})
	case isServerboundPlayPacket(adapter, packet.ID, "chat_message"):
		message, err := readFirstPacketString(packet.Data)
		if err != nil {
			return err
		}
		return handler.HandleChat(ctx, session, &limbgo.ChatEvent{
			Player:   session.player,
			Message:  message,
			Protocol: int(adapter.protocol),
		})
	case isServerboundPlayPacket(adapter, packet.ID, "chat_command"), isServerboundPlayPacket(adapter, packet.ID, "chat_command_signed"):
		command, err := readFirstPacketString(packet.Data)
		if err != nil {
			return err
		}
		return handler.HandleCommand(ctx, session, &limbgo.CommandEvent{
			Player:   session.player,
			Command:  strings.TrimPrefix(command, "/"),
			Protocol: int(adapter.protocol),
		})
	case isServerboundPlayPacket(adapter, packet.ID, "custom_click_action"):
		event, err := readCustomClickAction(adapter, session.player, packet.Data)
		if err != nil {
			return err
		}
		return handler.HandleDialogClick(ctx, session, event)
	default:
		return nil
	}
}

func isServerboundPlayPacket(adapter playAdapter, got int32, name string) bool {
	id, ok := packetid.ID(adapter.packetProtocol, packetid.StatePlay, packetid.ToServer, name)
	return ok && got == id
}

func readFirstPacketString(data []byte) (string, error) {
	return wire.ReadString(bytes.NewReader(data), 256)
}

func readCustomClickAction(adapter playAdapter, player limbgo.Player, data []byte) (*limbgo.DialogClickEvent, error) {
	reader := bytes.NewReader(data)
	id, err := wire.ReadString(reader, 32767)
	if err != nil {
		return nil, err
	}
	payload, err := readOptionalAnonymousNBT(reader)
	if err != nil {
		return nil, err
	}
	return &limbgo.DialogClickEvent{
		Player:   player,
		ID:       id,
		Payload:  payload,
		Protocol: int(adapter.protocol),
	}, nil
}

func readOptionalAnonymousNBT(reader *bytes.Reader) ([]byte, error) {
	if reader.Len() == 0 {
		return nil, nil
	}
	present, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	if present == 0 {
		return nil, nil
	}
	payload := make([]byte, reader.Len())
	_, _ = reader.Read(payload)
	return payload, nil
}

func writeSystemMessage(conn net.Conn, adapter playAdapter, message component.Component) error {
	if message == nil {
		message = &component.Text{}
	}
	if adapter.protocol >= protocol765 {
		return writeSystemMessageNBT(conn, adapter, message)
	}
	raw, err := marshalComponentJSON(adapter.protocol, message)
	if err != nil {
		return err
	}
	if id, ok := packetid.ID(adapter.packetProtocol, packetid.StatePlay, packetid.ToClient, "system_chat"); ok {
		var data bytes.Buffer
		if err := wire.WriteString(&data, string(raw)); err != nil {
			return err
		}
		if adapter.protocol == protocol759 {
			if err := wire.WriteVarInt(&data, 1); err != nil {
				return err
			}
		} else {
			if err := wire.WriteBool(&data, false); err != nil {
				return err
			}
		}
		return wire.WritePacket(conn, wire.Packet{ID: id, Data: data.Bytes()})
	}
	id, ok := packetid.ID(adapter.packetProtocol, packetid.StatePlay, packetid.ToClient, "chat")
	if !ok {
		return fmt.Errorf("missing chat/system_chat packet id for protocol %d", adapter.protocol)
	}
	var data bytes.Buffer
	if err := wire.WriteString(&data, string(raw)); err != nil {
		return err
	}
	if err := wire.WriteByte(&data, 0); err != nil {
		return err
	}
	if adapter.protocol >= protocol757 {
		data.Write(make([]byte, 16))
	}
	return wire.WritePacket(conn, wire.Packet{ID: id, Data: data.Bytes()})
}

func writeSystemMessageNBT(conn net.Conn, adapter playAdapter, message component.Component) error {
	id, ok := packetid.ID(adapter.packetProtocol, packetid.StatePlay, packetid.ToClient, "system_chat")
	if !ok {
		return fmt.Errorf("missing system_chat packet id for protocol %d", adapter.protocol)
	}
	raw, err := marshalComponentJSON(adapter.protocol, message)
	if err != nil {
		return err
	}
	nbt, err := componentJSONToAnonymousNBT(raw)
	if err != nil {
		return err
	}
	var data bytes.Buffer
	if _, err := data.Write(nbt); err != nil {
		return err
	}
	if err := wire.WriteBool(&data, false); err != nil {
		return err
	}
	return wire.WritePacket(conn, wire.Packet{ID: id, Data: data.Bytes()})
}

func writeShowDialog(conn net.Conn, adapter playAdapter, value dialog.Dialog) error {
	id, ok := packetid.ID(adapter.packetProtocol, packetid.StatePlay, packetid.ToClient, "show_dialog")
	if !ok {
		return fmt.Errorf("show_dialog is not available for protocol %d", adapter.protocol)
	}
	if value == nil {
		return fmt.Errorf("show_dialog requires a dialog")
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	nbt, err := componentJSONToAnonymousNBT(raw)
	if err != nil {
		return err
	}
	var data bytes.Buffer
	if err := wire.WriteVarInt(&data, 0); err != nil {
		return err
	}
	if _, err := data.Write(nbt); err != nil {
		return err
	}
	return wire.WritePacket(conn, wire.Packet{ID: id, Data: data.Bytes()})
}

func writeClearDialog(conn net.Conn, adapter playAdapter) error {
	id, ok := packetid.ID(adapter.packetProtocol, packetid.StatePlay, packetid.ToClient, "clear_dialog")
	if !ok {
		return fmt.Errorf("clear_dialog is not available for protocol %d", adapter.protocol)
	}
	return wire.WritePacket(conn, wire.Packet{ID: id})
}

func marshalComponentJSON(protocol int32, message component.Component) ([]byte, error) {
	var out bytes.Buffer
	encoder := codec.Json{
		UseLegacyFieldNames:                     protocol < protocol770,
		UseLegacyClickEventStructure:            protocol < protocol770,
		UseLegacyHoverEventStructure:            protocol < protocol770,
		EmitCompactTextComponent:                false,
		EmitHoverShowEntityIdAsIntArray:         protocol >= protocol764,
		EmitHoverShowEntityKeyAsTypeAndUuidAsId: protocol < protocol770,
		EmitDefaultItemHoverQuantity:            protocol >= protocol766,
		StdJson:                                 true,
	}
	if err := encoder.Marshal(&out, message); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func componentJSONToAnonymousNBT(raw []byte) ([]byte, error) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := writeAnonymousJSONNBT(&out, value); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func writeAnonymousJSONNBT(out *bytes.Buffer, value any) error {
	switch typed := value.(type) {
	case map[string]any:
		if err := out.WriteByte(nbtCompound); err != nil {
			return err
		}
		for key, child := range typed {
			if err := writeNamedJSONNBT(out, key, child); err != nil {
				return err
			}
		}
		return out.WriteByte(nbtEnd)
	case []any:
		return writeAnonymousJSONListNBT(out, typed)
	case string:
		if err := out.WriteByte(nbtString); err != nil {
			return err
		}
		writeRawNBTString(out, typed)
		return nil
	case bool:
		if err := out.WriteByte(nbtByte); err != nil {
			return err
		}
		if typed {
			return out.WriteByte(1)
		}
		return out.WriteByte(0)
	case json.Number:
		tag, err := jsonNumberNBTTag(typed)
		if err != nil {
			return err
		}
		if err := out.WriteByte(tag); err != nil {
			return err
		}
		return writeJSONNumberNBTPayload(out, tag, typed)
	default:
		if err := out.WriteByte(nbtCompound); err != nil {
			return err
		}
		return out.WriteByte(nbtEnd)
	}
}

func writeNamedJSONNBT(out *bytes.Buffer, name string, value any) error {
	tag := jsonNBTTag(value)
	if err := out.WriteByte(tag); err != nil {
		return err
	}
	writeRawNBTString(out, name)
	return writeJSONNBTPayload(out, tag, value)
}

func writeJSONNBTPayload(out *bytes.Buffer, tag byte, value any) error {
	switch tag {
	case nbtCompound:
		object, _ := value.(map[string]any)
		for key, child := range object {
			if err := writeNamedJSONNBT(out, key, child); err != nil {
				return err
			}
		}
		return out.WriteByte(nbtEnd)
	case nbtList:
		values, _ := value.([]any)
		return writeJSONListPayload(out, values)
	case nbtString:
		text, _ := value.(string)
		writeRawNBTString(out, text)
	case nbtByte:
		boolean, _ := value.(bool)
		if boolean {
			return out.WriteByte(1)
		}
		return out.WriteByte(0)
	case nbtInt:
		number, _ := value.(json.Number)
		integer, err := number.Int64()
		if err != nil {
			return err
		}
		writeRawNBTInt(out, int32(integer))
	case nbtFloat:
		number, _ := value.(json.Number)
		floatValue, err := number.Float64()
		if err != nil {
			return err
		}
		writeRawNBTFloat(out, float32(floatValue))
	}
	return nil
}

func writeAnonymousJSONListNBT(out *bytes.Buffer, values []any) error {
	if err := out.WriteByte(nbtList); err != nil {
		return err
	}
	return writeJSONListPayload(out, values)
}

func writeJSONListPayload(out *bytes.Buffer, values []any) error {
	childTag := byte(nbtEnd)
	if len(values) > 0 {
		childTag = jsonNBTTag(values[0])
	}
	if err := out.WriteByte(childTag); err != nil {
		return err
	}
	writeRawNBTInt(out, int32(len(values)))
	for _, value := range values {
		if err := writeJSONNBTPayload(out, childTag, value); err != nil {
			return err
		}
	}
	return nil
}

func jsonNBTTag(value any) byte {
	switch value.(type) {
	case map[string]any:
		return nbtCompound
	case []any:
		return nbtList
	case bool:
		return nbtByte
	case json.Number:
		tag, err := jsonNumberNBTTag(value.(json.Number))
		if err == nil {
			return tag
		}
		return nbtString
	default:
		return nbtString
	}
}

func jsonNumberNBTTag(value json.Number) (byte, error) {
	text := value.String()
	if strings.ContainsAny(text, ".eE") {
		if _, err := value.Float64(); err != nil {
			return 0, err
		}
		return nbtFloat, nil
	}
	if _, err := value.Int64(); err != nil {
		return 0, err
	}
	return nbtInt, nil
}

func writeJSONNumberNBTPayload(out *bytes.Buffer, tag byte, value json.Number) error {
	switch tag {
	case nbtInt:
		integer, err := value.Int64()
		if err != nil {
			return err
		}
		writeRawNBTInt(out, int32(integer))
	case nbtFloat:
		floatValue, err := value.Float64()
		if err != nil {
			return err
		}
		writeRawNBTFloat(out, float32(floatValue))
	default:
		return fmt.Errorf("unsupported json number nbt tag %d", tag)
	}
	return nil
}

func writeRawNBTString(out *bytes.Buffer, value string) {
	_ = wire.WriteUnsignedShort(out, uint16(len(value)))
	_, _ = out.WriteString(value)
}

func writeRawNBTInt(out *bytes.Buffer, value int32) {
	_ = wire.WriteInt(out, value)
}

func writeRawNBTFloat(out *bytes.Buffer, value float32) {
	_ = wire.WriteInt(out, int32(math.Float32bits(value)))
}
