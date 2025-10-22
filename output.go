package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

/************************************
* Analysis struct for output
*************************************/
type Analysis struct {
	Repo         string                         `json:"repo"`
	Type         []string                       `json:"type"`
	Dependencies map[string]map[string][]string `json:"dependencies"`
	Files        []string                       `json:"files"`
}

/************************************
* Function Name: analyzeRepository
* Purpose: Build a high-level analysis including types, dependencies and files.
* Parameters: repoURL string, root string, managers map[string][]string
* Output: Analysis
*************************************/
func analyzeRepository(repoURL, root string, managers map[string][]string) Analysis {
	var a Analysis
	if repoURL != "" {
		a.Repo = repoURL
	} else {
		a.Repo = filepath.Base(root)
	}

	// Types
	types := make([]string, 0, len(managers))
	for k := range managers {
		types = append(types, niceName(k))
	}
	sort.Strings(types)
	a.Type = types

	// Files
	fileSet := map[string]struct{}{}
	for _, paths := range managers {
		for _, p := range paths {
			rel, err := filepath.Rel(root, p)
			if err != nil {
				rel = p
			}
			fileSet[rel] = struct{}{}
		}
	}
	for f := range fileSet {
		a.Files = append(a.Files, f)
	}
	sort.Strings(a.Files)

	// Dependencies per ecosystem -> file -> deps
	a.Dependencies = map[string]map[string][]string{}
	for k, paths := range managers {
		eco := niceName(k)
		perFile := map[string][]string{}
		switch k {
		case "go":
			for _, p := range paths {
				rel, err := filepath.Rel(root, p)
				if err != nil {
					rel = p
				}
				perFile[rel] = parseGoModDeps(p)
			}
		case "node/npm":
			for _, p := range paths {
				rel, err := filepath.Rel(root, p)
				if err != nil {
					rel = p
				}
				perFile[rel] = parsePackageJSONDeps(p)
			}
		case "maven":
			for _, p := range paths {
				rel, err := filepath.Rel(root, p)
				if err != nil {
					rel = p
				}
				perFile[rel] = parsePomDeps(p, root)
			}
		case "gradle":
			for _, p := range paths {
				rel, err := filepath.Rel(root, p)
				if err != nil {
					rel = p
				}
				perFile[rel] = parseGradleDeps(p)
			}
		case "rust":
			perFile := map[string][]string{}
			for _, p := range paths {
				deps := parseCargoTomlDeps(p)
				if len(deps) > 0 {
					rel, err := filepath.Rel(root, p)
					if err != nil {
						rel = p
					}
					perFile[rel] = deps
				}
			}
			if len(perFile) > 0 {
				a.Dependencies[eco] = perFile
			}
		case "python":
			for _, p := range paths {
				rel, err := filepath.Rel(root, p)
				if err != nil {
					rel = p
				}
				if strings.HasSuffix(p, "setup.py") {
					perFile[rel] = parseSetupPyDeps(p)
				} else {
					perFile[rel] = parseRequirementsTxtDeps(p)
				}
			}
		case "swift":
			for _, p := range paths {
				rel, err := filepath.Rel(root, p)
				if err != nil {
					rel = p
				}
				perFile[rel] = parsePackageSwiftDeps(p)
			}
		case "ruby":
			for _, p := range paths {
				rel, err := filepath.Rel(root, p)
				if err != nil {
					rel = p
				}
				perFile[rel] = parseGemfileDeps(p)
			}
		default:
			for _, p := range paths {
				rel, err := filepath.Rel(root, p)
				if err != nil {
					rel = p
				}
				perFile[rel] = []string{}
			}
		}
		if _, exists := a.Dependencies[eco]; !exists {
			a.Dependencies[eco] = map[string][]string{}
		}
		for file, deps := range perFile {
			a.Dependencies[eco][file] = deps
		}
	}

	return a
}

/************************************
* Pretty printing helpers
*************************************/
func printHeader(repo string) {
	title := fmt.Sprintf(" SCA Scan: %s ", repo)
	border := strings.Repeat("=", len(title))
	fmt.Println()
	fmt.Println(border)
	fmt.Println(title)
	fmt.Println(border)
	fmt.Println()
}

func printDivider() {
	fmt.Println(strings.Repeat("-", 60))
}

func printDependencies(m map[string]map[string][]string) {
	ecos := make([]string, 0, len(m))
	for k := range m {
		ecos = append(ecos, k)
	}

	fmt.Println(ecos)
	sort.Strings(ecos)
	if len(ecos) == 0 {
		fmt.Println("  (none)")
		return
	}
	for _, eco := range ecos {
		fmt.Printf("- %s:\n", eco)
		files := make([]string, 0, len(m[eco]))
		for f := range m[eco] {
			files = append(files, f)
		}
		sort.Strings(files)
		for _, f := range files {
			fmt.Printf("  %s:\n", f)
			deps := m[eco][f]
			if len(deps) == 0 {
				fmt.Println("    (none)")
				continue
			}
			for _, dep := range deps {
				fmt.Printf("    - %s\n", dep)
			}
		}
	}
}

func printFooter() {
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("Scan complete.")
}
