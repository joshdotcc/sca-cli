package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

func main() {
	// CLI flags (user-friendly names)
	var repoURL string
	var targetDir string
	var skipCloneFlag bool
	var outputFmt string
	var outputFile string

	flag.StringVar(&repoURL, "repo", "", "git repository URL to clone")
	flag.StringVar(&targetDir, "dir", "./repo", "target directory for the repository")
	flag.BoolVar(&skipCloneFlag, "skip-clone", false, "skip cloning and analyze existing directory")
	flag.StringVar(&outputFmt, "output", "cli", "output format: cli or json")
	flag.StringVar(&outputFile, "o", "", "filepath to write JSON output (implies JSON)")
	flag.Parse()

	// allow positional first arg as repo URL
	if repoURL == "" && flag.NArg() > 0 {
		repoURL = flag.Arg(0)
	}

	// If -o was provided after positional args it may not have been parsed by flag package.
	// Fallback: scan original os.Args for -o or -o=path and set outputFile if still empty.
	if outputFile == "" {
		for i := 1; i < len(os.Args); i++ {
			arg := os.Args[i]
			if arg == "-o" && i+1 < len(os.Args) {
				outputFile = os.Args[i+1]
				break
			}
			if strings.HasPrefix(arg, "-o=") {
				outputFile = strings.SplitN(arg, "=", 2)[1]
				break
			}
		}
	}

	if repoURL == "" && (targetDir == "" || !pathExists(targetDir)) {
		fmt.Println("Usage: sca-cli <git-url> [-dir <path>] [-o filepath] or point -dir to an existing checkout")
		os.Exit(1)
	}

	if repoURL != "" && !skipCloneFlag {
		if pathExists(targetDir) {
			log.Printf("target dir %s exists; removing to make room\n", targetDir)
			if err := removePath(targetDir); err != nil {
				log.Fatalf("failed to remove existing dir: %v", err)
			}
		}
		log.Printf("cloning %s -> %s\n", repoURL, targetDir)
		if err := cloneRepository(repoURL, targetDir); err != nil {
			log.Fatalf("git clone failed: %v", err)
		}
	}

	managers, err := detectPackageManagers(targetDir)
	if err != nil {
		log.Fatalf("detection failed: %v", err)
	}

	analysis := analyzeRepository(repoURL, targetDir, managers)

	if strings.ToLower(outputFmt) == "json" || outputFile != "" {
		enc, err := json.MarshalIndent(analysis, "", "  ")
		if err != nil {
			log.Fatalf("failed to marshal json: %v", err)
		}
		if outputFile != "" {
			if err := os.WriteFile(outputFile, enc, 0644); err != nil {
				log.Fatalf("failed to write output file: %v", err)
			}
			fmt.Printf("Wrote JSON to %s\n", outputFile)
		} else {
			fmt.Println(string(enc))
		}
		return
	}

	// Pretty CLI output
	printHeader(analysis.Repo)
	printDivider()
	fmt.Printf("Types: %s\n\n", strings.Join(analysis.Type, ", "))
	fmt.Println("Dependencies:")
	printDependencies(analysis.Dependencies)
	printDivider()
	fmt.Printf("Files:\n")
	for _, f := range analysis.Files {
		fmt.Printf("  - %s\n", f)
	}
	fmt.Println()
	printFooter()
}