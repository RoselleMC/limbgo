package limbgo

import "context"

// WorldProvider resolves worlds by ID. Deployments can use one schematic-backed
// world, while API users can route players to different preloaded worlds.
type WorldProvider interface {
	World(ctx context.Context, id string) (World, error)
}

// World is the protocol-facing view of a precomputed limbo map.
type World interface {
	ID() string
	Dimension() Dimension
	BlockPalette() []BlockState
	Chunk(x int32, z int32) (Chunk, bool)
}

// Dimension contains only the properties needed to serialize login and chunks.
type Dimension struct {
	Name               string
	Environment        DimensionEnvironment
	MinY               int32
	Height             int32
	LogicalHeight      int32
	Natural            bool
	HasSkylight        bool
	HasCeiling         bool
	UltraWarm          bool
	AmbientLight       float32
	FixedTime          *int64
	TimeOfDay          *int64
	WorldAge           int64
	CoordinateScale    float64
	RespawnAnchorWorks bool
	BedWorks           bool
	PiglinSafe         bool
	HasRaids           bool
	Infiniburn         string
	Effects            string
	MonsterSpawn       MonsterSpawnSettings
}

// DimensionEnvironment selects the vanilla visual/behavior preset for a limbo
// world.
type DimensionEnvironment string

const (
	DimensionOverworld DimensionEnvironment = "overworld"
	DimensionNether    DimensionEnvironment = "nether"
	DimensionEnd       DimensionEnvironment = "end"
)

// MonsterSpawnSettings mirrors the dimension_type monster settings. Limbo does
// not spawn mobs, but clients require these fields in modern registries.
type MonsterSpawnSettings struct {
	BlockLightLimit int32
	LightLevel      IntProvider
}

// IntProvider is the small integer provider shape needed by dimension_type.
type IntProvider struct {
	Value        *int32
	MinInclusive *int32
	MaxInclusive *int32
}

// FixedInt returns a constant integer provider.
func FixedInt(value int32) IntProvider {
	return IntProvider{Value: &value}
}

// UniformInt returns a minecraft:uniform integer provider.
func UniformInt(minInclusive, maxInclusive int32) IntProvider {
	return IntProvider{MinInclusive: &minInclusive, MaxInclusive: &maxInclusive}
}

// DimensionPreset returns vanilla-like dimension defaults for a limbo world.
// If height is positive, it overrides the preset height and logical height.
func DimensionPreset(environment DimensionEnvironment, height int32) Dimension {
	height = normalizeDimensionHeight(height)
	switch environment {
	case DimensionNether:
		fixedTime := int64(18000)
		return Dimension{
			Name:               "minecraft:the_nether",
			Environment:        DimensionNether,
			MinY:               0,
			Height:             fallbackHeight(height, 256),
			LogicalHeight:      128,
			Natural:            false,
			HasSkylight:        false,
			HasCeiling:         true,
			UltraWarm:          true,
			AmbientLight:       0.1,
			FixedTime:          &fixedTime,
			CoordinateScale:    8,
			RespawnAnchorWorks: true,
			BedWorks:           false,
			PiglinSafe:         true,
			HasRaids:           false,
			Infiniburn:         "#minecraft:infiniburn_nether",
			Effects:            "minecraft:the_nether",
			MonsterSpawn: MonsterSpawnSettings{
				BlockLightLimit: 15,
				LightLevel:      FixedInt(7),
			},
		}
	case DimensionEnd:
		fixedTime := int64(6000)
		resolvedHeight := fallbackHeight(height, 256)
		return Dimension{
			Name:               "minecraft:the_end",
			Environment:        DimensionEnd,
			MinY:               0,
			Height:             resolvedHeight,
			LogicalHeight:      resolvedHeight,
			Natural:            false,
			HasSkylight:        false,
			HasCeiling:         false,
			UltraWarm:          false,
			AmbientLight:       0,
			FixedTime:          &fixedTime,
			CoordinateScale:    1,
			RespawnAnchorWorks: false,
			BedWorks:           false,
			PiglinSafe:         false,
			HasRaids:           true,
			Infiniburn:         "#minecraft:infiniburn_end",
			Effects:            "minecraft:the_end",
			MonsterSpawn: MonsterSpawnSettings{
				BlockLightLimit: 0,
				LightLevel:      UniformInt(0, 7),
			},
		}
	default:
		resolvedHeight := fallbackHeight(height, 256)
		return Dimension{
			Name:               "minecraft:overworld",
			Environment:        DimensionOverworld,
			MinY:               0,
			Height:             resolvedHeight,
			LogicalHeight:      resolvedHeight,
			Natural:            true,
			HasSkylight:        true,
			HasCeiling:         false,
			UltraWarm:          false,
			AmbientLight:       0,
			CoordinateScale:    1,
			RespawnAnchorWorks: false,
			BedWorks:           true,
			PiglinSafe:         false,
			HasRaids:           true,
			Infiniburn:         "#minecraft:infiniburn_overworld",
			Effects:            "minecraft:overworld",
			MonsterSpawn: MonsterSpawnSettings{
				BlockLightLimit: 0,
				LightLevel:      UniformInt(0, 7),
			},
		}
	}
}

// NormalizeDimension fills protocol-required zero-value fields while preserving
// explicit values that have already been set by API callers.
func NormalizeDimension(d Dimension, schematicHeight int32) Dimension {
	if d.Environment == "" {
		d.Environment = inferDimensionEnvironment(d.Name)
	}
	if d.Name == "" {
		return DimensionPreset(d.Environment, schematicHeight)
	}
	if d.Height == 0 {
		d.Height = normalizeDimensionHeight(schematicHeight)
	}
	if d.Height == 0 {
		d.Height = 256
	}
	if d.LogicalHeight == 0 {
		d.LogicalHeight = d.Height
	}
	if d.CoordinateScale == 0 {
		d.CoordinateScale = 1
	}
	if d.Infiniburn == "" || d.Effects == "" || d.MonsterSpawn.LightLevel.empty() {
		preset := DimensionPreset(d.Environment, d.Height)
		if d.Infiniburn == "" {
			d.Infiniburn = preset.Infiniburn
		}
		if d.Effects == "" {
			d.Effects = preset.Effects
		}
		if d.MonsterSpawn.LightLevel.empty() {
			d.MonsterSpawn = preset.MonsterSpawn
		}
	}
	return d
}

func inferDimensionEnvironment(name string) DimensionEnvironment {
	switch name {
	case "minecraft:the_nether":
		return DimensionNether
	case "minecraft:the_end":
		return DimensionEnd
	default:
		return DimensionOverworld
	}
}

func normalizeDimensionHeight(height int32) int32 {
	if height <= 0 {
		return 0
	}
	if remainder := height % 16; remainder != 0 {
		height += 16 - remainder
	}
	return height
}

func fallbackHeight(height, fallback int32) int32 {
	if height > 0 {
		return height
	}
	return fallback
}

func (p IntProvider) empty() bool {
	return p.Value == nil && p.MinInclusive == nil && p.MaxInclusive == nil
}

// Chunk is intentionally version-neutral. Protocol adapters translate it into
// the client-specific chunk packet shape.
type Chunk struct {
	X        int32
	Z        int32
	MinY     int32
	Sections []ChunkSection
}

// ChunkSection stores palette IDs in a compact, version-neutral form.
type ChunkSection struct {
	Y             int32
	BlockStateIDs []uint32
	BiomeIDs      []uint32
}

// BlockState is a version-neutral Minecraft block state.
type BlockState struct {
	Name       string
	Properties map[string]string
}

// MemoryWorld is a small immutable World implementation useful for embedded API users.
type MemoryWorld struct {
	WorldID        string
	WorldDimension Dimension
	Palette        []BlockState
	Chunks         map[ChunkPos]Chunk
}

// ChunkPos identifies a chunk in a world.
type ChunkPos struct {
	X int32
	Z int32
}

// ID returns the world ID.
func (w *MemoryWorld) ID() string {
	return w.WorldID
}

// Dimension returns the world dimension.
func (w *MemoryWorld) Dimension() Dimension {
	return w.WorldDimension
}

// BlockPalette returns the world's block palette.
func (w *MemoryWorld) BlockPalette() []BlockState {
	out := make([]BlockState, len(w.Palette))
	copy(out, w.Palette)
	return out
}

// Chunk returns a chunk by position.
func (w *MemoryWorld) Chunk(x int32, z int32) (Chunk, bool) {
	chunk, ok := w.Chunks[ChunkPos{X: x, Z: z}]
	return chunk, ok
}

// StaticWorldProvider serves a fixed set of worlds from memory.
type StaticWorldProvider map[string]World

// World resolves a world by ID.
func (p StaticWorldProvider) World(_ context.Context, id string) (World, error) {
	world, ok := p[id]
	if !ok {
		return nil, ErrWorldNotFound
	}
	return world, nil
}
