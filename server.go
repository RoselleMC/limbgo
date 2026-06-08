package limbgo

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"sync"
	"time"
)

// Config controls a limbo server instance.
type Config struct {
	Addr           string
	ProtocolRouter ProtocolRouter
	Worlds         WorldProvider
	SpawnResolver  SpawnResolver
	Events         PlayerEventHandler
	Logger         *slog.Logger
}

// Server is an embeddable limbo server.
type Server struct {
	cfg Config

	mu       sync.Mutex
	listener net.Listener
	conns    map[net.Conn]struct{}
	wg       sync.WaitGroup
	closed   bool
}

// NewServer validates config and creates a server.
func NewServer(cfg Config) (*Server, error) {
	if cfg.Addr == "" {
		cfg.Addr = ":25565"
	}
	if cfg.ProtocolRouter == nil {
		return nil, ErrMissingProtocolRouter
	}
	if cfg.Worlds == nil {
		return nil, ErrMissingWorldProvider
	}
	if cfg.SpawnResolver == nil {
		return nil, ErrMissingSpawnResolver
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return &Server{
		cfg:   cfg,
		conns: make(map[net.Conn]struct{}),
	}, nil
}

// ListenAndServe listens on Config.Addr and serves until the context is done or
// the listener fails.
func (s *Server) ListenAndServe(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return err
	}
	return s.Serve(ctx, ln)
}

// Serve accepts connections from an existing listener.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		_ = ln.Close()
		return net.ErrClosed
	}
	s.listener = ln
	s.mu.Unlock()

	go func() {
		<-ctx.Done()
		_ = s.Shutdown(context.Background())
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) || ctx.Err() != nil {
				return nil
			}
			return err
		}

		s.trackConn(conn)
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer s.untrackConn(conn)
			if err := s.cfg.ProtocolRouter.ServeConn(ctx, conn, s); err != nil && ctx.Err() == nil {
				s.cfg.Logger.Debug("connection closed", "remote", conn.RemoteAddr(), "error", err)
			}
		}()
	}
}

// Shutdown closes the listener and active connections, then waits for handlers.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	ln := s.listener
	conns := make([]net.Conn, 0, len(s.conns))
	for conn := range s.conns {
		conns = append(conns, conn)
	}
	s.mu.Unlock()

	if ln != nil {
		_ = ln.Close()
	}
	for _, conn := range conns {
		_ = conn.Close()
	}

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ResolveSpawn implements SessionServices.
func (s *Server) ResolveSpawn(ctx context.Context, player Player) (SpawnTarget, error) {
	return s.cfg.SpawnResolver.ResolveSpawn(ctx, player)
}

// World implements SessionServices.
func (s *Server) World(ctx context.Context, id string) (World, error) {
	return s.cfg.Worlds.World(ctx, id)
}

// Events implements SessionServices.
func (s *Server) Events() PlayerEventHandler {
	return s.cfg.Events
}

func (s *Server) trackConn(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		_ = conn.Close()
		return
	}
	s.conns[conn] = struct{}{}
}

func (s *Server) untrackConn(conn net.Conn) {
	_ = conn.Close()
	s.mu.Lock()
	delete(s.conns, conn)
	s.mu.Unlock()
}

// ShutdownTimeout is a convenience wrapper around Shutdown.
func (s *Server) ShutdownTimeout(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.Shutdown(ctx)
}
