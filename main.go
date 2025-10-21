package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func main() {
	// CLI flags (user-friendly names)
	var repoURL string
	var targetDir string
	var skipCloneFlag bool
	var outputFmt string

	flag.StringVar(&repoURL, "repo", "", "git repository URL to clone")
	flag.StringVar(&targetDir, "dir", "./repo", "target directory for the repository")
	flag.BoolVar(&skipCloneFlag, "skip-clone", false, "skip cloning and analyze existing directory")
	flag.StringVar(&outputFmt, "output", "cli", "output format: cli or json")
	flag.Parse()

	if repoURL == "" && (targetDir == "" || !pathExists(targetDir)) {
		fmt.Println("Usage: sca-cli -repo <git-url> [-dir <path>] or point -dir to an existing checkout")
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

	if strings.ToLower(outputFmt) == "json" {
		enc, err := json.MarshalIndent(analysis, "", "  ")
		if err != nil {
			log.Fatalf("failed to marshal json: %v", err)
		}
		fmt.Println(string(enc))
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

/************************************
* Function Name: pathExists
* Purpose: Check whether a filesystem path exists.
* Parameters: path string
* Output: bool
*************************************/
func pathExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

/************************************
* Function Name: cloneRepository
* Purpose: Clones a git repository into a specified directory.
* Parameters: repo string, dir string
* Output: error
*************************************/
func cloneRepository(repo, dir string) error {
	cmd := exec.Command("git", "clone", "--depth", "1", repo, dir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

/************************************
* Function Name: detectPackageManagers
* Purpose: Walks a repository tree and detects files that indicate
*          which package managers or ecosystems are in use.
* Parameters: root string
* Output: map[string][]string, error
*************************************/
func detectPackageManagers(root string) (map[string][]string, error) {
	found := make(map[string][]string)

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			// skip .git
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		name := strings.ToLower(info.Name())

		switch name {
		case "go.mod":
			found["go"] = append(found["go"], path)
		case "package.json":
			found["node/npm"] = append(found["node/npm"], path)
		case "yarn.lock":
			found["node/yarn"] = append(found["node/yarn"], path)
		case "requirements.txt", "setup.py", "pipfile", "pyproject.toml":
			found["python"] = append(found["python"], path)
		case "pom.xml":
			found["maven"] = append(found["maven"], path)
		case "build.gradle", "build.gradle.kts", "gradle.properties":
			found["gradle"] = append(found["gradle"], path)
		case "composer.json":
			found["composer/php"] = append(found["composer/php"], path)
		case "gemfile":
			found["ruby"] = append(found["ruby"], path)
		case "cargo.toml":
			found["rust"] = append(found["rust"], path)
		case "package.swift":
			found["swift"] = append(found["swift"], path)
		}
		return nil
	}

	err := filepath.Walk(root, walkFn)
	if err != nil {
		return nil, err
	}
	return found, nil
}

/************************************
* Function Name: readFileContent
* Purpose: Read a file and return its contents as a string.
* Parameters: path string
* Output: string, error
*************************************/
func readFileContent(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

/************************************
* Analysis struct for output
*************************************/
type Analysis struct {
	Repo         string              `json:"repo"`
	Type         []string            `json:"type"`
	Dependencies map[string][]string `json:"dependencies"`
	Files        []string            `json:"files"`
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
			fileSet[filepath.Base(p)] = struct{}{}
		}
	}
	for f := range fileSet {
		a.Files = append(a.Files, f)
	}
	sort.Strings(a.Files)

	// Dependencies
	a.Dependencies = map[string][]string{}
	for k, paths := range managers {
		switch k {
		case "go":
			set := map[string]struct{}{}
			for _, p := range paths {
				for _, d := range parseGoModDeps(p) {
					set[d] = struct{}{}
				}
			}
			list := setToSortedSlice(set)
			a.Dependencies[niceName(k)] = list
		case "node/npm":
			set := map[string]struct{}{}
			for _, p := range paths {
				for _, d := range parsePackageJSONDeps(p) {
					set[d] = struct{}{}
				}
			}
			list := setToSortedSlice(set)
			a.Dependencies[niceName(k)] = list
		default:
			// other ecosystems: leave empty for now
			a.Dependencies[niceName(k)] = []string{}
		}
	}

	return a
}

func setToSortedSlice(s map[string]struct{}) []string {
	list := make([]string, 0, len(s))
	for k := range s {
		list = append(list, k)
	}
	sort.Strings(list)
	return list
}

func niceName(key string) string {
	switch key {
	case "go":
		return "Go"
	case "node/npm":
		return "Node"
	case "node/yarn":
		return "Yarn"
	case "python":
		return "Python"
	case "maven":
		return "Maven"
	case "gradle":
		return "Gradle"
	case "composer/php":
		return "Composer"
	case "ruby":
		return "Ruby"
	case "rust":
		return "Rust"
	case "swift":
		return "Swift"
	default:
		return key
	}
}

/************************************
* Function Name: parseGoModDeps
* Purpose: Extract module dependency names and versions from a go.mod file.
* Parameters: path string
* Output: []string (format: name@version)
*************************************/
func parseGoModDeps(path string) []string {
	s, err := readFileContent(path)
	if err != nil {
		return nil
	}
	deps := map[string]struct{}{}

	// match single-line require: require github.com/foo v1.2.3
	re1 := regexp.MustCompile(`(?m)^\s*require\s+([^\s]+)\s+([^\s]+)`) 
	for _, m := range re1.FindAllStringSubmatch(s, -1) {
		name := m[1]
		ver := m[2]
		deps[fmt.Sprintf("%s@%s", name, ver)] = struct{}{}
	}

	// match block require: require ( ... )
	reBlock := regexp.MustCompile(`(?s)require\s*\((.*?)\)`) 
	for _, bm := range reBlock.FindAllStringSubmatch(s, -1) {
		inner := bm[1]
		reLine := regexp.MustCompile(`([^\s]+)\s+([^\s]+)`) 
		for _, lm := range reLine.FindAllStringSubmatch(inner, -1) {
			name := lm[1]
			ver := lm[2]
			deps[fmt.Sprintf("%s@%s", name, ver)] = struct{}{}
		}
	}

	return setToSortedSlice(deps)
}

/************************************
* Function Name: parsePackageJSONDeps
* Purpose: Extract dependency names and versions from a package.json (dependencies + devDependencies).
* Parameters: path string
* Output: []string (format: name@version)
*************************************/
func parsePackageJSONDeps(path string) []string {
	s, err := readFileContent(path)
	if err != nil {
		return nil
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(s), &data); err != nil {
		return nil
	}
	set := map[string]struct{}{}
	if deps, ok := data["dependencies"].(map[string]interface{}); ok {
		for k, v := range deps {
			ver := ""
			switch vv := v.(type) {
			case string:
				ver = vv
			default:
				ver = fmt.Sprintf("%v", vv)
			}
			set[fmt.Sprintf("%s@%s", k, ver)] = struct{}{}
		}
	}
	if dev, ok := data["devDependencies"].(map[string]interface{}); ok {
		for k, v := range dev {
			ver := ""
			switch vv := v.(type) {
			case string:
				ver = vv
			default:
				ver = fmt.Sprintf("%v", vv)
			}
			set[fmt.Sprintf("%s@%s", k, ver)] = struct{}{}
		}
	}
	return setToSortedSlice(set)
}

/************************************
* Function Name: removePath
* Purpose: Remove a filesystem path recursively with a basic safety check.
* Parameters: path string
* Output: error
*************************************/
func removePath(path string) error {
	if path == "" {
		return fmt.Errorf("refusing to remove empty path")
	}
	// basic safety: do not remove root
	if path == "/" || path == "." || path == ".." {
		return fmt.Errorf("refusing to remove unsafe path: %q", path)
	}
	return os.RemoveAll(path)
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

func printDependencies(m map[string][]string) {
	// determine column width for names across all groups
	maxName := 0
	for _, deps := range m {
		for _, dv := range deps {
			parts := strings.SplitN(dv, "@", 2)
			name := parts[0]
			if len(name) > maxName {
				maxName = len(name)
			}
		}
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		fmt.Println("  (none)")
		return
	}
	for _, k := range keys {
		label := fmt.Sprintf("%s", k)
		fmt.Printf("- %s:\n", label)
		deps := m[k]
		if len(deps) == 0 {
			fmt.Println("    (none)")
			continue
		}
		for _, dv := range deps {
			parts := strings.SplitN(dv, "@", 2)
			name := parts[0]
			ver := ""
			if len(parts) > 1 {
				ver = parts[1]
			}
			fmt.Printf("    - %-*s  @  %s\n", maxName, name, ver)
		}
		fmt.Println()
	}
}

func printFooter() {
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("Scan complete.\n")
}