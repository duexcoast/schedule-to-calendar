package main

import (
	"encoding/csv"
	"log"
	"os"
)

func readCSVFile(filePath string) [][]string {
	f, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("unable to read input file %q err=%v", filePath, err)
	}
	defer f.Close()

	csvReader := csv.NewReader(f)
	records, err := csvReader.ReadAll()
	if err != nil {
		log.Fatalf("unable to parse file as a CSV. %q err=%v", filePath, err)
	}
	return records

}
