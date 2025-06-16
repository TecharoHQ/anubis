/*
Batch process robots.txt files from archives like https://github.com/nrjones8/robots-dot-txt-archive-bot/tree/master/data/cleaned
into Anubis CEL policies. Usage: go run batch_process.go <directory with robots.txt files>
*/
package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run batch_process.go <cleaned_directory>")
		fmt.Println("Example: go run batch_process.go ./cleaned")
		os.Exit(1)
	}

	cleanedDir := os.Args[1]
	outputDir := "generated_policies"

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	count := 0
	err := filepath.WalkDir(cleanedDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Generate policy name from file path
		relPath, _ := filepath.Rel(cleanedDir, path)
		policyName := strings.ReplaceAll(relPath, "/", "-")
		policyName = strings.TrimSuffix(policyName, "-robots.txt")
		policyName = strings.ReplaceAll(policyName, ".", "-")

		outputFile := filepath.Join(outputDir, policyName+".yaml")

		cmd := exec.Command("go", "run", "main.go",
			"-input", path,
			"-output", outputFile,
			"-name", policyName,
			"-format", "yaml")

		if err := cmd.Run(); err != nil {
			fmt.Printf("Warning: Failed to process %s: %v\n", path, err)
			return nil // Continue processing other files
		}

		count++
		if count%100 == 0 {
			fmt.Printf("Processed %d files...\n", count)
		} else if count%10 == 0 {
			fmt.Print(".")
		}

		return nil
	})

	if err != nil {
		log.Fatalf("Error walking directory: %v", err)
	}

	fmt.Printf("Successfully processed %d robots.txt files\n", count)
	fmt.Printf("Generated policies saved to: %s/\n", outputDir)
}
