package limbgo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFileConfigProtocolAndStatusFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "limbgo.json")
	raw := []byte(`{
  "listen": ":25570",
  "status": {
    "description": "configured limbo",
    "max_players": 7
  },
  "protocol": {
    "modern_protocols": "modern.json",
    "registry_data": "registry.json"
  },
  "world": {
    "id": "spawn",
    "schematic": "spawn.schem"
  },
  "spawn": {
    "pos": { "x": 1, "y": 65, "z": 2 },
    "mode": 2
  }
}`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadFileConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Status.Description != "configured limbo" {
		t.Fatalf("status.description = %q", cfg.Status.Description)
	}
	if cfg.Status.MaxPlayers != 7 {
		t.Fatalf("status.max_players = %d", cfg.Status.MaxPlayers)
	}
	if cfg.Protocol.ModernProtocols != "modern.json" {
		t.Fatalf("protocol.modern_protocols = %q", cfg.Protocol.ModernProtocols)
	}
	if cfg.Protocol.RegistryData != "registry.json" {
		t.Fatalf("protocol.registry_data = %q", cfg.Protocol.RegistryData)
	}
	if cfg.Spawn.World != "spawn" {
		t.Fatalf("spawn.world default = %q", cfg.Spawn.World)
	}
}

func TestLoadFileConfigStatusDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "limbgo.json")
	raw := []byte(`{
  "world": {
    "schematic": "spawn.schem"
  }
}`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadFileConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Listen != ":25565" {
		t.Fatalf("listen default = %q", cfg.Listen)
	}
	if cfg.Status.Description != "limbgo" {
		t.Fatalf("status.description default = %q", cfg.Status.Description)
	}
	if cfg.Status.MaxPlayers != 1 {
		t.Fatalf("status.max_players default = %d", cfg.Status.MaxPlayers)
	}
}

func TestLoadFileConfigStatusMOTDFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "limbgo.json")
	raw := []byte(`{
  "status": {
    "motd_minimessage": "<gold>limbgo</gold>",
    "version_name": "limbgo status",
    "version_protocol": 771,
    "max_players": 100,
    "online_players": 3,
    "sample_players": [{"name": "Score2", "id": "00000000-0000-0000-0000-000000000002"}],
    "hide_players": true,
    "favicon": "data:image/png;base64,AAAA",
    "enforces_secure_chat": true,
    "previews_chat": false,
    "prevents_chat_reports": true,
    "rate_limit": {
      "requests": 12,
      "window_millis": 500
    }
  },
  "world": {
    "schematic": "spawn.schem"
  }
}`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadFileConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	component, err := cfg.Status.Component()
	if err != nil {
		t.Fatalf("status component: %v", err)
	}
	options := cfg.Status.Options(component)
	if options.VersionName != "limbgo status" {
		t.Fatalf("version name = %q", options.VersionName)
	}
	if options.Protocol != 771 {
		t.Fatalf("version protocol = %d", options.Protocol)
	}
	if !options.HidePlayers {
		t.Fatalf("hide players = false")
	}
	if options.EnforcesSecureChat == nil || !*options.EnforcesSecureChat {
		t.Fatalf("enforces secure chat = %v", options.EnforcesSecureChat)
	}
	if options.PreviewsChat == nil || *options.PreviewsChat {
		t.Fatalf("previews chat = %v", options.PreviewsChat)
	}
	if options.PreventsChatReports == nil || !*options.PreventsChatReports {
		t.Fatalf("prevents chat reports = %v", options.PreventsChatReports)
	}
	if limiter := cfg.Status.RateLimit.RateLimiter(); limiter == nil {
		t.Fatalf("rate limiter = nil")
	}
}

func TestLoadFileConfigDimensionFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "limbgo.json")
	raw := []byte(`{
  "world": {
    "id": "nether",
    "schematic": "spawn.schem",
    "dimension": {
      "environment": "nether",
      "time": 18000,
      "world_age": 42,
      "ambient_light": 0.2,
      "has_skylight": false,
      "monster_spawn_light_level": 7
    }
  }
}`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadFileConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	dimension := cfg.Dimension(32)
	if dimension.Environment != DimensionNether {
		t.Fatalf("environment = %q, want nether", dimension.Environment)
	}
	if dimension.Name != "minecraft:the_nether" {
		t.Fatalf("dimension name = %q", dimension.Name)
	}
	if dimension.HasSkylight {
		t.Fatalf("has_skylight = true, want false")
	}
	if dimension.TimeOfDay == nil || *dimension.TimeOfDay != 18000 {
		t.Fatalf("time = %v, want 18000", dimension.TimeOfDay)
	}
	if dimension.WorldAge != 42 {
		t.Fatalf("world_age = %d, want 42", dimension.WorldAge)
	}
	if dimension.MonsterSpawn.LightLevel.Value == nil || *dimension.MonsterSpawn.LightLevel.Value != 7 {
		t.Fatalf("monster spawn light level = %+v, want fixed 7", dimension.MonsterSpawn.LightLevel)
	}
}
