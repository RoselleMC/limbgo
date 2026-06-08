package limbgo

import (
	"encoding/json"
	"fmt"
	"os"
)

// FileConfig is the simple deployable configuration format.
type FileConfig struct {
	Listen string `json:"listen"`
	Status struct {
		Description string `json:"description"`
		MaxPlayers  int    `json:"max_players"`
	} `json:"status"`
	Protocol struct {
		ModernProtocols string `json:"modern_protocols"`
		RegistryData    string `json:"registry_data"`
	} `json:"protocol"`
	World struct {
		ID        string `json:"id"`
		Schematic string `json:"schematic"`
	} `json:"world"`
	Spawn struct {
		World string   `json:"world"`
		Pos   Vec3     `json:"pos"`
		Look  Rotation `json:"look"`
		Mode  GameMode `json:"mode"`
	} `json:"spawn"`
}

// LoadFileConfig reads a JSON deployment config.
func LoadFileConfig(path string) (FileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return FileConfig{}, err
	}
	var cfg FileConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return FileConfig{}, err
	}
	if cfg.Listen == "" {
		cfg.Listen = ":25565"
	}
	if cfg.Status.Description == "" {
		cfg.Status.Description = "limbgo"
	}
	if cfg.Status.MaxPlayers <= 0 {
		cfg.Status.MaxPlayers = 1
	}
	if cfg.World.ID == "" {
		cfg.World.ID = "default"
	}
	if cfg.Spawn.World == "" {
		cfg.Spawn.World = cfg.World.ID
	}
	if cfg.World.Schematic == "" {
		return FileConfig{}, fmt.Errorf("limbgo: world.schematic is required")
	}
	return cfg, nil
}

// SpawnTarget returns the static spawn described by the file.
func (cfg FileConfig) SpawnTarget() SpawnTarget {
	return SpawnTarget{
		World:    cfg.Spawn.World,
		Position: cfg.Spawn.Pos,
		Rotation: cfg.Spawn.Look,
		GameMode: cfg.Spawn.Mode,
	}
}
