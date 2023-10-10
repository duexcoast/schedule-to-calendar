package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/unidoc/unipdf/v3/extractor"
	"github.com/unidoc/unipdf/v3/model"
	"github.com/unidoc/unipdf/v3/pdfutil"
	"golang.org/x/text/unicode/norm"
)

type pdfParser struct {
	*Common

	// filename does not include extension
	filename string
	// path to input PDF of weekly schedule
	inPath string
	// path to output CSV of weekly schedule
	outPath string
}

func newPDFParser(filename string, common *Common) *pdfParser {
	// take the filename and add the correct extensions for the in and out paths
	fullIn := strings.Join([]string{filename, ".pdf"}, "")
	fullOut := strings.Join([]string{filename, ".csv"}, "")
	in := path.Join(common.sharedDirectory, "pdf", fullIn)
	out := path.Join(common.sharedDirectory, "csv", fullOut)
	parser := &pdfParser{
		filename: filename,
		inPath:   in,
		outPath:  out,
		Common:   common,
	}
	return parser
}

// Method PDFtoCSV takes the PDF file stored at p.inPath and stores it as a CSV
// file in p.outPath
func (p *pdfParser) PDFtoCSV() error {
	// working under the assumption that the schedule fits on one page. might
	// need to make this more robust
	result, err := extractTables(p.inPath, 1, 1)
	if err != nil {
		return fmt.Errorf("Could not parse PDF: %q into CSV. Err: %v", p.inPath, err)
	}
	// potentially may need to call result.filter() here to set a minimum cell
	// height and width. if there is extraneous table information, this is probably
	// the solution
	// result.filter(width, height)
	result.saveCSVFiles(p.outPath)

	return nil
}

// extractTables extracts tables from pages `firstPage` to `lastPage` in PDF file `inPath`.
func extractTables(inPath string, firstPage, lastPage int) (docTables, error) {
	f, err := os.Open(inPath)
	if err != nil {
		return docTables{}, fmt.Errorf("Could not open %q err=%w", inPath, err)
	}
	defer f.Close()

	pdfReader, err := model.NewPdfReaderLazy(f)
	if err != nil {
		return docTables{}, fmt.Errorf("NewPdfReaderLazy failed. %q err=%w", inPath, err)
	}
	numPages, err := pdfReader.GetNumPages()
	if err != nil {
		return docTables{}, fmt.Errorf("GetNumPages failed. %q err=%w", inPath, err)
	}

	if firstPage < 1 {
		firstPage = 1
	}
	if lastPage > numPages {
		lastPage = numPages
	}

	result := docTables{pageTables: make(map[int][]stringTable)}
	for pageNum := firstPage; pageNum <= lastPage; pageNum++ {
		tables, err := extractPageTables(pdfReader, pageNum)
		if err != nil {
			return docTables{}, fmt.Errorf("extractPageTables failed. inPath=%q pageNum=%d err=%w",
				inPath, pageNum, err)
		}
		result.pageTables[pageNum] = tables
	}
	return result, nil
}

// extractPageTables extracts the tables from (1-offset) page number `pageNum` in opened
// PdfReader `pdfReader.
func extractPageTables(pdfReader *model.PdfReader, pageNum int) ([]stringTable, error) {
	page, err := pdfReader.GetPage(pageNum)
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
	tables := pageText.Tables()
	stringTables := make([]stringTable, len(tables))
	for i, table := range tables {
		stringTables[i] = asStringTable(table)
	}
	return stringTables, nil
}

// docTables describes the tables in a document.
type docTables struct {
	pageTables map[int][]stringTable
}

// stringTable is the strings in TextTable.
type stringTable [][]string

// altered this function to be hardcoded for a single page. May present problems
// in the future, may not
func (r docTables) saveCSVFiles(csvRoot string) error {
	// for _, pageNum := range r.pageNumbers() {
	// 	for i, table := range r.pageTables[pageNum] {
	// 		csvPath := fmt.Sprintf("%s.page%d.table%d.csv", csvRoot, pageNum, i+1)
	// 		contents := table.csv()
	// 		if err := os.WriteFile(csvPath, []byte(contents), 0666); err != nil {
	// 			return fmt.Errorf("failed to write csvPath=%q err=%w", csvPath, err)
	// 		}
	// 	}
	// }
	for _, table := range r.pageTables[1] {
		contents := table.csv()
		if err := os.WriteFile(csvRoot, []byte(contents), 0666); err != nil {
			return fmt.Errorf("failed to write csvPath=%q err=%w", csvRoot, err)
		}
	}
	return nil
}

// wh returns the width and height of table `t`.
func (t stringTable) wh() (int, int) {
	if len(t) == 0 {
		return 0, 0
	}
	return len(t[0]), len(t)
}

// csv returns `t` in CSV format.
func (t stringTable) csv() string {
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
	return b.String()
}

func (r *docTables) String() string {
	return r.describe(1)
}

// describe returns a string describing the tables in `r`.
//
//	                            (level 0)
//	%d pages %d tables          (level 1)
//	  page %d: %d tables        (level 2)
//	    table %d: %d x %d       (level 3)
//	        contents            (level 4)
func (r *docTables) describe(level int) string {
	if level == 0 || r.numTables() == 0 {
		return "\n"
	}
	var sb strings.Builder
	pageNumbers := r.pageNumbers()
	fmt.Fprintf(&sb, "%d pages %d tables\n", len(pageNumbers), r.numTables())
	if level <= 1 {
		return sb.String()
	}
	for _, pageNum := range r.pageNumbers() {
		tables := r.pageTables[pageNum]
		if len(tables) == 0 {
			continue
		}
		fmt.Fprintf(&sb, "   page %d: %d tables\n", pageNum, len(tables))
		if level <= 2 {
			continue
		}
		for i, table := range tables {
			w, h := table.wh()
			fmt.Fprintf(&sb, "      table %d: %d x %d\n", i+1, w, h)
			if level <= 3 || len(table) == 0 {
				continue
			}
			for _, row := range table {
				cells := make([]string, len(row))
				for i, cell := range row {
					if len(cell) > 0 {
						cells[i] = fmt.Sprintf("%q", cell)
					}
				}
				fmt.Fprintf(&sb, "        [%s]\n", strings.Join(cells, ", "))
			}
		}
	}
	return sb.String()
}

func (r *docTables) pageNumbers() []int {
	pageNums := make([]int, len(r.pageTables))
	i := 0
	for pageNum := range r.pageTables {
		pageNums[i] = pageNum
		i++
	}
	sort.Ints(pageNums)
	return pageNums
}

func (r *docTables) numTables() int {
	n := 0
	for _, tables := range r.pageTables {
		n += len(tables)
	}
	return n
}

// filter returns the tables in `r` that are at least `width` cells wide and `height` cells high.
func (r docTables) filter(width, height int) docTables {
	filtered := docTables{pageTables: make(map[int][]stringTable)}
	for pageNum, tables := range r.pageTables {
		var filteredTables []stringTable
		for _, table := range tables {
			if len(table[0]) >= width && len(table) >= height {
				filteredTables = append(filteredTables, table)
			}
		}
		if len(filteredTables) > 0 {
			filtered.pageTables[pageNum] = filteredTables
		}
	}
	return filtered
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
