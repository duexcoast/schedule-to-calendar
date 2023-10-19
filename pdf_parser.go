package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"regexp"
	"strings"

	"github.com/unidoc/unipdf/v3/common/license"
	"github.com/unidoc/unipdf/v3/extractor"
	"github.com/unidoc/unipdf/v3/model"
	"github.com/unidoc/unipdf/v3/pdfutil"
	"golang.org/x/text/unicode/norm"
)

// InitPDFLicense intializes the Unidoc PDF license
func InitPDFLicense(key string) {

	if err := license.SetMeteredKey(key); err != nil {
		log.Fatal("Could not register Unidoc License key", err)
	}
}

type pdfParser struct {
	*Common
}

func ParseSchedPDF(data io.ReadSeeker) ([]byte, error) {
	result, err := extractTable(data)
	if err != nil {
		return nil, err
	}
	bytes := result.Bytes()
	fmt.Println(string(bytes))
	return bytes, nil
}

// extractTable extracts the schedule table from the provided pdf, and returns
// the a docTable containing the contents
func extractTable(data io.ReadSeeker) (docTables, error) {
	pdfReader, err := model.NewPdfReaderLazy(data)
	if err != nil {
		return docTables{}, fmt.Errorf("NewPdfReaderLazy failed. %q err=%w", data, err)
	}

	table, err := extractPageTable(pdfReader)
	if err != nil {
		return docTables{}, fmt.Errorf("extractPageTables failed. inPath=%q err=%w",
			data, err)
	}
	result := docTables{table: table}
	return result, nil
}

// extractPageTable takes an initalized pdfReader and uses it to return a
// stringTable containing the schedule data from the pdf
func extractPageTable(pdfReader *model.PdfReader) (stringTable, error) {
	page, err := pdfReader.GetPage(1)
	if err != nil {
		return nil, err
	}
	if err := pdfutil.NormalizePage(page); err != nil {
		return nil, err
	}

	ex, err := extractor.New(page)
	if err != nil {
		return nil, err
	}
	pageText, _, _, err := ex.ExtractPageText()
	if err != nil {
		return nil, err
	}
	stringTable, err := createTable(pageText)
	if err != nil {
		return nil, err
	}
	return stringTable, nil
}

func createTable(pageText *extractor.PageText) (stringTable, error) {
	empCol, dateRow, err := extractHeader(pageText.Text())
	if err != nil {
		return nil, err
	}
	tables := pageText.Tables()
	stringTable := asStringTable(tables[0])
	stringTable = insertHeader(stringTable, empCol, dateRow)

	return stringTable, nil
}

func insertHeader(table stringTable, empCol, dateRow []string) stringTable {
	updatedTable := make([][]string, len(table)+1)
	updatedTable[0] = dateRow

	for i := 1; i < len(updatedTable); i++ {
		row := make([]string, 8)
		row[0] = empCol[i]
		for j, v := range table[i-1] {
			row[j+1] = v
		}
		updatedTable[i] = row
	}
	return updatedTable
}

// extractHeader takes the pure text extracted from the PDF and grabs the employee
// and date information that doesn't get encoded into the table
func extractHeader(text string) (employeeCol []string, dateRow []string, err error) {
	pageReader := strings.NewReader(text)
	scanner := bufio.NewScanner(pageReader)

	// add all the employee information that isn't included in the table
	// to a slice
	for scanner.Scan() {
		token := scanner.Text()
		if token == "" {
			break
		}
		employeeCol = append(employeeCol, token)
	}
	dateRow = append(dateRow, "")
	// add all the date information that isn't included in the table to a
	// slice
	for scanner.Scan() {
		token := scanner.Text()
		if token == "" {
			break
		}
		// we want to get rid of the weekday in lines of the form:
		// "Tuesday 10/17/2023"
		// we only want to retain the date information
		separatorIndex := strings.Index(token, " ")
		if separatorIndex > 0 {
			token = token[separatorIndex+1:]
		}
		// only one of these checks SHOULD be needed, but better robust than sorry
		if strings.Contains(token, "/") && !strings.Contains(token, "day") {
			dateRow = append(dateRow, token)
		}
	}
	if len(dateRow) != 8 {
		err = fmt.Errorf("Did not extract correct date row information from pdf. Expected 8 dates, got: %d", len(dateRow))
		return employeeCol, dateRow, err
	}
	return employeeCol, dateRow, nil
}

// docTables describes the tables in a document.
type docTables struct {
	table stringTable
}

// stringTable is the strings in TextTable.
type stringTable [][]string

func (r docTables) Bytes() []byte {
	return r.table.csv()
}

// wh returns the width and height of table `t`.
func (t stringTable) wh() (int, int) {
	if len(t) == 0 {
		return 0, 0
	}
	return len(t[0]), len(t)
}

// csv returns `t` in CSV format.
func (t stringTable) csv() []byte {
	w, h := t.wh()
	b := new(bytes.Buffer)
	csvwriter := csv.NewWriter(b)
	for y, row := range t {
		if len(row) != w {
			err := fmt.Errorf("table = %d x %d row[%d]=%d %q", w, h, y, len(row), row)
			panic(err)
		}
		csvwriter.Write(row)
	}
	csvwriter.Flush()
	return b.Bytes()
}

// asStringTable returns TextTable `table` as a stringTable.
func asStringTable(table extractor.TextTable) stringTable {
	cells := make(stringTable, table.H)
	for y, row := range table.Cells {
		cells[y] = make([]string, table.W)
		for x, cell := range row {
			cells[y][x] = cell.Text
		}
	}
	return normalizeTable(cells)
}

// normalizeTable returns `cells` with each cell normalized.
func normalizeTable(cells stringTable) stringTable {
	for y, row := range cells {
		for x, cell := range row {
			cells[y][x] = normalize(cell)
		}
	}
	return cells
}

// normalize returns a version of `text` that is NFKC normalized and has reduceSpaces() applied.
func normalize(text string) string {
	return reduceSpaces(norm.NFKC.String(text))
}

// reduceSpaces returns `text` with runs of spaces of any kind (spaces, tabs, line breaks, etc)
// reduced to a single space.
func reduceSpaces(text string) string {
	text = reSpace.ReplaceAllString(text, " ")
	return strings.Trim(text, " \t\n\r\v")
}

var reSpace = regexp.MustCompile(`(?m)\s+`)
