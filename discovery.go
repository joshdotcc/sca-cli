package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
		case "requirements.txt":
			fmt.Printf("Detected requirements.txt at: %s\n", path) // Debug log
			found["python"] = append(found["python"], path)
		case "setup.py", "pipfile", "pyproject.toml":
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
