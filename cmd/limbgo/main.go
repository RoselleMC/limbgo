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

	world, err := loadWorld(fileCfg)
	if err != nil {
		fatal(err)
	}
	motd, err := fileCfg.Status.Component()
	if err != nil {
		fatal(err)
	}

	router := limbo.Router{
		Description:         fileCfg.Status.Description,
		MOTD:                motd,
		StatusRateLimiter:   fileCfg.Status.RateLimit.RateLimiter(),
		VersionName:         fileCfg.Status.VersionName,
		VersionProtocol:     fileCfg.Status.VersionProtocol,
		MaxPlayers:          fileCfg.Status.MaxPlayers,
		OnlinePlayers:       fileCfg.Status.OnlinePlayers,
		SamplePlayers:       fileCfg.Status.SamplePlayers,
		HidePlayers:         fileCfg.Status.HidePlayers,
		Favicon:             fileCfg.Status.Favicon,
		EnforcesSecureChat:  fileCfg.Status.EnforcesSecureChat,
		PreviewsChat:        fileCfg.Status.PreviewsChat,
		PreventsChatReports: fileCfg.Status.PreventsChatReports,
		LoginMode:           fileCfg.Auth.Mode,
		YggdrasilVerifier: limbgo.YggdrasilVerifierConfig{
			BaseURL: fileCfg.Auth.YggdrasilBaseURL,
		},
		OnlineServerID: fileCfg.Auth.OnlineServerID,
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
		Events:         limbgo.PlayerEventHandlerFuncs{},
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

func loadWorld(fileCfg limbgo.FileConfig) (limbgo.World, error) {
	if fileCfg.World.Schematic == "" {
		var dimension limbgo.Dimension
		if !fileCfg.World.Dimension.Empty() {
			dimension = fileCfg.Dimension(256)
		}
		return limbgo.DefaultWorldWithDimension(fileCfg.World.ID, dimension), nil
	}
	schematicOptions := schematic.Options{WorldID: fileCfg.World.ID}
	if !fileCfg.World.Dimension.Empty() {
		schematicOptions.Dimension = fileCfg.Dimension(0)
	}
	return schematic.LoadFile(fileCfg.World.Schematic, schematicOptions)
}
