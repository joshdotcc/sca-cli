package main

import (
	"fmt"
	"os"
	"os/exec"
)

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
