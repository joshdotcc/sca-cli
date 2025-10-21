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
		default:
			for _, p := range paths {
				rel, err := filepath.Rel(root, p)
				if err != nil {
					rel = p
				}
				perFile[rel] = []string{}
			}
		}
		a.Dependencies[eco] = perFile
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
	// ecosystems
	ecos := make([]string, 0, len(m))
	for k := range m {
		ecos = append(ecos, k)
	}
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
			// determine name width for this file
			maxName := 0
			parsed := make([][2]string, 0, len(deps))
			for _, dv := range deps {
				parts := strings.SplitN(dv, "@", 2)
				name := parts[0]
				ver := ""
				if len(parts) > 1 {
					ver = parts[1]
				}
				parsed = append(parsed, [2]string{name, ver})
				if len(name) > maxName {
					maxName = len(name)
				}
			}
			for _, nv := range parsed {
				fmt.Printf("    - %-*s  @  %s\n", maxName, nv[0], nv[1])
			}
			fmt.Println()
		}
	}
}

func printFooter() {
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("Scan complete.\n")
}
