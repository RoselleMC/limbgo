# limbgo API

This document covers the embeddable Go API. The standalone binary remains a
thin deployment wrapper around the same server, world, spawn, protocol, and
event surfaces.

## Server Setup

Create a server with `limbgo.NewServer` and provide a protocol router, world
source, spawn resolver, status policy, and optional player event handler:

```go
motd, err := limbgo.ParseMiniMessage("<gold>limbgo</gold>")
if err != nil {
	return err
}
srv, err := limbgo.NewServer(limbgo.Config{
	Addr: ":25565",
	ProtocolRouter: limbo.Router{
		MOTD:              motd,
		StatusRateLimiter: limbgo.NewRateLimiter(limbgo.RateLimitConfig{}),
	},
	Worlds:        worlds,
	SpawnResolver: limbgo.StaticSpawn(spawn),
	Events:        events,
})
if err != nil {
	return err
}
return srv.ListenAndServe(ctx)
```

`Config.Worlds` is used by the default world resolver. For applications that
need to select a different world instance per player, prefer `JoinResolver`.

```go
srv, err := limbgo.NewServer(limbgo.Config{
	Addr:           ":25565",
	ProtocolRouter: limbo.Router{},
	JoinResolver: limbgo.JoinResolverFunc(func(ctx context.Context, player limbgo.Player) (limbgo.JoinTarget, error) {
		world := pickWorldFor(player)
		return limbgo.JoinTarget{
			World: world,
			Spawn: limbgo.SpawnTarget{
				Position: limbgo.Vec3{X: 0, Y: 65, Z: 0},
				GameMode: limbgo.GameModeAdventure,
			},
		}, nil
	}),
})
```

`JoinTarget.World` is a full `World` object. The schematic is only one possible
data source for that object; world time, dimension/environment, height, logical
height, skylight, ambient light, coordinate scale, visual effects, and spawn
metadata travel with the world instance. If a resolver owns temporary per-player
worlds, it may also implement `JoinReleaser` to clean them up when the
connection closes.

`Dimension` intentionally exposes only the subset that matters for a limbo
login/chunk view. Modern clients still receive complete `dimension_type`
registry data, but gameplay-only protocol fields such as bed behavior, raids,
piglin safety, infiniburn, and monster spawn settings are derived internally
from vanilla-like presets instead of being API or file-config inputs.

For a zero-asset limbo world, use `DefaultWorld` and `DefaultSpawn`:

```go
spawn := limbgo.DefaultSpawn("default")
world := limbgo.DefaultWorld("default")
```

The default world is air plus one `minecraft:bedrock` block directly below the
spawn position. The standalone binary uses this same world when
`world.schematic` is omitted. If both `world.schematic` and `spawn.pos` are
omitted in the file config, `spawn.pos` defaults to `{X: 0, Y: 65, Z: 0}` and
the bedrock block is placed at `0,64,0`.

Use `DefaultWorldWithDimension` when you want the same no-schematic world but
with custom dimension properties:

```go
world := limbgo.DefaultWorldWithDimension("nether-login", limbgo.DimensionPreset(limbgo.DimensionNether, 256))
```

## Status And MOTD

For static server-list data, set fields directly on `limbo.Router`:

```go
motd, err := limbgo.ParseMiniMessage("<gradient:#55ff55:#55ffff>limbgo</gradient>")
if err != nil {
	return err
}
router := limbo.Router{
	MOTD:                motd,
	VersionName:         "limbgo",
	MaxPlayers:          100,
	OnlinePlayers:       3,
	SamplePlayers:       []limbgo.StatusSamplePlayer{{Name: "Score2", ID: "00000000-0000-0000-0000-000000000002"}},
	EnforcesSecureChat:  limbgo.Bool(false),
	PreventsChatReports: limbgo.Bool(true),
	StatusRateLimiter:   limbgo.NewRateLimiter(limbgo.RateLimitConfig{}),
}
```

For dynamic MOTD, player samples, favicon, or protocol-specific status, provide
a `StatusProvider`:

```go
router := limbo.Router{
	StatusProvider: limbgo.StatusProviderFunc(func(ctx context.Context, req limbgo.StatusRequest) (limbgo.Status, error) {
		return limbgo.Status{
			VersionName: "limbgo",
			Protocol:    req.Protocol,
			Description: &component.Text{Content: "Welcome " + req.Address},
			MaxPlayers:  100,
		}, nil
	}),
	StatusRateLimiter: limbgo.NewRateLimiter(limbgo.RateLimitConfig{
		Requests: 60,
		Window:   time.Second,
	}),
}
```

`StatusRequest` includes the handshake protocol, requested address/port, and
remote address. This keeps MOTD logic in API code rather than in protocol
adapters.

## Player Events

Attach `Config.Events` to observe player input after the limbo join sequence has
completed. If `Events` is nil, limbgo keeps the old minimal behavior and returns
after writing the join/chunk sequence.

```go
events := limbgo.PlayerEventHandlerFuncs{
	Chat: func(ctx context.Context, session limbgo.PlayerSession, event *limbgo.ChatEvent) error {
		return session.SendMessage(ctx, &component.Text{Content: "chat accepted"})
	},
	Command: func(ctx context.Context, session limbgo.PlayerSession, event *limbgo.CommandEvent) error {
		return session.SendMessage(ctx, &component.Text{Content: "command accepted"})
	},
	DialogClick: func(ctx context.Context, session limbgo.PlayerSession, event *limbgo.DialogClickEvent) error {
		return session.SendMessage(ctx, &component.Text{Content: event.ID})
	},
}
```

The handler interface is:

```go
type PlayerEventHandler interface {
	HandleChat(ctx context.Context, session PlayerSession, event *ChatEvent) error
	HandleCommand(ctx context.Context, session PlayerSession, event *CommandEvent) error
	HandleDialogClick(ctx context.Context, session PlayerSession, event *DialogClickEvent) error
}
```

`PlayerEventHandlerFuncs` is a convenience adapter. For stateful applications,
define your own type implementing `PlayerEventHandler`.

## Player Session

Event handlers receive a `PlayerSession`:

```go
type PlayerSession interface {
	Player() Player
	Capabilities() SessionCapabilities
	SendMessage(ctx context.Context, message component.Component) error
	SendActionBar(ctx context.Context, message component.Component) error
	ShowTitle(ctx context.Context, title Title) error
	ClearTitle(ctx context.Context, reset bool) error
	ShowDialog(ctx context.Context, dialog dialog.Dialog) error
	ClearDialog(ctx context.Context) error
	StoreCookie(ctx context.Context, key string, value []byte) error
	Transfer(ctx context.Context, host string, port int) error
	Disconnect(ctx context.Context, reason component.Component) error
}

type SessionCapabilities struct {
	SystemMessage bool
	ActionBar     bool
	Title         bool
	Dialog        bool
	StoreCookie   bool
	Transfer      bool
	Disconnect    bool
}
```

`Player()` returns the connected player metadata. `SendMessage` writes a system
message. `SendActionBar`, `ShowTitle`, and `ClearTitle` write the vanilla
message overlay packets. `ShowDialog` and `ClearDialog` are available for
clients whose protocol contains the official dialog packets.

`Capabilities()` lets portal code branch on vanilla feature support without
touching protocol numbers. If a session method is called when unsupported, it
returns `ErrUnsupportedCapability`.

Auth portals can complete a vanilla transfer flow without protocol-specific
code:

```go
events := limbgo.PlayerEventHandlerFuncs{
	Command: func(ctx context.Context, session limbgo.PlayerSession, event *limbgo.CommandEvent) error {
		if event.Command != "login-ok" {
			return nil
		}
		caps := session.Capabilities()
		if !caps.StoreCookie || !caps.Transfer {
			return session.Disconnect(ctx, &component.Text{Content: "Please use Minecraft 1.20.5+"})
		}
		if err := session.StoreCookie(ctx, "authman:transfer", grantToken); err != nil {
			return err
		}
		return session.Transfer(ctx, "play.example.net", 25565)
	},
}
```

`StoreCookie` writes a vanilla store-cookie packet, `Transfer` writes the
vanilla transfer packet, and `Disconnect` writes a rich-text kick reason before
closing the connection.

## Rich Text

limbgo uses Minekube rich text components instead of defining its own text
model:

```go
import "go.minekube.com/common/minecraft/component"
```

Any API field that takes `component.Component` supports rich text. That includes
chat/system messages, actionbar messages, title/subtitle overlays, and dialog
title, external title, body text, button labels, button tooltips, input labels,
and option display labels.

Protocol adapters serialize rich text as JSON for older clients and anonymous
NBT for modern clients that require it.

MiniMessage can be parsed with:

```go
message, err := limbgo.ParseMiniMessage("<red><bold>Hello</bold></red>")
```

The parser is lenient: malformed or currently unrepresentable tags remain
literal text instead of crashing the connection.

## Actionbar And Titles

Actionbar and title APIs use the same Minekube component model as chat and
dialogs:

```go
events := limbgo.PlayerEventHandlerFuncs{
	Command: func(ctx context.Context, session limbgo.PlayerSession, event *limbgo.CommandEvent) error {
		if event.Command != "notice" {
			return nil
		}
		action, err := limbgo.ParseMiniMessage("<green>Login accepted</green>")
		if err != nil {
			return err
		}
		title, err := limbgo.ParseMiniMessage("<gold><bold>Welcome</bold></gold>")
		if err != nil {
			return err
		}
		subtitle := &component.Text{Content: "Preparing transfer"}
		if session.Capabilities().ActionBar {
			if err := session.SendActionBar(ctx, action); err != nil {
				return err
			}
		}
		if session.Capabilities().Title {
			return session.ShowTitle(ctx, limbgo.Title{
				Title:    title,
				Subtitle: subtitle,
				Times:    limbgo.TitleTimesTicks(10, 40, 10),
			})
		}
		return nil
	},
}
```

`ClearTitle(ctx, reset)` clears the current title. When `reset` is true, vanilla
clients also reset title timings to their defaults. Protocol adapters send
legacy title action packets for older clients and split actionbar/title packets
for modern clients.

## Chat And Commands

`ChatEvent` is emitted when the player sends chat text. `CommandEvent` is
emitted when the player sends a command or, on legacy protocols, when a chat
message starts with `/`.

```go
type ChatEvent struct {
	Player   Player
	Message  string
	Protocol int
	Canceled bool
}

type CommandEvent struct {
	Player   Player
	Command  string
	Protocol int
	Canceled bool
}
```

The `Command` field does not include the leading `/`.

## Dialog UI

The official dialog UI appears in the generated protocol data after Minecraft
1.21.5. In limbgo terms, use dialog APIs when the client protocol has
`show_dialog`, `clear_dialog`, and `custom_click_action`; current generated data
starts at protocol `771`.

Import the helper package:

```go
import "github.com/RoselleMC/limbgo/dialog"
```

A minimal notice dialog:

```go
err := session.ShowDialog(ctx, dialog.Notice(dialog.Common{
	Title: dialog.Text("Welcome"),
	Body: []dialog.Raw{
		dialog.PlainMessage(dialog.Text("Pick a destination"), 220),
	},
	Pause:       dialog.Bool(false),
	AfterAction: dialog.AfterActionWaitForResponse,
}, dialog.Button(
	dialog.Text("Continue"),
	dialog.DynamicCustom("limbgo:continue", dialog.Raw{"screen": "spawn"}),
)))
```

A richer dialog with Minekube components:

```go
err := session.ShowDialog(ctx, dialog.Notice(dialog.Common{
	Title: &component.Text{
		Content: "Welcome",
		Extra: []component.Component{
			&component.Text{Content: " player"},
		},
	},
	Body: []dialog.Raw{
		dialog.PlainMessage(&component.Text{Content: "Choose an action"}, 220),
	},
	Inputs: []dialog.Raw{
		dialog.TextInput("name", &component.Text{Content: "Name"}, dialog.TextInputOptions{
			Initial:   event.Player.Name,
			MaxLength: 32,
		}),
		dialog.NumberRangeInput("level", &component.Text{Content: "Level"}, dialog.NumberRangeOptions{
			Start:       1,
			End:         10,
			Initial:     dialog.Float(4.5),
			Step:        dialog.Float(0.5),
			LabelFormat: "options.generic_value",
		}),
	},
	CanCloseWithEscape: dialog.Bool(true),
	Pause:              dialog.Bool(false),
	AfterAction:        dialog.AfterActionWaitForResponse,
}, dialog.ActionButton{
	Label:   &component.Text{Content: "Submit"},
	Tooltip: &component.Text{Content: "Send rich payload"},
	Action:  dialog.DynamicCustom("limbgo:submit", dialog.Raw{"source": "spawn"}),
}))
```

To close any currently open dialog:

```go
err := session.ClearDialog(ctx)
```

### Dialog Constructors

Dialog types:

- `dialog.Notice`
- `dialog.Confirmation`
- `dialog.MultiAction`
- `dialog.MultiActionWithExit`
- `dialog.DialogList`
- `dialog.ServerLinks`
- `dialog.ServerLinksWithOptions`

Body helpers:

- `dialog.PlainMessage`
- `dialog.Item`
- `dialog.ItemDescription`
- `dialog.ItemWithDescription`

Input helpers:

- `dialog.TextInput`
- `dialog.BooleanInput`
- `dialog.SingleOptionInput`
- `dialog.SingleOptionInputWithOptions`
- `dialog.NumberRangeInput`

Button and action helpers:

- `dialog.Button`
- `dialog.RunCommand`
- `dialog.SuggestCommand`
- `dialog.OpenURL`
- `dialog.CopyToClipboard`
- `dialog.ChangePage`
- `dialog.ShowDialog`
- `dialog.Custom`
- `dialog.DynamicRunCommand`
- `dialog.DynamicCustom`

Common optional pointer helpers:

- `dialog.Bool`
- `dialog.Float`

### Custom Dialog Actions

Use `dialog.Custom` or `dialog.DynamicCustom` to receive a serverbound
`DialogClickEvent`:

```go
events := limbgo.PlayerEventHandlerFuncs{
	DialogClick: func(ctx context.Context, session limbgo.PlayerSession, event *limbgo.DialogClickEvent) error {
		switch event.ID {
		case "limbgo:submit":
			// event.Payload is the raw anonymous NBT body when the client sends one.
			return session.SendMessage(ctx, dialog.Text("submitted"))
		default:
			return nil
		}
	},
}
```

`DialogClickEvent` contains:

```go
type DialogClickEvent struct {
	Player   Player
	ID       string
	Payload  []byte
	Protocol int
	Canceled bool
}
```

`Payload` is kept as raw anonymous NBT. This avoids baking a long-lived NBT
schema into limbgo while still exposing the full client response to applications
that want to decode it.

## Raw Data And Future Versions

The dialog package intentionally exposes `dialog.Raw`:

```go
raw := dialog.Raw{
	"type":  "minecraft:notice",
	"title": dialog.Text("Config loaded"),
	"body": []dialog.Raw{
		{"type": "minecraft:plain_message", "contents": dialog.Text("Hello")},
	},
}
err := session.ShowDialog(ctx, raw)
```

Use `Raw` for config-loaded dialogs, exact vanilla fields not covered by helper
functions, or future fields added by Minecraft before limbgo grows a typed
wrapper. Nested Minekube `component.Component` values inside `Raw` are still
serialized as rich text.
