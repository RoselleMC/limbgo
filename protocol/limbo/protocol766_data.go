package limbo

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/RoselleMC/limbgo"
	"github.com/RoselleMC/limbgo/internal/protocol/wire"
	"github.com/RoselleMC/limbgo/protocol/registrydata"
)

func dimensionTypeRegistry766(dimension limbgo.Dimension) registrydata.Registry {
	if dimension.Name == "" {
		dimension.Name = "minecraft:overworld"
	}
	if dimension.Height == 0 {
		dimension.Height = 256
	}
	if dimension.CoordinateScale == 0 {
		dimension.CoordinateScale = 1
	}
	var n nbtWriter
	n.writeAnonymousCompound(func() {
		n.writeByte("piglin_safe", 0)
		n.writeByte("natural", boolByte(dimension.Natural))
		n.writeFloat("ambient_light", dimension.AmbientLight)
		n.writeInt("monster_spawn_block_light_limit", 0)
		n.writeString("infiniburn", "#minecraft:infiniburn_overworld")
		n.writeByte("respawn_anchor_works", boolByte(dimension.RespawnAnchorWorks))
		n.writeByte("has_skylight", boolByte(dimension.HasSkylight))
		n.writeByte("bed_works", boolByte(dimension.BedWorks))
		n.writeString("effects", "minecraft:overworld")
		n.writeByte("has_raids", 1)
		n.writeInt("logical_height", dimension.Height)
		n.writeDouble("coordinate_scale", dimension.CoordinateScale)
		n.writeCompound("monster_spawn_light_level", func() {
			n.writeInt("min_inclusive", 0)
			n.writeInt("max_inclusive", 7)
			n.writeString("type", "minecraft:uniform")
		})
		n.writeInt("min_y", dimension.MinY)
		n.writeByte("ultrawarm", 0)
		n.writeByte("has_ceiling", 0)
		n.writeInt("height", dimension.Height)
	})
	return registrydata.Registry{
		ID: "minecraft:dimension_type",
		Entries: []registrydata.Entry{{
			Key:   dimension.Name,
			Value: n.bytes(),
		}},
	}
}

func heightmapsNBT766() []byte {
	var n nbtWriter
	n.writeAnonymousCompound(func() {
		n.writeLongArray("MOTION_BLOCKING", make([]int64, 37))
	})
	return n.bytes()
}

func writeHeightmapsArrayModern(data *bytes.Buffer) {
	_ = wire.WriteVarInt(data, 1)
	_ = wire.WriteVarInt(data, 4)
	_ = wire.WriteVarInt(data, 37)
	for i := 0; i < 37; i++ {
		_ = wire.WriteLong(data, 0)
	}
}

func encodeChunkDataModern(world limbgo.World, chunk limbgo.Chunk, protocol int32) []byte {
	dimension := world.Dimension()
	sectionCount := int(dimension.Height / 16)
	if sectionCount <= 0 {
		sectionCount = 16
	}
	var data bytes.Buffer
	for i := 0; i < sectionCount; i++ {
		sectionY := dimension.MinY/16 + int32(i)
		writeSection766(&data, world.BlockPalette(), findSection(chunk, sectionY), protocol)
	}
	return data.Bytes()
}

func writeSection766(data *bytes.Buffer, palette []limbgo.BlockState, section *limbgo.ChunkSection, protocol int32) {
	_ = wire.WriteShort(data, int16(nonAirBlockCountModern(palette, section, protocol)))
	writeBlockStates766(data, palette, section, protocol)
	writeBiomes766(data)
}

func writeBlockStates766(data *bytes.Buffer, palette []limbgo.BlockState, section *limbgo.ChunkSection, protocol int32) {
	_ = wire.WriteByte(data, 4)
	localPalette := buildLocalPaletteModern(palette, section, protocol)
	_ = wire.WriteVarInt(data, int32(len(localPalette)))
	for _, state := range localPalette {
		_ = wire.WriteVarInt(data, int32(state))
	}
	indexByState := make(map[uint32]uint64, len(localPalette))
	for i, state := range localPalette {
		indexByState[state] = uint64(i)
	}
	longs := packSectionPaletteIndicesModern(palette, section, indexByState, protocol)
	_ = wire.WriteVarInt(data, int32(len(longs)))
	for _, value := range longs {
		_ = wire.WriteLong(data, int64(value))
	}
}

func writeBiomes766(data *bytes.Buffer) {
	_ = wire.WriteByte(data, 0)
	_ = wire.WriteVarInt(data, 0)
	_ = wire.WriteVarInt(data, 0)
}

func buildLocalPaletteModern(palette []limbgo.BlockState, section *limbgo.ChunkSection, protocol int32) []uint32 {
	seen := map[uint32]struct{}{0: {}}
	local := []uint32{0}
	if section == nil {
		return local
	}
	for _, paletteID := range section.BlockStateIDs {
		state := blockStateForProtocol(protocol, palette, paletteID)
		if _, ok := seen[state]; ok {
			continue
		}
		seen[state] = struct{}{}
		local = append(local, state)
	}
	return local
}

func packSectionPaletteIndicesModern(palette []limbgo.BlockState, section *limbgo.ChunkSection, indexByState map[uint32]uint64, protocol int32) []uint64 {
	const bitsPerBlock = 4
	const valuesPerLong = 64 / bitsPerBlock
	longs := make([]uint64, 16*16*16/valuesPerLong)
	for i := 0; i < 16*16*16; i++ {
		state := uint32(0)
		if section != nil && i < len(section.BlockStateIDs) {
			state = blockStateForProtocol(protocol, palette, section.BlockStateIDs[i])
		}
		paletteIndex := indexByState[state] & 0xf
		longIndex := i / valuesPerLong
		bitOffset := uint((i % valuesPerLong) * bitsPerBlock)
		longs[longIndex] |= paletteIndex << bitOffset
	}
	return longs
}

func nonAirBlockCountModern(palette []limbgo.BlockState, section *limbgo.ChunkSection, protocol int32) int {
	if section == nil {
		return 0
	}
	count := 0
	for _, paletteID := range section.BlockStateIDs {
		if blockStateForProtocol(protocol, palette, paletteID) != 0 {
			count++
		}
	}
	return count
}

func writeLightData766(data *bytes.Buffer) {
	writeBitSet(data, []uint64{0x3ffff})
	writeBitSet(data, []uint64{0})
	writeBitSet(data, []uint64{0})
	writeBitSet(data, []uint64{0x3ffff})
	_ = wire.WriteVarInt(data, 18)
	for i := 0; i < 18; i++ {
		_ = wire.WriteVarInt(data, 2048)
		data.Write(bytes.Repeat([]byte{0xff}, 2048))
	}
	_ = wire.WriteVarInt(data, 0)
}

func writeBitSet(data *bytes.Buffer, values []uint64) {
	_ = wire.WriteVarInt(data, int32(len(values)))
	for _, value := range values {
		_ = wire.WriteLong(data, int64(value))
	}
}

func boolByte(value bool) byte {
	if value {
		return 1
	}
	return 0
}

func writeUUID(data *bytes.Buffer, uuid string) error {
	cleaned := strings.ReplaceAll(uuid, "-", "")
	raw, err := hex.DecodeString(cleaned)
	if err != nil {
		return fmt.Errorf("decode uuid %q: %w", uuid, err)
	}
	if len(raw) != 16 {
		return fmt.Errorf("uuid %q decoded to %d bytes", uuid, len(raw))
	}
	_, err = data.Write(raw)
	return err
}
