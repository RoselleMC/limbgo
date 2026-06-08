package limbgo

import (
	"context"
	"net"
)

// Player describes the identity known after the handshake/login stage.
type Player struct {
	Name            string
	UUID            string
	ProtocolVersion int
	RemoteAddr      net.Addr
	Properties      map[string]string
}

// Vec3 is a Minecraft world position.
type Vec3 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// Rotation is the yaw/pitch pair used when spawning a player.
type Rotation struct {
	Yaw   float32 `json:"yaw"`
	Pitch float32 `json:"pitch"`
}

// SpawnTarget is the resolved destination for a player entering limbo.
type SpawnTarget struct {
	World    string
	Position Vec3
	Rotation Rotation
	GameMode GameMode
}

// GameMode is the small subset of vanilla game modes relevant to a limbo spawn.
type GameMode int

const (
	GameModeSurvival GameMode = iota
	GameModeCreative
	GameModeAdventure
	GameModeSpectator
)

// SpawnResolver decides where a player should enter limbo.
type SpawnResolver interface {
	ResolveSpawn(ctx context.Context, player Player) (SpawnTarget, error)
}

// SpawnResolverFunc adapts a function to SpawnResolver.
type SpawnResolverFunc func(context.Context, Player) (SpawnTarget, error)

// ResolveSpawn implements SpawnResolver.
func (fn SpawnResolverFunc) ResolveSpawn(ctx context.Context, player Player) (SpawnTarget, error) {
	return fn(ctx, player)
}

// StaticSpawn returns the same spawn target for every player.
func StaticSpawn(target SpawnTarget) SpawnResolver {
	return SpawnResolverFunc(func(context.Context, Player) (SpawnTarget, error) {
		return target, nil
	})
}
