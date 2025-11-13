package eventlog

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// FindIbcmd attempts to find ibcmd executable automatically
func FindIbcmd() (string, error) {
	// Common paths for ibcmd
	var searchPaths []string
	
	if runtime.GOOS == "windows" {
		// Windows paths
		programFiles := os.Getenv("ProgramFiles")
		if programFiles == "" {
			programFiles = "C:\\Program Files"
		}
		
		// Try common 1C installation paths
		searchPaths = []string{
			filepath.Join(programFiles, "1cv8", "x86_64", "8.3.27.0", "ibcmd.exe"),
			filepath.Join(programFiles, "1cv8", "x86_64", "8.3.26.0", "ibcmd.exe"),
			filepath.Join(programFiles, "1cv8", "x86_64", "8.3.25.0", "ibcmd.exe"),
			filepath.Join(programFiles, "1cv8", "x86_64", "8.3.24.0", "ibcmd.exe"),
			filepath.Join(programFiles, "1cv8", "ibcmd.exe"),
		}
		
		// Also try Program Files (x86) for 32-bit
		programFilesX86 := os.Getenv("ProgramFiles(x86)")
		if programFilesX86 != "" {
			searchPaths = append(searchPaths,
				filepath.Join(programFilesX86, "1cv8", "ibcmd.exe"),
			)
		}
	} else {
		// Linux paths
		searchPaths = []string{
			"/opt/1cv8/x86_64/8.3.27.0/ibcmd",
			"/opt/1cv8/x86_64/8.3.26.0/ibcmd",
			"/opt/1cv8/x86_64/8.3.25.0/ibcmd",
			"/opt/1cv8/x86_64/8.3.24.0/ibcmd",
			"/opt/1cv8/x86_64/ibcmd",
			"/usr/local/1cv8/x86_64/ibcmd",
		}
		
		// Also check PATH
		if path, err := exec.LookPath("ibcmd"); err == nil {
			searchPaths = append([]string{path}, searchPaths...)
		}
	}
	
	// Search for ibcmd
	for _, path := range searchPaths {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			// Check if executable (on Unix)
			if runtime.GOOS != "windows" {
				if info.Mode().Perm()&0111 == 0 {
					continue // Not executable
				}
			}
			
			// Try to run it to verify
			cmd := exec.Command(path, "--version")
			if err := cmd.Run(); err == nil {
				return path, nil
			}
		}
	}
	
	return "", fmt.Errorf("ibcmd not found in common paths: %s", strings.Join(searchPaths, ", "))
}

// VerifyIbcmd checks if ibcmd is available and working
func VerifyIbcmd(path string) error {
	if path == "" {
		return fmt.Errorf("ibcmd path is empty")
	}
	
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("ibcmd not found at: %s", path)
	}
	
	// Try to run ibcmd --version
	cmd := exec.Command(path, "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ibcmd verification failed: %w", err)
	}
	
	return nil
}

