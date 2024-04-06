package git

import (
	"bufio"
	"bytes"
	"os/exec"
	"strings"
)

type Repository struct {
	Path string
}

type Commit struct {
	Hash    string
	Author  string
	Date    string
	Message string
	Diffs   []FileDiff
}

type FileDiff struct {
	Path         string
	IndexBefore  string
	IndexAfter   string
	DiffContents string
}

func (r *Repository) CommitsOnBranch(branch, parent string) ([]Commit, error) {
	out, err := r.execGit("merge-base", parent, branch)
	if err != nil {
		return nil, err
	}

	base := strings.TrimSpace(string(out))
	revs := base + ".." + branch
	output, err := r.execGit("log", "--first-parent", "-p", "--no-color", revs)
	if err != nil {
		return nil, err
	}

	return r.parseLog(output)
}

func (r *Repository) execGit(args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Path
	return cmd.Output()
}

func (r *Repository) parseLog(output []byte) ([]Commit, error) {
	buf := []byte{}
	scanner := bufio.NewScanner(bytes.NewReader(output))
	scanner.Buffer(buf, 2048*1024)

	var commits []Commit
	var currentCommit *Commit
	var currentDiff *FileDiff

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "commit ") {
			if currentCommit != nil {
				if currentDiff != nil {
					currentCommit.Diffs = append(currentCommit.Diffs, *currentDiff)
				}
				commits = append(commits, *currentCommit)
			}
			currentCommit = &Commit{
				Hash: strings.TrimPrefix(line, "commit "),
			}
			currentDiff = nil
		} else if strings.HasPrefix(line, "Author: ") {
			currentCommit.Author = strings.TrimPrefix(line, "Author: ")
		} else if strings.HasPrefix(line, "Date:   ") {
			currentCommit.Date = strings.TrimPrefix(line, "Date:   ")
		} else if strings.HasPrefix(line, "diff --git ") {
			if currentDiff != nil {
				currentCommit.Diffs = append(currentCommit.Diffs, *currentDiff)
			}

			path := strings.TrimPrefix(strings.Fields(line)[2], "a/")
			currentDiff = &FileDiff{
				Path: path,
			}
		} else if strings.HasPrefix(line, "index ") {
			if currentDiff != nil {
				indexes := strings.Fields(line)[1]
				currentDiff.IndexBefore = indexes[:strings.Index(indexes, "..")]
				currentDiff.IndexAfter = indexes[strings.Index(indexes, "..")+2:]
			}
		} else if strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "--- ") {
			// skip
		} else if currentDiff != nil {
			// Skip SVG content
			if strings.Contains(currentDiff.Path, ".svg") {
				continue
			}
			currentDiff.DiffContents += line + "\n"
		} else {
			currentCommit.Message += line + "\n"
		}
	}

	if currentDiff != nil && currentCommit != nil {
		currentCommit.Diffs = append(currentCommit.Diffs, *currentDiff)
	}

	if currentCommit != nil {
		commits = append(commits, *currentCommit)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return commits, nil
}
