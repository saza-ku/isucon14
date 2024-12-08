package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// TODO: settings
var (
	vars = map[string]string{
		"GITHUB_TOKEN":     "",
		"GITHUB_REPO":      "",
		"ISUCON1_IP":       "",
		"ISUCON2_IP":       "",
		"ISUCON3_IP":       "",
		"MYSQL_USER":       "", // root
		"MYSQL_PASS":       "", // root
		"MYSQL_DBNAME":     "",
		"APP_NAME":         "", // binary file name of app
		"APP_SERVICE_NAME": "",
	}

	urlMatchingGroupsList = []string{
		// "/api/hoge",
		// "/api/fuga/.+",
		// `/api/piyo/.+\.js`,
	}

	ignoreDirs = []string{
		".git",
		"etc",
	}
)

func init() {
	urlMatchingGroupsStr := ""
	for _, str := range urlMatchingGroupsList {
		urlMatchingGroupsStr += fmt.Sprintf("- %s\n", str)
	}

	vars["MATCHING_GROUPS"] = urlMatchingGroupsStr
}

// whether skip directory or not
func shouldSkip(d fs.DirEntry) bool {
	for _, dir := range ignoreDirs {
		if strings.Contains(d.Name(), dir) {
			return true
		}
	}

	return false
}

// whether ignore file or not
func shouldIgnore(d fs.DirEntry) bool {
	if isSymLink(d) {
		return true
	}

	return false
}

func main() {
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			fmt.Println("Error:", err)
			return err
		}

		if d.IsDir() {
			if shouldSkip(d) {
				return filepath.SkipDir
			}

			return nil
		}

		if shouldIgnore(d) {
			return nil
		}

		fmt.Println("path: ", path)

		for k, v := range vars {
			if v == "" {
				continue
			}

			before := fmt.Sprintf("<PLACEHOLDER_%s>", k)
			after := v
			if err := replace(path, before, after); err != nil {
				fmt.Println("Error:", err)
				return err
			}
		}

		return nil
	})

	if err != nil {
		fmt.Println("Error:", err)
	}
}

func replace(path, before, after string) error {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	content := string(bytes)
	newContent := strings.ReplaceAll(content, before, after)

	if content == newContent {
		return nil
	}

	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return err
	}

	return nil
}

func isSymLink(d fs.DirEntry) bool {
	if d.Type()&os.ModeSymlink != 0 {
		return true
	}

	return false
}
