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
	var allowedLangs string

	flag.StringVar(&repoURL, "repo", "", "git repository URL to clone")
	flag.StringVar(&targetDir, "dir", "./repo", "target directory for the repository")
	flag.BoolVar(&skipCloneFlag, "skip-clone", false, "skip cloning and analyze existing directory")
	flag.StringVar(&outputFmt, "output", "cli", "output format: cli or json")
	flag.StringVar(&outputFile, "o", "", "filepath to write JSON output (must end in .json)")
	flag.StringVar(&allowedLangs, "langs", "", "comma-separated list of languages to include (e.g., Go,Python,Node)")
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

	// Parse allowed languages
	allowedSet := make(map[string]bool)
	if allowedLangs != "" {
		langs := strings.Split(allowedLangs, ",")
		for _, lang := range langs {
			lang = strings.TrimSpace(lang)
			if lang != "" {
				// Normalize to match the keys used in detectPackageManagers
				normalized := normalizeLangName(lang)
				allowedSet[normalized] = true
			}
		}
	}

	// Validate output file extension
	if outputFile != "" && !strings.HasSuffix(strings.ToLower(outputFile), ".json") {
		fmt.Println("Error: Output file must have .json extension")
		os.Exit(1)
	}

	if repoURL == "" && (targetDir == "" || !pathExists(targetDir)) {
		fmt.Println("Usage: sca-cli <git-url> [-dir <path>] [-o filepath.json] [--langs Go,Python,...] or point -dir to an existing checkout")
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

	// Filter managers based on allowed languages
	if len(allowedSet) > 0 {
		filteredManagers := make(map[string][]string)
		for key, files := range managers {
			normalized := normalizeLangKey(key)
			if allowedSet[normalized] {
				filteredManagers[key] = files
			}
		}
		managers = filteredManagers
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

// normalizeLangName normalizes user input language names to match detection keys
func normalizeLangName(lang string) string {
	lower := strings.ToLower(strings.TrimSpace(lang))
	switch lower {
	case "go", "golang":
		return "go"
	case "node", "nodejs", "npm", "javascript", "js":
		return "node"
	case "yarn":
		return "yarn"
	case "python", "py":
		return "python"
	case "maven", "java":
		return "maven"
	case "gradle":
		return "gradle"
	case "composer", "php":
		return "composer"
	case "ruby", "rb":
		return "ruby"
	case "rust", "rs":
		return "rust"
	case "swift":
		return "swift"
	default:
		return lower
	}
}

// normalizeLangKey normalizes detection keys to match user input
func normalizeLangKey(key string) string {
	switch key {
	case "go":
		return "go"
	case "node/npm":
		return "node"
	case "node/yarn":
		return "yarn"
	case "python":
		return "python"
	case "maven":
		return "maven"
	case "gradle":
		return "gradle"
	case "composer/php":
		return "composer"
	case "ruby":
		return "ruby"
	case "rust":
		return "rust"
	case "swift":
		return "swift"
	default:
		return key
	}
}
