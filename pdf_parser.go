package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/unidoc/unipdf/v3/contentstream"
	"github.com/unidoc/unipdf/v3/core"
	"github.com/unidoc/unipdf/v3/creator"
	"github.com/unidoc/unipdf/v3/extractor"
	"github.com/unidoc/unipdf/v3/model"
)

var saveParams saveMarkedupParams

type pdfParser struct {
	*Common

	// filename does not include extension
	filename string
	// path to input PDF of weekly schedule
	inPath string
	// path to output CSV of weekly schedule
	outPath string
}

func newPDFParser(filename string, common *Common) pdfParser {
	// take the filename and add the correct extensions for the in and out paths
	fullIn := strings.Join([]string{filename, ".pdf"}, "")
	fullOut := strings.Join([]string{filename, ".csv"}, "")
	in := path.Join(common.sharedDirectory, "pdf", fullIn)
	out := path.Join(common.sharedDirectory, "csv", fullOut)
	parser := pdfParser{
		filename: filename,
		inPath:   in,
		outPath:  out,
		Common:   common,
	}
	return parser
}

// ParsePDF extracts the table data from the PDF stored at p.inPath, and saves it
// as a CSV file at p.outPath
func (p pdfParser) ParsePDF() {
	saveParams = saveMarkedupParams{
		markupType:       "all",
		markupOutputPath: "/tmp/markup.pdf",
	}
	p.extractTableData()
}

func (p pdfParser) extractTableData() error {
	f, err := os.Open(p.inPath)

	if err != nil {
		return fmt.Errorf("Could not open %s err=%v", p.inPath, err)
	}
	defer f.Close()

	pdfReader, err := model.NewPdfReaderLazy(f)
	if err != nil {
		return fmt.Errorf("NewPdfReaderLazy failed. %q err=%v", p.inPath, err)
	}
	numPages, err := pdfReader.GetNumPages()
	if err != nil {
		return fmt.Errorf("GetNumPages failed. %q err=%v", p.inPath, err)
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
			return fmt.Errorf("GetNumPages failed. %q pageNum=%d err=%v", p.inPath, pageNum, err)
		}

		mbox, err := page.GetMediaBox()
		if err != nil {
			return err
		}
		if page.Rotate != nil && *page.Rotate == 90 {
			// TODO: This is a "hack" to change the perspective of the extractor to account for the rotation.
			contents, err := page.GetContentStreams()
			if err != nil {
				return err
			}

			cc := contentstream.NewContentCreator()
			cc.Translate(mbox.Width()/2, mbox.Height()/2)
			cc.RotateDeg(-90)
			cc.Translate(-mbox.Width()/2, -mbox.Height()/2)
			rotateOps := cc.Operations().String()
			contents = append([]string{rotateOps}, contents...)

			page.Duplicate()
			err = page.SetContentStreams(contents, core.NewRawEncoder())
			if err != nil {
				return err

			}
			page.Rotate = nil
		}
		ex, err := extractor.New(page)
		if err != nil {
			return fmt.Errorf("extractor.New failed. %q pageNum=%d err=%v", p.inPath, pageNum, err)
		}
		pageText, _, _, err := ex.ExtractPageText()
		if err != nil {
			return fmt.Errorf("ExtractPageText failed. %q pageNum=%d err=%v", p.inPath, pageNum, err)
		}
		// text := pageText.Text()
		textMarks := pageText.Marks()
		// p.logger.Debug("extracted text marks", "pageNum", pageNum, "text", len(text), "textMarks", textMarks.Len())

		group := []model.PdfRectangle{}
		for _, mark := range textMarks.Elements() {
			group = append(group, mark.BBox)
		}
		saveParams.markups[pageNum] = append(saveParams.markups[pageNum], group)

		pageCSV, err := p.pageMarksToCSV(textMarks)

		if err != nil {
			// p.logger.Debug("Error grouping text: %v", err)
			return err
		}
		csvData.WriteString(pageCSV)

	}
	if saveParams.markupType != "none" {
		err = p.saveMarkedupPDF(saveParams)
		if err != nil {
			return fmt.Errorf("Failed to save marked up pdf: %v", err)
		}
	}

	return os.WriteFile(p.outPath, csvData.Bytes(), 0666)
}

func pageMarksToCSV(textMarks *extractor.TextMarkArray) {
	panic("unimplemented")
}

type saveMarkedupParams struct {
	pdfReader        *model.PdfReader
	markups          map[int][][]model.PdfRectangle
	curPage          int
	markupType       string
	markupOutputPath string
}

func (p pdfParser) rectUnion(b1, b2 model.PdfRectangle) model.PdfRectangle {
	return model.PdfRectangle{
		Llx: math.Min(b1.Llx, b2.Llx),
		Lly: math.Min(b1.Lly, b2.Lly),
		Urx: math.Max(b1.Urx, b2.Urx),
		Ury: math.Max(b1.Ury, b2.Ury),
	}
}

func (p pdfParser) bboxArea(bbox model.PdfRectangle) float64 {
	return math.Abs(bbox.Urx-bbox.Llx) * math.Abs(bbox.Ury-bbox.Lly)
}

// Measure of the difference between areas of `bbox1` and `bbox2` individually
// and that of the union of the two.
func (p pdfParser) overlaps(bbox1, bbox2 model.PdfRectangle) float64 {
	union := p.rectUnion(bbox1, bbox2)
	a := p.bboxArea(union)
	b := p.bboxArea(bbox1) + p.bboxArea(bbox2)
	diff := (a - b) / (a + b)
	return diff
}

// Measure of the vertical overlap of `bbox1` and `bbox2`, when the difference is 0
// then they are exactly on top of each other, and there is overlap when < 0.
func (p pdfParser) lineOverlap(bbox1, bbox2 model.PdfRectangle) float64 {
	union := p.rectUnion(bbox1, bbox2)
	a := math.Abs(union.Ury - union.Lly)
	b := math.Abs(bbox1.Ury-bbox1.Lly) + math.Abs(bbox2.Ury-bbox2.Lly)
	diff := (a - b) / (a + b)
	return diff
}

// Measure of the horizontal overlap of `bbox1` and `bbox2`, when the difference is 0
// then they are exactly on next to each other, and there is overlap when < 0.
func (p pdfParser) columnOverlap(bbox1, bbox2 model.PdfRectangle) float64 {
	union := p.rectUnion(bbox1, bbox2)
	a := math.Abs(union.Urx - union.Llx)
	b := math.Abs(bbox1.Urx-bbox1.Llx) + math.Abs(bbox2.Urx-bbox2.Llx)
	diff := (a - b) / (a + b)
	return diff
}

// Identify lines. - segment words into lines
func (p pdfParser) identifyLines(words []segmentationWord) [][]segmentationWord {
	lines := [][]segmentationWord{}

	for _, word := range words {
		wbbox, ok := word.BBox()
		if !ok {
			continue
		}

		match := false
		for i, line := range lines {
			firstWord := line[0]
			firstBBox, ok := firstWord.BBox()
			if !ok {
				continue
			}

			overlap := p.lineOverlap(wbbox, firstBBox)
			// p.logger.Debug("[identifyLines]", "line1", word.String, "line2", firstWord.String(), "overlap", overlap, "wbbox", wbbox, "firstBBox", firstBBox)
			if overlap < 0 {
				lines[i] = append(lines[i], word)
				match = true
				break
			}
		}
		if !match {
			lines = append(lines, []segmentationWord{word})
		}
	}
	sort.SliceStable(lines, func(i, j int) bool {
		bboxi, _ := lines[i][0].BBox()
		bboxj, _ := lines[j][0].BBox()
		return bboxi.Lly >= bboxj.Lly
	})
	for li := range lines {
		sort.SliceStable(lines[li], func(i, j int) bool {
			bboxi, _ := lines[li][i].BBox()
			bboxj, _ := lines[li][j].BBox()
			return bboxi.Llx < bboxj.Llx
		})
	}

	// Save the line bounding boxes for markup output.
	lineGroups := []model.PdfRectangle{}
	for _, line := range lines {
		var lineRect model.PdfRectangle
		// p.logger.Debug("[identifyLines]", "line", li+1)
		for i, word := range line {
			wbbox, ok := word.BBox()
			if !ok {
				continue
			}
			// p.logger.Debug("[identifyLines]", "saved line", word.String())

			if i == 0 {
				lineRect = wbbox
			} else {
				lineRect = p.rectUnion(lineRect, wbbox)
			}
		}
		lineGroups = append(lineGroups, lineRect)
	}
	saveParams.markups[saveParams.curPage] = append(saveParams.markups[saveParams.curPage], lineGroups)
	return lines
}

// Identify columns.
func (p pdfParser) identifyColumns(words []segmentationWord) []model.PdfRectangle {
	columns := [][]segmentationWord{}
	for _, word := range words {
		wbbox, ok := word.BBox()
		if !ok {
			continue
		}

		match := false
		bestOverlap := 1.0
		bestColumn := 0
		for i, column := range columns {
			firstWord := column[0]
			firstBBox, ok := firstWord.BBox()
			if !ok {
				continue
			}

			overlap := p.columnOverlap(wbbox, firstBBox)
			// p.logger.Debug("[identifyColumns]", "column", word.String(), "column1", firstWord.String(), "overlap", overlap, "wbbox", wbbox, "firstBBox", firstBBox)
			if overlap < 0.0 {
				if overlap < bestOverlap {
					bestOverlap = overlap
					bestColumn = i
				}
				match = true
			}
		}
		if match {
			columns[bestColumn] = append(columns[bestColumn], word)
		} else {
			columns = append(columns, []segmentationWord{word})
		}
	}
	sort.SliceStable(columns, func(i, j int) bool {
		bboxi, _ := columns[i][0].BBox()
		bboxj, _ := columns[j][0].BBox()
		return bboxi.Llx < bboxj.Llx
	})
	for li := range columns {
		sort.SliceStable(columns[li], func(i, j int) bool {
			bboxi, _ := columns[li][i].BBox()
			bboxj, _ := columns[li][j].BBox()
			return bboxi.Lly >= bboxj.Lly
		})
	}

	colGroups := []model.PdfRectangle{}
	for _, column := range columns {
		var colRect model.PdfRectangle
		// p.logger.Debug("[identifyColumns]", "column", li+1)
		for i, word := range column {
			wbbox, ok := word.BBox()
			if !ok {
				continue
			}

			if i == 0 {
				colRect = wbbox
			} else {
				colRect = p.rectUnion(colRect, wbbox)
			}
		}
		// p.logger.Debug("[identifyColumns]", "column", li+1, "Bbox", colRect)
		colGroups = append(colGroups, colRect)
	}

	// Filter by combining overlapping columns.
	filtered := []model.PdfRectangle{}
	for i := 0; i < len(colGroups); {
		colgroup := colGroups[i]
		j := i + 1
		for ; j < len(colGroups); j++ {
			overlap := p.columnOverlap(colgroup, colGroups[j])
			// p.logger.Debug("[identifyColumns]", "columnOverlapA", i+1, "columnOverlapB", j+1, "overlap", overlap, "colgroup", colgroup, "colgroup[j]", colGroups[j])
			if overlap > 0.0 {
				break
			}
			colgroup = p.rectUnion(colgroup, colGroups[j])
		}
		i = j
		filtered = append(filtered, colgroup)
	}

	saveParams.markups[saveParams.curPage] = append(saveParams.markups[saveParams.curPage], filtered)
	return filtered
}

// getLineTableTextData converts the lines of words into table strings cells by accounting for
// distribution of lines into columns as specified by `columnBBoxes`.
func (p pdfParser) getLineTableTextData(lines [][]segmentationWord, columnBBoxes []model.PdfRectangle) [][]string {
	tabledata := [][]string{}
	for _, line := range lines {
		linedata := make([]string, len(columnBBoxes))
		for _, word := range line {
			wordBBox, ok := word.BBox()
			if !ok {
				continue
			}

			bestColumn := 0
			bestOverlap := 1.0
			for icol, colBBox := range columnBBoxes {
				overlap := p.columnOverlap(wordBBox, colBBox)
				if overlap < bestOverlap {
					bestOverlap = overlap
					bestColumn = icol
				}
			}
			linedata[bestColumn] += word.String()
		}
		tabledata = append(tabledata, linedata)
	}
	return tabledata
}

// segmentationWord represents a word that has been segmented in PDF text.
type segmentationWord struct {
	ma *extractor.TextMarkArray
}

func (w segmentationWord) Elements() []extractor.TextMark {
	return w.ma.Elements()
}

func (w segmentationWord) BBox() (model.PdfRectangle, bool) {
	return w.ma.BBox()
}

func (w segmentationWord) String() string {
	if w.ma == nil {
		return ""
	}

	var buf bytes.Buffer
	for _, m := range w.Elements() {
		buf.WriteString(m.Text)
	}
	return buf.String()
}

// pageMarksToCSV converts textMarks from a single page into CSV by grouping the marks into
// words, lines and columns and then writing the table cells data as CSV output.
func (p pdfParser) pageMarksToCSV(textMarks *extractor.TextMarkArray) (string, error) {
	// STEP - Form words.
	// Group the closest text marks that are overlapping.
	words := []segmentationWord{}
	word := segmentationWord{ma: &extractor.TextMarkArray{}}
	var lastMark extractor.TextMark
	isFirst := true
	for _, mark := range textMarks.Elements() {
		if mark.Text == "" {
			continue
		}

		// p.logger.Debug("[pageMarksToCSV]", "Mark", i, "string", mark.Text)
		if isFirst {
			word = segmentationWord{ma: &extractor.TextMarkArray{}}
			word.ma.Append(mark)
			lastMark = mark
			isFirst = false
			continue
		}
		// p.logger.Debug("[pageMarksToCSV]", "overlaps", p.overlaps(mark.BBox, lastMark.BBox))
		overlap := p.overlaps(mark.BBox, lastMark.BBox)
		if overlap > 0.1 {
			if len(strings.TrimSpace(word.String())) > 0 {
				// p.logger.Debug("Appending word: '%s' (%d chars) (%d elements)", word.String(), len(word.String()), len(word.Elements()))
				words = append(words, word)
			}
			word = segmentationWord{ma: &extractor.TextMarkArray{}}
		}
		word.ma.Append(mark)
		lastMark = mark
	}
	if len(strings.TrimSpace(word.String())) > 0 {
		// p.logger.Debug("[pageMarksToCSV]", "appendingWork", word.String(), "len", len(word.String()), "elements", len(word.Elements()))
		words = append(words, word)
	}

	// Include the words in the markup.
	{
		wbboxes := []model.PdfRectangle{}
		for _, word := range words {
			wbbox, ok := word.BBox()
			if !ok {
				continue
			}
			wbboxes = append(wbboxes, wbbox)
		}
		saveParams.markups[saveParams.curPage] = append(saveParams.markups[saveParams.curPage], wbboxes)
	}

	lines := p.identifyLines(words)

	// Filter out words in lines with only 1 column.
	tableLines := [][]segmentationWord{}
	for _, line := range lines {
		if len(line) <= 1 {
			continue
		}
		tableLines = append(tableLines, line)
	}

	tableWords := []segmentationWord{}
	for _, line := range tableLines {
		for _, word := range line {
			tableWords = append(tableWords, word)
		}
	}

	columnBBoxes := p.identifyColumns(tableWords)

	tabledata := p.getLineTableTextData(lines, columnBBoxes)

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	err := w.WriteAll(tabledata)
	if err != nil {
		return "", err
	}
	w.Flush()
	return buf.String(), nil
}

// Saves a marked up PDF with the original with certain groups highlighted: marks, words, lines, columns.
func (p pdfParser) saveMarkedupPDF(params saveMarkedupParams) error {
	var pageNums []int
	for pageNum, _ := range params.markups {
		pageNums = append(pageNums, pageNum)
	}
	sort.Ints(pageNums)
	if len(pageNums) == 0 {
		return nil
	}

	// Make a new PDF creator.
	c := creator.New()
	for _, pageNum := range pageNums {
		// p.logger.Debug("Page %d - %d marks", pageNum, len(params.markups[pageNum]))
		page, err := params.pdfReader.GetPage(pageNum)
		if err != nil {
			return fmt.Errorf("saveOutputPdf: Could not get page  pageNum=%d. err=%v", pageNum, err)
		}
		mediaBox, err := page.GetMediaBox()
		if err != nil {
			return fmt.Errorf("saveOutputPdf: Could not get MediaBox  pageNum=%d. err=%v", pageNum, err)
		}
		if page.MediaBox == nil {
			// Deal with MediaBox inherited from Parent.
			p.logger.Info("MediaBox: %v -> %v", page.MediaBox, mediaBox)
			page.MediaBox = mediaBox
		}
		h := mediaBox.Ury

		if err := c.AddPage(page); err != nil {
			return fmt.Errorf("AddPage failed err=%v ", err)
		}

		colors := map[int]string{
			0: "hide", // marks
			1: "hide", // words
			2: "hide", // lines
			3: "hide", // columns
		}

		switch saveParams.markupType {
		case "marks":
			colors[0] = "#0000ff"
		case "words":
			colors[1] = "#00ff00"
		case "lines":
			colors[2] = "#ff0000"
		case "columns":
			colors[3] = "#f0f000"
		case "all":
			colors[0] = "#0000ff"
			colors[1] = "#00ff00"
			colors[2] = "#ff0000"
			colors[3] = "#f0f000"
		}

		for gi, group := range params.markups[pageNum] {
			if colors[gi] == "hide" {
				continue
			}
			borderColor := creator.ColorRGBFromHex(colors[gi])
			for _, r := range group {
				// p.logger.Debug("Mark %d", i+1)
				rect := c.NewRectangle(r.Llx, h-r.Lly, r.Urx-r.Llx, -(r.Ury - r.Lly))
				rect.SetBorderColor(borderColor)
				rect.SetBorderWidth(1.0)
				err = c.Draw(rect)
				if err != nil {
					return fmt.Errorf("Draw failed. pageNum=%d err=%v", pageNum, err)
				}
			}
		}
	}

	c.SetOutlineTree(params.pdfReader.GetOutlineTree())
	if err := c.WriteToFile(saveParams.markupOutputPath); err != nil {
		return fmt.Errorf("WriteToFile failed. err=%v", err)
	}

	p.logger.Info("Saved marked-up PDF file", "path", saveParams.markupOutputPath)
	return nil
}
