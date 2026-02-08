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

// Stats prints Go lines of code and documentation word counts.
func Stats() error {
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
		return err
	}

	prdWords, err := countWordsInGlob("docs/specs/product-requirements/*.yaml")
	if err != nil {
		return err
	}
	ucWords, err := countWordsInGlob("docs/specs/use-cases/*.yaml")
	if err != nil {
		return err
	}
	testWords, err := countWordsInGlob("docs/specs/test-suites/*.yaml")
	if err != nil {
		return err
	}

	record := map[string]int{
		"go_loc_prod":  prodLines,
		"go_loc_test":  testLines,
		"go_loc":       prodLines + testLines,
		"spec_wc_prd":  prdWords,
		"spec_wc_uc":   ucWords,
		"spec_wc_test": testWords,
	}
	line, err := json.Marshal(record)
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
