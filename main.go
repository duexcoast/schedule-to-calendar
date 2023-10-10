package main

import (
	"fmt"
	"io"
	"os"
	"path"
	"runtime/debug"
	"strconv"
	"time"

	"golang.org/x/exp/slog"
)

func main() {
	// err := godotenv.Load()
	// if err != nil {
	// 	log.Fatal(err)
	// }
	//
	// appConfig, err := newAppConfig(os.Stdout)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	//
	// common, err := newCommon(appConfig)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	//
	// app := newApp(appConfig, common)
	// logger := app.logger
	// logger.Debug("App initialized.")
	//
	// // load metered License API key prior to using the Unidoc library
	// UNIDOC_API_KEY := os.Getenv("UNIDOC_API_KEY")
	// err = license.SetMeteredKey(UNIDOC_API_KEY)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	//
	// pdfParser := newPDFParser("", "", common)
	// pdfParser.parse()
	//
	// user := user{}
	//
	// csvParser := newCSVParser(pdfParser.outPath, )
	//
	// records := readCSVFile("testdata/schedule.csv")

	// srv, err := setupGmailService()
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// FindScheduleEmail(srv)
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
	// this is the folder where PDFs and CSVs will be saved to and read from
	sharedDirectory string
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

func newCommon(cfg appConfig, sharedDir string) (*Common, error) {
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

	// Create the shared directory if it doesn't exist yet. Including the /csv
	// and /pdf subdirectories
	csvPath := path.Join(sharedDir, "csv")
	pdfPath := path.Join(sharedDir, "pdf")
	os.MkdirAll(csvPath, 0750)
	os.MkdirAll(pdfPath, 0750)

	return &Common{
		logger:          logger,
		sharedDirectory: sharedDir,
	}, nil
}

func newApp(cfg appConfig, common *Common) app {
	common.logger.Info("App initialized")
	return app{
		Common: common,
		cfg:    cfg,
	}
}
