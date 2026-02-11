package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// statsRecord holds collected LOC and documentation word counts.
type statsRecord struct {
	GoProdLOC int `json:"go_loc_prod"`
	GoTestLOC int `json:"go_loc_test"`
	GoLOC     int `json:"go_loc"`
	PrdWords  int `json:"spec_wc_prd"`
	UcWords   int `json:"spec_wc_uc"`
	TestWords int `json:"spec_wc_test"`
}

// collectStats gathers Go LOC and documentation word counts.
func collectStats() (statsRecord, error) {
	var prodLines, testLines int

	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if path == "vendor" || path == ".git" || path == binaryDir {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		// Skip magefiles — they are build tooling, not project code.
		if strings.HasPrefix(path, "magefiles") {
			return nil
		}
		count, countErr := countLines(path)
		if countErr != nil {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			testLines += count
		} else {
			prodLines += count
		}
		return nil
	})
	if err != nil {
		return statsRecord{}, err
	}

	prdWords, err := countWordsInGlob("docs/specs/product-requirements/*.yaml")
	if err != nil {
		return statsRecord{}, err
	}
	ucWords, err := countWordsInGlob("docs/specs/use-cases/*.yaml")
	if err != nil {
		return statsRecord{}, err
	}
	testWords, err := countWordsInGlob("docs/specs/test-suites/*.yaml")
	if err != nil {
		return statsRecord{}, err
	}

	return statsRecord{
		GoProdLOC: prodLines,
		GoTestLOC: testLines,
		GoLOC:     prodLines + testLines,
		PrdWords:  prdWords,
		UcWords:   ucWords,
		TestWords: testWords,
	}, nil
}

// Stats prints Go lines of code and documentation word counts.
func Stats() error {
	rec, err := collectStats()
	if err != nil {
		return err
	}
	line, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	fmt.Println(string(line))
	return nil
}

func countLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		count++
	}
	return count, scanner.Err()
}

func countWordsInGlob(pattern string) (int, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return 0, nil
	}
	total := 0
	for _, path := range matches {
		words, wordErr := countWordsInFile(path)
		if wordErr != nil {
			continue
		}
		total += words
	}
	return total, nil
}

func countWordsInFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	count := 0
	inWord := false
	for _, r := range string(data) {
		if unicode.IsSpace(r) {
			inWord = false
		} else if !inWord {
			inWord = true
			count++
		}
	}
	return count, nil
}
