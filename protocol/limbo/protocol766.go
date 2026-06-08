package limbo

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"

	"github.com/RoselleMC/limbgo"
	"github.com/RoselleMC/limbgo/internal/protocol/wire"
	"github.com/RoselleMC/limbgo/protocol/packetid"
	"github.com/RoselleMC/limbgo/protocol/registrydata"
)

const protocol757 = int32(757)
const protocol758 = int32(758)
const protocol759 = int32(759)
const protocol760 = int32(760)
const protocol761 = int32(761)
const protocol762 = int32(762)
const protocol763 = int32(763)
const protocol764 = int32(764)
const protocol765 = int32(765)
const protocol766 = int32(766)
const protocol767 = int32(767)
const protocol768 = int32(768)
const protocol769 = int32(769)
const protocol770 = int32(770)
const protocol771 = int32(771)
const protocol772 = int32(772)
const protocol773 = int32(773)
const protocol774 = int32(774)
const protocol775 = int32(775)

func serveModernPreConfigurationProtocol(ctx context.Context, conn net.Conn, services limbgo.SessionServices, player limbgo.Player, cfg modernProtocolConfig, registryData *registrydata.Data) error {
	join, err := resolveJoin(ctx, services, player)
	if err != nil {
		return err
	}
	spawn := join.Spawn
	world := join.World
	if err := writeLoginSuccessModern(conn, player, cfg); err != nil {
		return err
	}
	if err := writeJoinGamePreConfigurationModern(conn, spawn, world, cfg, registryData); err != nil {
		return err
	}
	if err := writePlayerPositionModern(conn, spawn, cfg); err != nil {
		return err
	}
	if err := writeUpdateTime(cfg.packetProtocol(), conn, world); err != nil {
		return err
	}
	chunk, ok := world.Chunk(chunkCoord(spawn.Position.X), chunkCoord(spawn.Position.Z))
	if !ok {
		return fmt.Errorf("%w: spawn chunk %d,%d", limbgo.ErrWorldNotFound, chunkCoord(spawn.Position.X), chunkCoord(spawn.Position.Z))
	}
	if err := writeMapChunkModern(conn, world, chunk, cfg); err != nil {
		return err
	}
	return servePlayEvents(ctx, conn, bufio.NewReader(conn), services, player, newModernPlayAdapter(cfg))
}

type modernProtocolConfig struct {
	protocol                       int32
	packetIDProtocol               int32
	dataProtocol                   int32
	preConfiguration               bool
	preConfigurationDimensionNBT   bool
	preConfigurationDeath          bool
	preConfigurationPortalCooldown bool
	positionDismountVehicle        bool
	loginSuccessNoProperties       bool
	strictErrorHandling            bool
	registryCodecNBT               bool
	legacyPlayLogin                bool
	positionV2                     bool
	spawnInfoSeaLevel              bool
	positionFlagsU32               bool
	chunkHeightmapArray            bool
	chunkHeightmapFullNBT          bool
	chunkTrustEdges                bool
}

func (cfg modernProtocolConfig) packetProtocol() int32 {
	if cfg.packetIDProtocol != 0 {
		return cfg.packetIDProtocol
	}
	return cfg.protocol
}

func (cfg modernProtocolConfig) dataProtocolID() int32 {
	if cfg.dataProtocol != 0 {
		return cfg.dataProtocol
	}
	return cfg.protocol
}

func serveModernProtocol(ctx context.Context, conn net.Conn, reader *bufio.Reader, services limbgo.SessionServices, player limbgo.Player, cfg modernProtocolConfig, registryData *registrydata.Data) error {
	join, err := resolveJoin(ctx, services, player)
	if err != nil {
		return err
	}
	spawn := join.Spawn
	world := join.World

	if err := writeLoginSuccessModern(conn, player, cfg); err != nil {
		return err
	}
	ack, err := wire.ReadPacket(reader, 0)
	if err != nil {
		return err
	}
	ackID, ok := packetid.ID(cfg.packetProtocol(), packetid.StateLogin, packetid.ToServer, "login_acknowledged")
	if !ok || ack.ID != ackID {
		return fmt.Errorf("expected login_acknowledged packet %d, got %d", ackID, ack.ID)
	}

	if cfg.registryCodecNBT {
		if err := writeRegistryCodecModern(conn, cfg, registryData); err != nil {
			return err
		}
	} else {
		if err := writeRegistryDataModern(conn, world, cfg, registryData); err != nil {
			return err
		}
	}
	if err := writeTagsModern(conn, cfg); err != nil {
		return err
	}
	if err := writeNamedPacketModern(conn, cfg, packetid.StateConfiguration, "finish_configuration", nil); err != nil {
		return err
	}
	finish, err := wire.ReadPacket(reader, 0)
	if err != nil {
		return err
	}
	finishID, ok := packetid.ID(cfg.packetProtocol(), packetid.StateConfiguration, packetid.ToServer, "finish_configuration")
	if !ok || finish.ID != finishID {
		return fmt.Errorf("expected finish_configuration packet %d, got %d", finishID, finish.ID)
	}

	if cfg.legacyPlayLogin {
		if err := writeJoinGameLegacyModern(conn, spawn, world, cfg); err != nil {
			return err
		}
	} else {
		if err := writeJoinGameModern(conn, spawn, world, cfg); err != nil {
			return err
		}
	}
	if err := writePlayerPositionModern(conn, spawn, cfg); err != nil {
		return err
	}
	if err := writeUpdateTime(cfg.packetProtocol(), conn, world); err != nil {
		return err
	}
	chunk, ok := world.Chunk(chunkCoord(spawn.Position.X), chunkCoord(spawn.Position.Z))
	if !ok {
		return fmt.Errorf("%w: spawn chunk %d,%d", limbgo.ErrWorldNotFound, chunkCoord(spawn.Position.X), chunkCoord(spawn.Position.Z))
	}
	if err := writeNamedPacketModern(conn, cfg, packetid.StatePlay, "chunk_batch_start", nil); err != nil {
		return err
	}
	if err := writeMapChunkModern(conn, world, chunk, cfg); err != nil {
		return err
	}
	if err := writeChunkBatchFinishedModern(conn, 1, cfg); err != nil {
		return err
	}
	return servePlayEvents(ctx, conn, reader, services, player, newModernPlayAdapter(cfg))
}

func writeLoginSuccessModern(conn net.Conn, player limbgo.Player, cfg modernProtocolConfig) error {
	var data bytes.Buffer
	if err := writeUUID(&data, player.UUID); err != nil {
		return err
	}
	if err := wire.WriteString(&data, player.Name); err != nil {
		return err
	}
	if !cfg.loginSuccessNoProperties {
		if err := wire.WriteVarInt(&data, 0); err != nil {
			return err
		}
	}
	if cfg.strictErrorHandling {
		if err := wire.WriteBool(&data, false); err != nil {
			return err
		}
	}
	return writeNamedPacketModern(conn, cfg, packetid.StateLogin, "success", data.Bytes())
}

func writeRegistryCodecModern(conn net.Conn, cfg modernProtocolConfig, registryData *registrydata.Data) error {
	protocol := cfg.dataProtocolID()
	codec, ok := registryData.DimensionCodec(protocol)
	if !ok {
		return fmt.Errorf("no generated dimension codec for protocol %d", protocol)
	}
	return writeNamedPacketModern(conn, cfg, packetid.StateConfiguration, "registry_data", codec)
}

func writeRegistryDataModern(conn net.Conn, world limbgo.World, cfg modernProtocolConfig, registryData *registrydata.Data) error {
	protocol := cfg.dataProtocolID()
	registries, ok := registryData.Registries(protocol)
	if !ok {
		return fmt.Errorf("no generated registry data for protocol %d", protocol)
	}
	out := []registrydata.Registry{
		dimensionTypeRegistry766(world.Dimension()),
	}
	out = append(out, registries...)
	for _, registry := range out {
		var data bytes.Buffer
		if err := wire.WriteString(&data, registry.ID); err != nil {
			return err
		}
		if err := wire.WriteVarInt(&data, int32(len(registry.Entries))); err != nil {
			return err
		}
		for _, entry := range registry.Entries {
			if err := wire.WriteString(&data, entry.Key); err != nil {
				return err
			}
			if err := writeOptionAnonymousNBT(&data, true, entry.Value); err != nil {
				return err
			}
		}
		if err := writeNamedPacketModern(conn, cfg, packetid.StateConfiguration, "registry_data", data.Bytes()); err != nil {
			return err
		}
	}
	return nil
}

func writeTagsModern(conn net.Conn, cfg modernProtocolConfig) error {
	var data bytes.Buffer
	if err := wire.WriteVarInt(&data, 0); err != nil {
		return err
	}
	return writeNamedPacketModern(conn, cfg, packetid.StateConfiguration, "tags", data.Bytes())
}

func writeJoinGamePreConfigurationModern(conn net.Conn, spawn limbgo.SpawnTarget, world limbgo.World, cfg modernProtocolConfig, registryData *registrydata.Data) error {
	dimensionName := limbgo.NormalizeDimension(world.Dimension(), 256).Name
	codec, ok := registryData.DimensionCodec(cfg.dataProtocolID())
	if !ok {
		return fmt.Errorf("no generated dimension codec for protocol %d", cfg.protocol)
	}
	var data bytes.Buffer
	if err := wire.WriteInt(&data, 1); err != nil {
		return err
	}
	if err := wire.WriteBool(&data, false); err != nil {
		return err
	}
	if err := wire.WriteByte(&data, byte(spawn.GameMode)); err != nil {
		return err
	}
	if err := wire.WriteByte(&data, 255); err != nil {
		return err
	}
	if err := wire.WriteVarInt(&data, 1); err != nil {
		return err
	}
	if err := wire.WriteString(&data, dimensionName); err != nil {
		return err
	}
	if _, err := data.Write(fullNBT(codec)); err != nil {
		return err
	}
	if cfg.preConfigurationDimensionNBT {
		dimension, ok := registryData.Dimension(cfg.dataProtocolID())
		if !ok {
			return fmt.Errorf("no generated dimension for protocol %d", cfg.protocol)
		}
		if _, err := data.Write(fullNBT(dimension)); err != nil {
			return err
		}
	} else {
		if err := wire.WriteString(&data, dimensionName); err != nil {
			return err
		}
	}
	if err := wire.WriteString(&data, dimensionName); err != nil {
		return err
	}
	if err := wire.WriteLong(&data, 0); err != nil {
		return err
	}
	if err := wire.WriteVarInt(&data, 1); err != nil {
		return err
	}
	if err := wire.WriteVarInt(&data, 2); err != nil {
		return err
	}
	if err := wire.WriteVarInt(&data, 2); err != nil {
		return err
	}
	if cfg.preConfigurationDeath {
		if err := wire.WriteBool(&data, false); err != nil {
			return err
		}
	}
	if err := wire.WriteBool(&data, false); err != nil {
		return err
	}
	if err := wire.WriteBool(&data, false); err != nil {
		return err
	}
	if err := wire.WriteBool(&data, false); err != nil {
		return err
	}
	if err := wire.WriteBool(&data, false); err != nil {
		return err
	}
	if cfg.preConfigurationPortalCooldown {
		if err := wire.WriteVarInt(&data, 0); err != nil {
			return err
		}
	}
	return writeNamedPacketModern(conn, cfg, packetid.StatePlay, "login", data.Bytes())
}

func writeJoinGameLegacyModern(conn net.Conn, spawn limbgo.SpawnTarget, world limbgo.World, cfg modernProtocolConfig) error {
	dimensionName := limbgo.NormalizeDimension(world.Dimension(), 256).Name
	var data bytes.Buffer
	if err := wire.WriteInt(&data, 1); err != nil {
		return err
	}
	if err := wire.WriteBool(&data, false); err != nil {
		return err
	}
	if err := wire.WriteVarInt(&data, 1); err != nil {
		return err
	}
	if err := wire.WriteString(&data, dimensionName); err != nil {
		return err
	}
	if err := wire.WriteVarInt(&data, 1); err != nil {
		return err
	}
	if err := wire.WriteVarInt(&data, 2); err != nil {
		return err
	}
	if err := wire.WriteVarInt(&data, 2); err != nil {
		return err
	}
	if err := wire.WriteBool(&data, false); err != nil {
		return err
	}
	if err := wire.WriteBool(&data, false); err != nil {
		return err
	}
	if err := wire.WriteBool(&data, false); err != nil {
		return err
	}
	if err := wire.WriteString(&data, dimensionName); err != nil {
		return err
	}
	if err := wire.WriteString(&data, dimensionName); err != nil {
		return err
	}
	if err := wire.WriteLong(&data, 0); err != nil {
		return err
	}
	if err := wire.WriteByte(&data, byte(spawn.GameMode)); err != nil {
		return err
	}
	if err := wire.WriteByte(&data, 255); err != nil {
		return err
	}
	if err := wire.WriteBool(&data, false); err != nil {
		return err
	}
	if err := wire.WriteBool(&data, false); err != nil {
		return err
	}
	if err := wire.WriteBool(&data, false); err != nil {
		return err
	}
	if err := wire.WriteVarInt(&data, 0); err != nil {
		return err
	}
	return writeNamedPacketModern(conn, cfg, packetid.StatePlay, "login", data.Bytes())
}

func writeJoinGameModern(conn net.Conn, spawn limbgo.SpawnTarget, world limbgo.World, cfg modernProtocolConfig) error {
	dimensionName := limbgo.NormalizeDimension(world.Dimension(), 256).Name
	var data bytes.Buffer
	if err := wire.WriteInt(&data, 1); err != nil {
		return err
	}
	if err := wire.WriteBool(&data, false); err != nil {
		return err
	}
	if err := wire.WriteVarInt(&data, 1); err != nil {
		return err
	}
	if err := wire.WriteString(&data, dimensionName); err != nil {
		return err
	}
	if err := wire.WriteVarInt(&data, 1); err != nil {
		return err
	}
	if err := wire.WriteVarInt(&data, 2); err != nil {
		return err
	}
	if err := wire.WriteVarInt(&data, 2); err != nil {
		return err
	}
	if err := wire.WriteBool(&data, false); err != nil {
		return err
	}
	if err := wire.WriteBool(&data, false); err != nil {
		return err
	}
	if err := wire.WriteBool(&data, false); err != nil {
		return err
	}
	if err := wire.WriteVarInt(&data, 0); err != nil {
		return err
	}
	if err := wire.WriteString(&data, dimensionName); err != nil {
		return err
	}
	if err := wire.WriteLong(&data, 0); err != nil {
		return err
	}
	if err := wire.WriteByte(&data, byte(spawn.GameMode)); err != nil {
		return err
	}
	if err := wire.WriteByte(&data, 255); err != nil {
		return err
	}
	if err := wire.WriteBool(&data, false); err != nil {
		return err
	}
	if err := wire.WriteBool(&data, false); err != nil {
		return err
	}
	if err := wire.WriteBool(&data, false); err != nil {
		return err
	}
	if err := wire.WriteVarInt(&data, 0); err != nil {
		return err
	}
	if cfg.spawnInfoSeaLevel {
		if err := wire.WriteVarInt(&data, int32(spawn.Position.Y)); err != nil {
			return err
		}
	}
	if err := wire.WriteBool(&data, false); err != nil {
		return err
	}
	return writeNamedPacketModern(conn, cfg, packetid.StatePlay, "login", data.Bytes())
}

func writePlayerPositionModern(conn net.Conn, spawn limbgo.SpawnTarget, cfg modernProtocolConfig) error {
	var data bytes.Buffer
	if cfg.positionV2 {
		if err := wire.WriteVarInt(&data, 1); err != nil {
			return err
		}
	}
	if err := wire.WriteDouble(&data, spawn.Position.X); err != nil {
		return err
	}
	if err := wire.WriteDouble(&data, spawn.Position.Y); err != nil {
		return err
	}
	if err := wire.WriteDouble(&data, spawn.Position.Z); err != nil {
		return err
	}
	if cfg.positionV2 {
		if err := wire.WriteDouble(&data, 0); err != nil {
			return err
		}
		if err := wire.WriteDouble(&data, 0); err != nil {
			return err
		}
		if err := wire.WriteDouble(&data, 0); err != nil {
			return err
		}
	}
	if err := wire.WriteFloat(&data, spawn.Rotation.Yaw); err != nil {
		return err
	}
	if err := wire.WriteFloat(&data, spawn.Rotation.Pitch); err != nil {
		return err
	}
	if cfg.positionFlagsU32 {
		if err := wire.WriteInt(&data, 0); err != nil {
			return err
		}
	} else {
		if err := wire.WriteByte(&data, 0); err != nil {
			return err
		}
	}
	if !cfg.positionV2 {
		if err := wire.WriteVarInt(&data, 1); err != nil {
			return err
		}
	}
	if cfg.positionDismountVehicle {
		if err := wire.WriteBool(&data, false); err != nil {
			return err
		}
	}
	return writeNamedPacketModern(conn, cfg, packetid.StatePlay, "position", data.Bytes())
}

func writeMapChunkModern(conn net.Conn, world limbgo.World, chunk limbgo.Chunk, cfg modernProtocolConfig) error {
	chunkData := encodeChunkDataModern(world, chunk, cfg.dataProtocolID())
	var data bytes.Buffer
	if err := wire.WriteInt(&data, chunk.X); err != nil {
		return err
	}
	if err := wire.WriteInt(&data, chunk.Z); err != nil {
		return err
	}
	if cfg.chunkHeightmapArray {
		writeHeightmapsArrayModern(&data)
	} else if cfg.chunkHeightmapFullNBT {
		if _, err := data.Write(fullNBT(heightmapsNBT766())); err != nil {
			return err
		}
	} else {
		if _, err := data.Write(heightmapsNBT766()); err != nil {
			return err
		}
	}
	if err := wire.WriteVarInt(&data, int32(len(chunkData))); err != nil {
		return err
	}
	if _, err := data.Write(chunkData); err != nil {
		return err
	}
	if err := wire.WriteVarInt(&data, 0); err != nil {
		return err
	}
	if cfg.chunkTrustEdges {
		if err := wire.WriteBool(&data, false); err != nil {
			return err
		}
	}
	writeLightData766(&data)
	return writeNamedPacketModern(conn, cfg, packetid.StatePlay, "map_chunk", data.Bytes())
}

func writeChunkBatchFinishedModern(conn net.Conn, size int32, cfg modernProtocolConfig) error {
	var data bytes.Buffer
	if err := wire.WriteVarInt(&data, size); err != nil {
		return err
	}
	return writeNamedPacketModern(conn, cfg, packetid.StatePlay, "chunk_batch_finished", data.Bytes())
}

func writeNamedPacketModern(conn net.Conn, cfg modernProtocolConfig, state packetid.State, name string, data []byte) error {
	id, ok := packetid.ID(cfg.packetProtocol(), state, packetid.ToClient, name)
	if !ok {
		return fmt.Errorf("missing packet id for protocol %d using packet protocol %d state %s packet %s", cfg.protocol, cfg.packetProtocol(), state, name)
	}
	return wire.WritePacket(conn, wire.Packet{ID: id, Data: data})
}
