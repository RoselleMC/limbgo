package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/RoselleMC/limbgo"
	"github.com/RoselleMC/limbgo/protocol/limbo"
	"github.com/RoselleMC/limbgo/protocol/registrydata"
	"github.com/RoselleMC/limbgo/world/schematic"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "limbgo.json", "path to a limbgo JSON config file")
	flag.Parse()

	fileCfg, err := limbgo.LoadFileConfig(configPath)
	if err != nil {
		fatal(err)
	}

	world, err := schematic.LoadFile(fileCfg.World.Schematic, schematic.Options{WorldID: fileCfg.World.ID})
	if err != nil {
		fatal(err)
	}

	router := limbo.Router{
		Description: fileCfg.Status.Description,
		MaxPlayers:  fileCfg.Status.MaxPlayers,
	}
	if fileCfg.Protocol.ModernProtocols != "" {
		router.ModernProtocols, err = limbo.LoadModernProtocolsFile(fileCfg.Protocol.ModernProtocols)
		if err != nil {
			fatal(err)
		}
	}
	if fileCfg.Protocol.RegistryData != "" {
		router.RegistryData, err = registrydata.LoadFile(fileCfg.Protocol.RegistryData)
		if err != nil {
			fatal(err)
		}
	}

	server, err := limbgo.NewServer(limbgo.Config{
		Addr:           fileCfg.Listen,
		ProtocolRouter: router,
		Worlds:         limbgo.StaticWorldProvider{world.ID(): world},
		SpawnResolver:  limbgo.StaticSpawn(fileCfg.SpawnTarget()),
		Logger:         slog.Default(),
	})
	if err != nil {
		fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe(ctx)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			fatal(err)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			fatal(err)
		}
	}
}

func fatal(err error) {
	_, _ = fmt.Fprintf(os.Stderr, "limbgo: %v\n", err)
	os.Exit(1)
}
