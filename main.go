package main

import (
	"io"
	"log"
	"os"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/unidoc/unipdf/v3/common/license"
	"golang.org/x/exp/slog"
)

func main() {
	appConfig, err := newAppConfig(os.Stdout)
	if err != nil {
		log.Fatal(err)
	}
	common, err := newCommon(appConfig)
	if err != nil {
		log.Fatal(err)
	}
	app := newApp(appConfig, common)
	logger := app.logger
	logger.Debug("App initialized.")

	// load metered License API key prior to using the Unidoc library
	err := license.SetMeteredKey(os.Getenv("UNIDOC_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}
}

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
}

func newAppConfig(output io.Writer) (appConfig, error) {
	buildInfo, _ := debug.ReadBuildInfo()
	debugEnv, err := strconv.ParseBool(os.Getenv("DEBUG"))
	if err != nil {
		debugEnv = false
	}

	return appConfig{
		debug:     debugEnv,
		output:    output,
		goVersion: buildInfo.GoVersion,
		buildDate: time.Now(),
	}, nil
}

func newCommon(cfg appConfig) (*Common, error) {
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
		logger: logger,
	}, nil
}

func newApp(cfg appConfig, common *Common) app {

	return app{
		Common: common,
		cfg:    cfg,
	}
}
