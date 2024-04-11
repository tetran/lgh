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
		t.Fatal(err)
	}

	// Create a test file and commit it to the repository
	testFile := "test.txt"
	err = createCommit(tempDir, testFile, "initial content", "Initial commit")
	if err != nil {
		t.Fatal(err)
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
	_, err := execGit(dir, "init")
	if err != nil {
		return "", err
	}

	// Remember default branch name
	ret, err := execGit(dir, "branch", "--show-current")
	if err != nil {
		return "", err
	}
	defaultBranch := string(ret)
	defaultBranch = defaultBranch[:len(defaultBranch)-1]
	fmt.Printf("Default branch: %s\n", defaultBranch)

	_, err = execGit(dir, "config", "user.email", "test@example.com")
	if err != nil {
		return "", fmt.Errorf("failed to set user email: %v", err)
	}
	_, err = execGit(dir, "config", "user.name", "Test User")
	if err != nil {
		return "", fmt.Errorf("failed to set user name: %v", err)
	}

	return defaultBranch, nil
}

// Helper function to create a new commit in the test repository
func createCommit(tempDir, testFile, fileContent, message string) error {
	err := os.WriteFile(tempDir+"/"+testFile, []byte(fileContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}

	_, err = execGit(tempDir, "add", testFile)
	if err != nil {
		return err
	}
	_, err = execGit(tempDir, "commit", "-m", message)
	if err != nil {
		return err
	}
	return nil
}

func execGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to execute git command: %v\noutput: %s", err, out)
	}
	return string(out), nil
}
