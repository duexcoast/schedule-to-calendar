package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"golang.org/x/exp/slog"
)

type app struct {
	*Common
	cfg appConfig
}

type appConfig struct {
	debug     bool
	output    io.Writer
	goVersion string
	common    *Common
	buildDate time.Time
}

type Common struct {
	logger *slog.Logger
	user   user
}

func Initialize(output io.Writer) app {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Could not load environment variables. err: %w", err)
	}

	appConfig, err := newAppConfig(output)
	if err != nil {
		log.Fatal("Could not initialize app config. err: %w", err)
	}

	user := newUser("Conor Ney", "conor.ux@gmail.com")

	common, err := newCommon(appConfig, user)
	if err != nil {
		log.Fatalf("Could not initialize Common struct. err: %w", err)
	}

	// // load metered License API key prior to using the Unidoc library
	UNIDOC_API_KEY := os.Getenv("UNIDOC_API_KEY")
	InitPDFLicense(UNIDOC_API_KEY)

	app := newApp(appConfig, common)
	return app
}

func newAppConfig(output io.Writer) (appConfig, error) {
	buildInfo, _ := debug.ReadBuildInfo()
	debugEnv, err := strconv.ParseBool(os.Getenv("DEBUG"))
	if err != nil {
		debugEnv = false
		fmt.Printf("Could not parse DEBUG env variable. err: %v", err)
	}

	return appConfig{
		debug:     debugEnv,
		output:    output,
		goVersion: buildInfo.GoVersion,
		buildDate: time.Now(),
	}, nil
}

func newCommon(cfg appConfig, user user) (*Common, error) {
	// Structured logging setup
	logLevel := slog.LevelInfo
	if cfg.debug {
		logLevel = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	logger := slog.New(slog.NewTextHandler(cfg.output, opts))
	slog.SetDefault(logger)

	return &Common{
		user:   user,
		logger: logger,
	}, nil
}

func newApp(cfg appConfig, common *Common) app {
	common.logger.Info("App initialized")
	return app{
		Common: common,
		cfg:    cfg,
	}
}
