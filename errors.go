package limbgo

import "errors"

var (
	ErrMissingProtocolRouter = errors.New("limbgo: missing protocol router")
	ErrMissingWorldProvider  = errors.New("limbgo: missing world provider")
	ErrMissingSpawnResolver  = errors.New("limbgo: missing spawn resolver")
	ErrWorldNotFound         = errors.New("limbgo: world not found")
	ErrInvalidSchematic      = errors.New("limbgo: invalid schematic")
)
