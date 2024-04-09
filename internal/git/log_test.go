package git

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestCommitsOnBranch(t *testing.T) {
	// Create a temporary directory for the test repository
	tempDir := t.TempDir()

	defaultBranch, err := initTestRepo(tempDir)
	if err != nil {
		t.Fatalf("failed to initialize test repository: %v", err)
	}

	// Create a test file and commit it to the repository
	testFile := "test.txt"
	err = createCommit(tempDir, testFile, "initial content", "Initial commit")
	if err != nil {
		t.Fatalf("failed to create commit: %v", err)
	}

	// Create a new Repository instance for the test
	repo := &Repository{Path: tempDir}

	// Call the LogsOnBranch function with a test branch name
	branch := "test-branch"
	cmd := exec.Command("git", "checkout", "-b", branch)
	cmd.Dir = tempDir
	err = cmd.Run()
	if err != nil {
		t.Fatalf("failed to create test branch: %v", err)
	}

	// check branches
	cmd = exec.Command("git", "branch")
	cmd.Dir = tempDir
	cmdOutput, _ := cmd.Output()
	fmt.Printf("Branches: %s\n", cmdOutput)

	commits, err := repo.CommitsOnBranch(branch, defaultBranch)
	if err != nil {
		t.Fatalf("LogsOnBranch failed: %v", err)
	}

	// Check if the returned commits contain the expected commit
	if len(commits) != 0 {
		t.Fatalf("expected 0 commit, got %d", len(commits))
	}

	// Make some commits on the test branch
	err = createCommit(tempDir, testFile, "updated content", "Update test file")
	if err != nil {
		t.Fatalf("failed to create commit: %v", err)
	}

	// Call the LogsOnBranch function again with the test branch name
	commits, err = repo.CommitsOnBranch(branch, defaultBranch)
	if err != nil {
		t.Fatalf("LogsOnBranch failed: %v", err)
	}

	// Check if the returned commits contain the expected commit
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}
	if strings.TrimSpace(commits[0].Message) != "Update test file" {
		t.Fatalf("expected commit message 'Update test file', got '%s'", commits[0].Message)
	}

	// Cleanup the temporary directory
	err = os.RemoveAll(tempDir)
	if err != nil {
		t.Fatalf("failed to cleanup temporary directory: %v", err)
	}
}

// Initialize a new git repository in the temporary directory
func initTestRepo(dir string) (string, error) {
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	err := cmd.Run()
	if err != nil {
		return "", err
	}

	// Remember default branch name
	cmd = exec.Command("git", "branch", "--show-current")
	cmd.Dir = dir
	cmdOutput, _ := cmd.Output()
	defaultBranch := string(cmdOutput)
	defaultBranch = defaultBranch[:len(defaultBranch)-1]
	fmt.Printf("Default branch: %s\n", defaultBranch)

	return defaultBranch, nil
}

// Helper function to create a new commit in the test repository
func createCommit(tempDir, testFile, fileContent, message string) error {
	err := os.WriteFile(tempDir+"/"+testFile, []byte(fileContent), 0644)
	if err != nil {
		return err
	}
	cmd := exec.Command("git", "add", testFile)
	cmd.Dir = tempDir
	err = cmd.Run()
	if err != nil {
		return err
	}
	cmd = exec.Command("git", "commit", "-m", message)
	cmd.Dir = tempDir
	err = cmd.Run()
	if err != nil {
		return err
	}
	return nil
}
