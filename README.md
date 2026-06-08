# limbgo

`limbgo` is a Go-first Minecraft Java Edition limbo server project. The target
surface is intentionally tiny: accept players, place them in a precomputed
schematic-backed or default bedrock world, send enough chunk data for the client
to render, and keep the connection alive.

The project is designed for two runtime shapes:

- an embeddable Go API for projects that want to decide each player's world and
  spawn position dynamically;
- a standalone binary with a small JSON config containing the listen address,
  status text, spawn position, optional schematic file, and optional generated
  protocol data overrides.

## Compatibility Strategy

The protocol layer must not be a hand-maintained list of packet IDs and field
layouts. The intended implementation is:

- keep the runtime server and API in Go;
- generate protocol adapters from external Minecraft protocol/version data, such
  as PrismarineJS `minecraft-data`;
- keep world data version-neutral internally, then translate block palettes,
  registry data, login packets, and chunk packets at the protocol edge;
- use the client-sent protocol number from the handshake to select the adapter;
- for unknown future versions, try the newest compatible adapter only when the
  generated data marks the packet shapes as unchanged.

This keeps manual maintenance concentrated in generator rules and compatibility
tests, not in per-version packet tables.

Generated artifacts are produced from `minecraft-data/data/pc`:

- `protocol/versions/versions_gen.go`
- `protocol/packetid/packetid_gen.go`
- `protocol/blockstate/blockstate_gen.go`
- `protocol/registrydata/registrydata.json`

Version-specific compatibility switches for the modern configuration/play flow
live in `protocol/limbo/modern_protocols.json`, so adding a compatible protocol
line should normally start as data/config work before adding Go code. When a
new protocol keeps the same packet/data shape, `packet_id_protocol` and
`data_protocol` can point at the latest generated baseline.

Regenerate them with:

```sh
MINECRAFT_DATA_PC_DIR=/path/to/minecraft-data/data/pc go generate ./protocol/...
```

## Verification

Run the Go unit tests with:

```sh
go test ./...
```

For client-facing smoke coverage, `tools/js-smoke/chunk-check.mjs` starts a
temporary limbgo server, joins with `minecraft-protocol` clients, reads each
received `map_chunk`, and verifies the first block state decodes to stone:

```sh
node tools/js-smoke/chunk-check.mjs
```

Dialog API smoke coverage starts a temporary server that uses `Config.Events`,
`session.ShowDialog`, `session.ClearDialog`, and `DialogClick`, then drives it
with raw JavaScript fake clients across dialog-capable protocol lines:

```sh
node tools/js-smoke/dialog-check.mjs
```

## API

See [API.md](API.md) for the embeddable Go API.

## Deployment Config

The standalone command expects a small JSON config:

```json
{
  "listen": ":25565",
  "status": {
    "description": "limbgo",
    "motd_minimessage": "<gradient:#55ff55:#55ffff><bold>limbgo</bold></gradient>",
    "version_name": "limbgo",
    "max_players": 100,
    "online_players": 0,
    "sample_players": [
      { "name": "limbgo", "id": "00000000-0000-0000-0000-000000000000" }
    ],
    "enforces_secure_chat": false,
    "prevents_chat_reports": true,
    "rate_limit": {
      "requests": 60,
      "window_millis": 1000
    }
  },
  "protocol": {
    "modern_protocols": "protocol/limbo/modern_protocols.json",
    "registry_data": "protocol/registrydata/registrydata.json"
  },
  "world": {
    "id": "spawn",
    "schematic": "spawn.schem",
    "dimension": {
      "environment": "overworld",
      "time": 6000,
      "world_age": 0,
      "fixed_time": 6000,
      "ambient_light": 0,
      "has_skylight": true,
      "has_ceiling": false,
      "ultrawarm": false,
      "natural": true,
      "piglin_safe": false,
      "respawn_anchor_works": false,
      "bed_works": true,
      "has_raids": true,
      "coordinate_scale": 1,
      "logical_height": 256,
      "infiniburn": "#minecraft:infiniburn_overworld",
      "effects": "minecraft:overworld",
      "monster_spawn_block_light_limit": 0,
      "monster_spawn_light_min": 0,
      "monster_spawn_light_max": 7
    }
  },
  "spawn": {
    "world": "spawn",
    "pos": { "x": 0, "y": 65, "z": 0 },
    "look": { "yaw": 0, "pitch": 0 },
    "mode": 2
  }
}
```

The `protocol` paths are optional. When omitted, the binary uses the embedded
generated defaults. Supplying them is useful when testing a new protocol line or
regenerated registry data without changing Go source.

`world.schematic` is optional. When it is set, `world/schematic` loads the
Sponge `.schem` file into a version-neutral world palette. Protocol adapters
translate that palette to client-specific block state IDs at chunk serialization
time. When it is omitted, the standalone binary uses `DefaultWorld`: a
minimal air world with one `minecraft:bedrock` block directly below the spawn
position. If the config also omits `spawn.pos`, the default spawn is
`{ "x": 0, "y": 65, "z": 0 }`, so the bedrock block is at `0,64,0`.

The `world.dimension` block is optional. `environment` accepts `overworld`,
`nether`, or `end` and fills vanilla-like defaults for limbo-relevant client
state. Individual fields can then override the preset. Even without mobs,
portals, or beds, modern clients receive dimension type registry data, so limbgo
tracks the properties that affect rendering or client behavior: `fixed_time`,
`time`, `world_age`, `ambient_light`, `has_skylight`, `has_ceiling`, `ultrawarm`,
`natural`, `piglin_safe`, `respawn_anchor_works`, `bed_works`, `has_raids`,
`coordinate_scale`, `logical_height`, `infiniburn`, `effects`, and monster spawn
light fields. Legacy clients receive the matching dimension id for overworld,
nether, or end.

## Current Protocol State

The current play-state adapters support Minecraft Java protocol `47`
(`1.8.x`), protocol `340` (`1.12.2`), protocols `757`-`774`
(`1.18` through `1.21.11`), and protocol `775` (`26.1` through `26.1.2`) as
early end-to-end baselines:

- offline-mode login success;
- join game;
- spawn position;
- player position;
- one spawn chunk using the generated packet ID table and a small legacy
  block-state translator. Protocol 47 uses the pre-palette chunk format;
  protocol 340 uses a 4-bit section palette and packed long array; protocols
  757-763 use the pre-configuration modern login packet with generated dimension
  codec data; protocols 764-765 use the modern configuration phase with a
  generated legacy dimension codec; protocol 766 and the compatible 1.21
  protocol lines use generated minimal biome/chat/damage registry data, a
  runtime dimension_type, heightmaps, light data, and modern paletted chunk
  sections. Protocol 770+ uses the newer heightmap array / ByteArray chunk
  packet shape. Protocol 775 currently uses the configured compatibility alias
  to reuse protocol 774 packet IDs and generated data.

Newer protocol lines are intentionally still rejected during login until their
generated serializers are implemented. Server-list status/ping works through the
shared router independently of play support.
