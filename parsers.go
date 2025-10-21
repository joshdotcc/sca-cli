package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func setToSortedSlice(s map[string]struct{}) []string {
	list := make([]string, 0, len(s))
	for k := range s {
		list = append(list, k)
	}
	sort.Strings(list)
	return list
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
*          This is a conservative line-based parser that handles 'require' blocks,
*          single-line requires, comments (//), and simple replace directives.
* Parameters: path string
* Output: []string (format: module@version or module => replacement)
*************************************/
func parseGoModDeps(path string) []string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	text := string(b)
	lines := strings.Split(text, "\n")
	inBlock := false
	deps := map[string]struct{}{}

	for _, raw := range lines {
		ln := strings.TrimSpace(raw)
		if ln == "" {
			continue
		}
		// strip inline comments
		if idx := strings.Index(ln, "//"); idx != -1 {
			ln = strings.TrimSpace(ln[:idx])
			if ln == "" {
				continue
			}
		}

		// handle block start
		if strings.HasPrefix(ln, "require (") || ln == "require(" {
			inBlock = true
			continue
		}
		// handle block end
		if inBlock {
			if strings.HasPrefix(ln, ")") {
				inBlock = false
				continue
			}
			// expect: module version
			parts := strings.Fields(ln)
			if len(parts) >= 2 {
				name := parts[0]
				ver := parts[1]
				deps[fmt.Sprintf("%s@%s", name, ver)] = struct{}{}
			}
			continue
		}

		// single-line require: require module version
		if strings.HasPrefix(ln, "require ") {
			rest := strings.TrimSpace(strings.TrimPrefix(ln, "require"))
			// rest may be '( ' which we handled, otherwise module version
			parts := strings.Fields(rest)
			if len(parts) >= 2 {
				name := parts[0]
				ver := parts[1]
				deps[fmt.Sprintf("%s@%s", name, ver)] = struct{}{}
			}
			continue
		}

		// replace directives: support 'replace old => new' or 'replace old new'
		if strings.HasPrefix(ln, "replace ") {
			rest := strings.TrimSpace(strings.TrimPrefix(ln, "replace"))
			if strings.Contains(rest, "=>") {
				sides := strings.SplitN(rest, "=>", 2)
				left := strings.Fields(strings.TrimSpace(sides[0]))
				right := strings.Fields(strings.TrimSpace(sides[1]))
				if len(left) > 0 && len(right) > 0 {
					from := left[0]
					to := right[0]
					deps[fmt.Sprintf("%s => %s", from, to)] = struct{}{}
				}
			} else {
				parts := strings.Fields(rest)
				if len(parts) >= 2 {
					from := parts[0]
					to := parts[1]
					deps[fmt.Sprintf("%s => %s", from, to)] = struct{}{}
				}
			}
			continue
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
* Function Name: parsePomProperties
* Purpose: Extract properties defined in a pom.xml (<properties>...</properties>).
* Parameters: path string
* Output: map[string]string
*************************************/
func parsePomProperties(path string) map[string]string {
	props := map[string]string{}
	s, err := readFileContent(path)
	if err != nil {
		return props
	}
	// extract properties block
	reProps := regexp.MustCompile(`(?s)<properties>(.*?)</properties>`) 
	if m := reProps.FindStringSubmatch(s); len(m) > 1 {
		inner := m[1]
		// capture each <name>value</name> inside properties
		rePair := regexp.MustCompile(`(?s)<([^>\s]+)>\s*([^<]+)\s*</[^>]+>`)
		for _, pm := range rePair.FindAllStringSubmatch(inner, -1) {
			k := strings.TrimSpace(pm[1])
			v := strings.TrimSpace(pm[2])
			props[k] = v
		}
	}
	return props
}

/************************************
* Function Name: parsePomDependencyManagement
* Purpose: Extract dependencyManagement versions from a pom.xml as map[group:artifact]version
* Parameters: path string
* Output: map[string]string
*************************************/
func parsePomDependencyManagement(path string) map[string]string {
	mmap := map[string]string{}
	s, err := readFileContent(path)
	if err != nil {
		return mmap
	}
	// find dependencyManagement block
	reDM := regexp.MustCompile(`(?s)<dependencyManagement>(.*?)</dependencyManagement>`) 
	if m := reDM.FindStringSubmatch(s); len(m) > 1 {
		inner := m[1]
		// find dependency blocks inside
		reDep := regexp.MustCompile(`(?s)<dependency>(.*?)</dependency>`)
		reGroup := regexp.MustCompile(`<groupId>\s*([^<\s]+)\s*</groupId>`)
		reArtifact := regexp.MustCompile(`<artifactId>\s*([^<\s]+)\s*</artifactId>`)
		reVersion := regexp.MustCompile(`<version>\s*([^<\s]+)\s*</version>`)
		for _, dm := range reDep.FindAllStringSubmatch(inner, -1) {
			block := dm[1]
			g := ""
			a := ""
			v := ""
			if gm := reGroup.FindStringSubmatch(block); len(gm) > 1 {
				g = strings.TrimSpace(gm[1])
			}
			if am := reArtifact.FindStringSubmatch(block); len(am) > 1 {
				a = strings.TrimSpace(am[1])
			}
			if vm := reVersion.FindStringSubmatch(block); len(vm) > 1 {
				v = strings.TrimSpace(vm[1])
			}
			if g != "" && a != "" {
				mmap[fmt.Sprintf("%s:%s", g, a)] = v
			}
		}
	}
	return mmap
}

/************************************
* Function Name: resolvePomValue
* Purpose: Resolve ${...} placeholders using a properties map; leaves unknown placeholders intact.
* Parameters: val string, props map[string]string
* Output: string
*************************************/
func resolvePomValue(val string, props map[string]string) string {
	reVar := regexp.MustCompile(`\$\{([^}]+)\}`)
	res := reVar.ReplaceAllStringFunc(val, func(match string) string {
		m := reVar.FindStringSubmatch(match)
		if len(m) > 1 {
			if v, ok := props[m[1]]; ok {
				return v
			}
		}
		return match
	})
	return res
}

/************************************
* Function Name: aggregatePomData
* Purpose: Walk the repository and aggregate properties and dependencyManagement
*          entries from all pom.xml files to help resolve placeholders.
* Parameters: repoRoot string
* Output: (properties map, dependencyManagement map)
*************************************/
func aggregatePomData(repoRoot string) (map[string]string, map[string]string) {
	allProps := map[string]string{}
	allDM := map[string]string{}
	// walk repository for pom.xml
	filepath.Walk(repoRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, "pom.xml") {
			p := parsePomProperties(path)
			for k, v := range p {
				if _, ok := allProps[k]; !ok {
					allProps[k] = v
				}
			}
			d := parsePomDependencyManagement(path)
			for k, v := range d {
				if _, ok := allDM[k]; !ok {
					allDM[k] = v
				}
			}
		}
		return nil
	})
	return allProps, allDM
}

/************************************
* Function Name: parsePomDeps
* Purpose: Extract dependencies from a pom.xml file as group:artifact@version (version optional).
*          If the version is a property placeholder like ${...}, the version is omitted.
* Parameters: path string, repoRoot string (repoRoot kept for signature compatibility)
* Output: []string
*************************************/
func parsePomDeps(path string, repoRoot string) []string {
	s, err := readFileContent(path)
	if err != nil {
		return nil
	}
	deps := map[string]struct{}{}

	// find dependency blocks
	reDep := regexp.MustCompile(`(?s)<dependency>(.*?)</dependency>`)
	reGroup := regexp.MustCompile(`<groupId>\s*([^<\s]+)\s*</groupId>`)
	reArtifact := regexp.MustCompile(`<artifactId>\s*([^<\s]+)\s*</artifactId>`)
	reVersion := regexp.MustCompile(`<version>\s*([^<\s]+)\s*</version>`)

	for _, m := range reDep.FindAllStringSubmatch(s, -1) {
		block := m[1]
		g := ""
		a := ""
		v := ""
		if gm := reGroup.FindStringSubmatch(block); len(gm) > 1 {
			g = strings.TrimSpace(gm[1])
		}
		if am := reArtifact.FindStringSubmatch(block); len(am) > 1 {
			a = strings.TrimSpace(am[1])
		}
		if vm := reVersion.FindStringSubmatch(block); len(vm) > 1 {
			v = strings.TrimSpace(vm[1])
		}
		if g == "" && a == "" {
			continue
		}
		// If version is a property placeholder (${...}), treat as unspecified
		if strings.Contains(v, "${") {
			v = ""
		}
		var key string
		if v != "" {
			key = fmt.Sprintf("%s:%s@%s", g, a, v)
		} else {
			key = fmt.Sprintf("%s:%s", g, a)
		}
		deps[key] = struct{}{}
	}

	return setToSortedSlice(deps)
}

/************************************
* Function Name: parseGradleDeps
* Purpose: Extract dependencies from build.gradle (and kotlin DSL) in forms like
*          implementation 'group:artifact:version' or map-style group: 'g', name: 'a', version: 'v'.
* Parameters: path string
* Output: []string (format: group:artifact@version)
*************************************/
func parseGradleDeps(path string) []string {
	s, err := readFileContent(path)
	if err != nil {
		return nil
	}
	deps := map[string]struct{}{}

	// match simple string notation: configuration 'group:artifact:version' or "group:artifact:version"
	reSimple := regexp.MustCompile(`(?m)^\s*(?:implementation|api|compile|compileOnly|runtimeOnly|runtime|testImplementation|testCompile|testRuntimeOnly|testRuntime)\s*\(?['"]([^'"\)]+)['"]\)?`)
	for _, m := range reSimple.FindAllStringSubmatch(s, -1) {
		parts := strings.Split(m[1], ":")
		if len(parts) >= 2 {
			g := parts[0]
			a := parts[1]
			ver := ""
			if len(parts) >= 3 {
				ver = strings.Join(parts[2:], ":")
			}
			if ver != "" {
				deps[fmt.Sprintf("%s:%s@%s", g, a, ver)] = struct{}{}
			} else {
				deps[fmt.Sprintf("%s:%s", g, a)] = struct{}{}
			}
		}
	}

	// match map-style notation: implementation group: 'g', name: 'a', version: 'v'
	reMap := regexp.MustCompile(`(?m)^[ \t]*(?:implementation|api|compile|testImplementation|testCompile|runtimeOnly|testRuntimeOnly)[ \t]+([^\n]+)`)
	for _, m := range reMap.FindAllStringSubmatch(s, -1) {
		line := m[1]
		// find group/name/version in the line
		reG := regexp.MustCompile(`group\s*:\s*['\"]([^'\"]+)['\"]`)
		reA := regexp.MustCompile(`name\s*:\s*['\"]([^'\"]+)['\"]`)
		reV := regexp.MustCompile(`version\s*:\s*['\"]([^'\"]+)['\"]`)
		g := ""
		a := ""
		v := ""
		if gm := reG.FindStringSubmatch(line); len(gm) > 1 {
			g = gm[1]
		}
		if am := reA.FindStringSubmatch(line); len(am) > 1 {
			a = am[1]
		}
		if vm := reV.FindStringSubmatch(line); len(vm) > 1 {
			v = vm[1]
		}
		if g != "" && a != "" {
			if v != "" {
				deps[fmt.Sprintf("%s:%s@%s", g, a, v)] = struct{}{}
			} else {
				deps[fmt.Sprintf("%s:%s", g, a)] = struct{}{}
			}
		}
	}

	return setToSortedSlice(deps)
}
