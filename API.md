# limbgo API

This document covers the embeddable Go API. The standalone binary remains a
thin deployment wrapper around the same server, world, spawn, protocol, and
event surfaces.

## Server Setup

Create a server with `limbgo.NewServer` and provide a protocol router, world
source, spawn resolver, and optional player event handler:

```go
srv, err := limbgo.NewServer(limbgo.Config{
	Addr:           ":25565",
	ProtocolRouter: limbo.Router{},
	Worlds:         worlds,
	SpawnResolver:  limbgo.StaticSpawn(spawn),
	Events:         events,
})
if err != nil {
	return err
}
return srv.ListenAndServe(ctx)
```

`Config.Worlds` is used by the default world resolver. For per-player routing,
provide a custom `SpawnResolver` or implement the lower-level session services
used by protocol routers.

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
	SendMessage(ctx context.Context, message component.Component) error
	ShowDialog(ctx context.Context, dialog dialog.Dialog) error
	ClearDialog(ctx context.Context) error
}
```

`Player()` returns the connected player metadata. `SendMessage` writes a system
message. `ShowDialog` and `ClearDialog` are available for clients whose protocol
contains the official dialog packets.

## Rich Text

limbgo uses Minekube rich text components instead of defining its own text
model:

```go
import "go.minekube.com/common/minecraft/component"
```

Any API field that takes `component.Component` supports rich text. That includes
chat/system messages and dialog title, external title, body text, button labels,
button tooltips, input labels, and option display labels.

Protocol adapters serialize rich text as JSON for older clients and anonymous
NBT for modern clients that require it.

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
