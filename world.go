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
	MinY               int32
	Height             int32
	Natural            bool
	HasSkylight        bool
	AmbientLight       float32
	FixedTime          *int64
	CoordinateScale    float64
	RespawnAnchorWorks bool
	BedWorks           bool
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
