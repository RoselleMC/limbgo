package limbgo

import (
	"context"

	"github.com/RoselleMC/limbgo/dialog"
	"go.minekube.com/common/minecraft/component"
)

// PlayerSession is the API exposed to event handlers for the connected player.
type PlayerSession interface {
	Player() Player
	Capabilities() SessionCapabilities
	SendMessage(ctx context.Context, message component.Component) error
	ShowDialog(ctx context.Context, dialog dialog.Dialog) error
	ClearDialog(ctx context.Context) error
	StoreCookie(ctx context.Context, key string, value []byte) error
	Transfer(ctx context.Context, host string, port int) error
	Disconnect(ctx context.Context, reason component.Component) error
}

// SessionCapabilities describes optional vanilla features available for the
// connected player's protocol.
type SessionCapabilities struct {
	SystemMessage bool
	Dialog        bool
	StoreCookie   bool
	Transfer      bool
	Disconnect    bool
}

// PlayerEventHandler receives optional player actions after the limbo join
// sequence has completed.
type PlayerEventHandler interface {
	HandleChat(ctx context.Context, session PlayerSession, event *ChatEvent) error
	HandleCommand(ctx context.Context, session PlayerSession, event *CommandEvent) error
	HandleDialogClick(ctx context.Context, session PlayerSession, event *DialogClickEvent) error
}

// PlayerEventHandlerFuncs adapts functions to PlayerEventHandler.
type PlayerEventHandlerFuncs struct {
	Chat        func(context.Context, PlayerSession, *ChatEvent) error
	Command     func(context.Context, PlayerSession, *CommandEvent) error
	DialogClick func(context.Context, PlayerSession, *DialogClickEvent) error
}

// HandleChat implements PlayerEventHandler.
func (h PlayerEventHandlerFuncs) HandleChat(ctx context.Context, session PlayerSession, event *ChatEvent) error {
	if h.Chat == nil {
		return nil
	}
	return h.Chat(ctx, session, event)
}

// HandleCommand implements PlayerEventHandler.
func (h PlayerEventHandlerFuncs) HandleCommand(ctx context.Context, session PlayerSession, event *CommandEvent) error {
	if h.Command == nil {
		return nil
	}
	return h.Command(ctx, session, event)
}

// HandleDialogClick implements PlayerEventHandler.
func (h PlayerEventHandlerFuncs) HandleDialogClick(ctx context.Context, session PlayerSession, event *DialogClickEvent) error {
	if h.DialogClick == nil {
		return nil
	}
	return h.DialogClick(ctx, session, event)
}

// ChatEvent is emitted when a player sends a chat message.
type ChatEvent struct {
	Player   Player
	Message  string
	Protocol int
	Canceled bool
}

// CommandEvent is emitted when a player sends a command.
type CommandEvent struct {
	Player   Player
	Command  string
	Protocol int
	Canceled bool
}

// DialogClickEvent is emitted for minecraft:custom dialog and text click
// actions. Payload is the raw anonymous NBT body sent by the client when one is
// present.
type DialogClickEvent struct {
	Player   Player
	ID       string
	Payload  []byte
	Protocol int
	Canceled bool
}
