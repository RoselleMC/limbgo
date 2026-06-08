package limbo

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
)

//go:embed modern_protocols.json
var rawModernProtocolConfigs []byte

type modernProtocolConfigRecord struct {
	PacketIDProtocol               int32 `json:"packet_id_protocol"`
	DataProtocol                   int32 `json:"data_protocol"`
	PreConfiguration               bool  `json:"pre_configuration"`
	PreConfigurationDimensionNBT   bool  `json:"pre_configuration_dimension_nbt"`
	PreConfigurationDeath          bool  `json:"pre_configuration_death"`
	PreConfigurationPortalCooldown bool  `json:"pre_configuration_portal_cooldown"`
	PositionDismountVehicle        bool  `json:"position_dismount_vehicle"`
	LoginSuccessNoProperties       bool  `json:"login_success_no_properties"`
	StrictErrorHandling            bool  `json:"strict_error_handling"`
	RegistryCodecNBT               bool  `json:"registry_codec_nbt"`
	LegacyPlayLogin                bool  `json:"legacy_play_login"`
	PositionV2                     bool  `json:"position_v2"`
	SpawnInfoSeaLevel              bool  `json:"spawn_info_sea_level"`
	PositionFlagsU32               bool  `json:"position_flags_u32"`
	ChunkHeightmapArray            bool  `json:"chunk_heightmap_array"`
	ChunkHeightmapFullNBT          bool  `json:"chunk_heightmap_full_nbt"`
	ChunkTrustEdges                bool  `json:"chunk_trust_edges"`
}

var (
	modernConfigOnce sync.Once
	modernConfigs    *ModernProtocols
	modernConfigErr  error
)

// ModernProtocols contains the version-specific switches for the modern
// login/configuration/play flow.
type ModernProtocols struct {
	configs map[int32]modernProtocolConfig
}

// DefaultModernProtocols returns the embedded generated/configured protocol
// compatibility table.
func DefaultModernProtocols() (*ModernProtocols, error) {
	modernConfigOnce.Do(func() {
		modernConfigs, modernConfigErr = loadModernProtocolConfigs(rawModernProtocolConfigs)
	})
	return modernConfigs, modernConfigErr
}

// LoadModernProtocolsFile reads a modern protocol compatibility table from a
// JSON file.
func LoadModernProtocolsFile(path string) (*ModernProtocols, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return LoadModernProtocolsBytes(raw)
}

// LoadModernProtocolsBytes reads a modern protocol compatibility table from
// JSON bytes.
func LoadModernProtocolsBytes(raw []byte) (*ModernProtocols, error) {
	return loadModernProtocolConfigs(raw)
}

func (p *ModernProtocols) configFor(protocol int32) (modernProtocolConfig, bool) {
	if p == nil {
		return modernProtocolConfig{}, false
	}
	cfg, ok := p.configs[protocol]
	return cfg, ok
}

func (p *ModernProtocols) supportedProtocols() []int32 {
	if p == nil {
		return nil
	}
	out := make([]int32, 0, len(p.configs))
	for protocol := range p.configs {
		out = append(out, protocol)
	}
	return out
}

func loadModernProtocolConfigs(raw []byte) (*ModernProtocols, error) {
	var records map[string]modernProtocolConfigRecord
	if err := json.Unmarshal(raw, &records); err != nil {
		return nil, fmt.Errorf("parse modern protocol configs: %w", err)
	}
	configs := make(map[int32]modernProtocolConfig, len(records))
	for rawProtocol, record := range records {
		protocol, err := strconv.ParseInt(rawProtocol, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("parse protocol %q: %w", rawProtocol, err)
		}
		configs[int32(protocol)] = modernProtocolConfig{
			protocol:                       int32(protocol),
			packetIDProtocol:               protocolAliasOrSelf(record.PacketIDProtocol, int32(protocol)),
			dataProtocol:                   protocolAliasOrSelf(record.DataProtocol, int32(protocol)),
			preConfiguration:               record.PreConfiguration,
			preConfigurationDimensionNBT:   record.PreConfigurationDimensionNBT,
			preConfigurationDeath:          record.PreConfigurationDeath,
			preConfigurationPortalCooldown: record.PreConfigurationPortalCooldown,
			positionDismountVehicle:        record.PositionDismountVehicle,
			loginSuccessNoProperties:       record.LoginSuccessNoProperties,
			strictErrorHandling:            record.StrictErrorHandling,
			registryCodecNBT:               record.RegistryCodecNBT,
			legacyPlayLogin:                record.LegacyPlayLogin,
			positionV2:                     record.PositionV2,
			spawnInfoSeaLevel:              record.SpawnInfoSeaLevel,
			positionFlagsU32:               record.PositionFlagsU32,
			chunkHeightmapArray:            record.ChunkHeightmapArray,
			chunkHeightmapFullNBT:          record.ChunkHeightmapFullNBT,
			chunkTrustEdges:                record.ChunkTrustEdges,
		}
	}
	return &ModernProtocols{configs: configs}, nil
}

func protocolAliasOrSelf(alias, self int32) int32 {
	if alias != 0 {
		return alias
	}
	return self
}
