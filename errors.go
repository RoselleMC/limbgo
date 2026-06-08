package limbgo

import "errors"

var (
	ErrMissingProtocolRouter = errors.New("limbgo: missing protocol router")
	ErrMissingWorldProvider  = errors.New("limbgo: missing world provider")
	ErrMissingSpawnResolver  = errors.New("limbgo: missing spawn resolver")
	ErrMissingWorld          = errors.New("limbgo: missing world")
	ErrWorldNotFound         = errors.New("limbgo: world not found")
	ErrInvalidSchematic      = errors.New("limbgo: invalid schematic")
)
