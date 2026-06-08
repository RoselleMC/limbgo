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
