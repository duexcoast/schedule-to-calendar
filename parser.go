package main

import (
	"bytes"
	"fmt"
	"os"

	"github.com/unidoc/unipdf/v3/common"
	"github.com/unidoc/unipdf/v3/extractor"
	"github.com/unidoc/unipdf/v3/model"
)

var saveParams saveMarkedUpParams

type parser struct {
	*Common

	// path to input PDF of weekly schedule
	inPath string
	// path to output CSV of weekly schedule
	outPath string
}

func newParser(inPath, outPath string, common *Common) parser {
	parser := parser{
		Common:  common,
		inPath:  inPath,
		outPath: outPath,
	}
	if inPath == "" {
		parser.inPath = "testdata/Server-Schedule-10.9-10.15.pdf"
	}
	if outPath == "" {
		parser.outPath = "testdata/schedule.csv"
	}
	return parser
}

func parse(common *Common) {
	saveParams = saveMarkedUpParams{
		markupType:       "all",
		markupOutputPath: "/tmp/markup.pdf",
	}

	// TODO: Should maybe set up unidoc logger here or in main, connect it
	// to log file
	// TODO: determine structure for setting up config for pdf parser
	parser := newParser("", "", common)

	extractTableData(parser)
}

func extractTableData(parser parser) error {
	f, err := os.Open(parser.inPath)
	if err != nil {
		return fmt.Errorf("Could not open %d err=%v", parser.inPath, err)
	}
	defer f.Close()

	pdfReader, err := model.NewPdfReaderLazy(f)
	if err != nil {
		return fmt.Errorf("NewPdfReaderLazy failed. %q err=%v", parser.inPath, err)
	}
	numPages, err := pdfReader.GetNumPages()
	if err != nil {
		return fmt.Errorf("GetNumPages failed. %q err=%v", parser.inPath, err)
	}

	saveParams.pdfReader = pdfReader
	saveParams.markups = map[int][][]model.PdfRectangle{}

	var csvData bytes.Buffer
	for pageNum := 1; pageNum <= numPages; pageNum++ {
		if pageNum > 3 {
			break
		}
		saveParams.curPage = pageNum
		page, err := pdfReader.GetPage(pageNum)
		if err != nil {
			return fmt.Errorf("GetNumPages failed. %q pageNum=%d err=%v", parser.inPath, pageNum, err)
		}

		mbox, err := page.GetMediaBox()
		if err != nil {
			return err
		}
		ex, err := extractor.New(page)
		if err != nil {
			return fmt.Errorf("extractor.New failed. %q pageNum=%d err=%v", parser.inPath, pageNum, err)
		}
		pageText, _, _, err := ex.ExtractPageText()
		if err != nil {
			return fmt.Errorf("ExtractPageText failed. %q pageNum=%d err=%v", parser.inPath, pageNum, err)
		}
		text := pageText.Text()
		textMarks := pageText.Marks()
		// common.Log.Debug("pageNum=%d text=%d textMarks=%d", pageNum, len(text), textMarks.Len())

		group := []model.PdfRectangle{}
		for _, mark := range textMarks.Elements() {
			group = append(group, mark.BBox)
		}
		saveParams.markups[pageNum] = append(saveParams.markups[pageNum], group)

		pageCSV, err := pageMarksToCSV(textMarks)

		if err != nil {
			common.Log.Debug("Error grouping text: %v", err)
			return err
		}
	}

	return nil
}

type saveMarkedUpParams struct {
	pdfReader        *model.PdfReader
	markups          map[int][][]model.PdfRectangle
	curPage          int
	markupType       string
	markupOutputPath string
}
