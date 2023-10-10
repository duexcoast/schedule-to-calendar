package main

import (
	"log"
	"os"
	"path"
	"testing"

	"github.com/joho/godotenv"
	"github.com/unidoc/unipdf/v3/common/license"
)

func Test_ParsePDF(t *testing.T) {
	sharedDir := path.Join("testdata", "pdf_parser")
	app := setupAppForTest(t, sharedDir)

	// // load metered License API key prior to using the Unidoc library
	UNIDOC_API_KEY := os.Getenv("UNIDOC_API_KEY")
	err := license.SetMeteredKey(UNIDOC_API_KEY)
	if err != nil {
		log.Fatal(err)
	}

	t.Run("table data 1", func(t *testing.T) {
		pdfParser := newPDFParser("ServerSchedule10.9-10.15", app.Common)

		// if the converted CSV file already exists from previous tests, remove it
		_ = os.Remove(pdfParser.outPath) // ignore error - we don't care if the file doesn't exist

		pdfParser.ParsePDF()

		_, err := os.ReadFile(pdfParser.outPath)
		if err != nil {
			t.Fatalf("Could not read converted csv file: %q err: %v", pdfParser.outPath, err)
		}
		t.Logf("CSV file successfully created: %q", pdfParser.outPath)
	})
}

// Returns an initialized app, reusuable across all Tests. sharedDir parameter
// determines where files will be shared across parsers
func setupAppForTest(t *testing.T, sharedDir string) app {
	t.Helper()
	err := godotenv.Load()
	if err != nil {
		t.Fatal(err)
	}

	appConfig, err := newAppConfig(os.Stdout)
	if err != nil {
		t.Fatal(err)
	}

	common, err := newCommon(appConfig, sharedDir)
	if err != nil {
		t.Fatal(err)
	}

	app := newApp(appConfig, common)
	return app
}
