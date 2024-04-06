/*
Copyright © 2024 Koichi Kaneshige <coarse.ground@gmail.com>
*/
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tetran/lgh/internal/config"
	"github.com/tetran/lgh/internal/git"
	"github.com/tetran/lgh/internal/openai"
)

var (
	bsCmd = &cobra.Command{
		Use:   "branch-summary",
		Short: "Summarize the changes made in the specified branch since the base branch.",
		Long:  ``,
		Run:   branchSummary,
	}
)

const instruction = `
## Instruction:
Please summarize the following git commits briefly, using bullet points and word-for-word descriptions.
Focus on the purpose of each commit, ignore the minor file-by-file fixes.
Preferred language is %s.

## Expected Output Format:
* Add feature X to screen A (if the screen name is not clear, assume it based on the file name)
* Change B setting from Y to Z
* Fix C bug

## Commits to summarize:
%s
`

func init() {
	bsCmd.Flags().StringP("base", "b", "main", "Base branch")
	bsCmd.Flags().StringP("target", "t", "", "Target branch")
	bsCmd.Flags().BoolP("debug", "d", false, "Enable debug mode")
}

func branchSummary(cmd *cobra.Command, args []string) {
	key := viper.GetString("openai-api-key")
	if key == "" {
		fmt.Println("OpenAI API key is required. Please set it in the config file (using `lgh config` command) or pass it via the --openai-api-key flag.")
		os.Exit(1)
	}

	base, err := cmd.Flags().GetString("base")
	cobra.CheckErr(err)
	tgt, err := cmd.Flags().GetString("target")
	cobra.CheckErr(err)
	if tgt == "" {
		fmt.Println("Target branch is required")
		os.Exit(1)
	}
	debug, err := cmd.Flags().GetBool("debug")
	cobra.CheckErr(err)

	current, err := os.Getwd()
	cobra.CheckErr(err)

	cfg := config.Config{
		ApiKey: key,
		Lang:   viper.GetString("lang"),
	}
	cli := &cli{
		repo:   &git.Repository{Path: current},
		client: &openai.Client{Config: &openai.Config{ApiKey: cfg.ApiKey}},
		cfg:    cfg,
		base:   base,
		tgt:    tgt,
		debug:  debug,
	}
	err = cli.run()
	cobra.CheckErr(err)
}

type cli struct {
	repo   *git.Repository
	client *openai.Client
	cfg    config.Config
	base   string
	tgt    string
	debug  bool
}

func (c *cli) run() error {
	if !c.repo.IsGitRepository() {
		return fmt.Errorf("not a git repository")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	work := filepath.Join(home, config.WorkDir, "tmp", filepath.Base(c.repo.Path))
	err = os.RemoveAll(work)
	if err != nil {
		return err
	}

	commitLogDir := filepath.Join(work, "commits")
	err = os.MkdirAll(commitLogDir, 0700)
	if err != nil {
		return err
	}
	outdir := filepath.Join(work, "out")
	err = os.MkdirAll(outdir, 0700)
	if err != nil {
		return err
	}

	err = c.saveCommits(commitLogDir)
	if err != nil {
		return err
	}
	err = c.askOpenai(commitLogDir, outdir)
	if err != nil {
		return err
	}

	return nil
}

func (c *cli) saveCommits(outdir string) error {
	commits, err := c.repo.CommitsOnBranch(c.tgt, c.base)
	if err != nil {
		return err
	}
	num := len(commits)
	fmt.Println("Number of commits:", num)
	for i, commit := range commits {
		filename := filepath.Join(outdir, fmt.Sprintf("%03d.txt", num-i))
		file, err := os.Create(filename)
		if err != nil {
			return err
		}
		defer file.Close()

		// _, err = file.WriteString(fmt.Sprintf("## Hash: %s\n", commit.Hash))
		// if err != nil {
		// 	return err
		// }
		// _, err = file.WriteString(fmt.Sprintf("Author: %s\n", commit.Author))
		// if err != nil {
		// 	fmt.Println("Failed to write commit author:", err)
		// 	return
		// }
		// _, err = file.WriteString(fmt.Sprintf("## Date: %s\n", commit.Date))
		// if err != nil {
		// 	return err
		// }
		_, err = file.WriteString(fmt.Sprintf("### Message: %s\n", strings.TrimSpace(commit.Message)))
		if err != nil {
			return err
		}

		_, err = file.WriteString("### Diffs:\n")
		if err != nil {
			return err
		}
		for _, diff := range commit.Diffs {
			_, err = file.WriteString(fmt.Sprintf("#### File: %s\n", diff.Path))
			if err != nil {
				return err
			}
			// _, err = file.WriteString(fmt.Sprintf("IndexBefore: %s\n", diff.IndexBefore))
			// if err != nil {
			// 	fmt.Println("Failed to write diff index before:", err)
			// 	return
			// }
			// _, err = file.WriteString(fmt.Sprintf("IndexAfter: %s\n", diff.IndexAfter))
			// if err != nil {
			// 	fmt.Println("Failed to write diff index after:", err)
			// 	return
			// }
			_, err = file.WriteString(fmt.Sprintf("```\n%s```\n\n", diff.DiffContents))
			if err != nil {
				return err
			}
		}

		file.Sync()
		file.Close()
	}
	return nil
}

func (c *cli) askOpenai(logd, outd string) error {
	fmt.Println("Begin to ask OpenAI...")

	system := &openai.Message{
		Role:    "system",
		Content: "You are an expert project manager. Your mission is to make a report on the changes made in the git repository for the client.",
	}

	outfile, err := os.Create(filepath.Join(outd, "out.txt"))
	if err != nil {
		return err
	}
	defer outfile.Close()

	files, err := os.ReadDir(logd)
	if err != nil {
		return err
	}
	prompt, completion := 0, 0
	for i, file := range files {
		if c.debug {
			fmt.Println("File:", file.Name())
		}

		diff, err := c.readCommitLog(filepath.Join(logd, file.Name()))
		if err != nil {
			return err
		}

		messages := []*openai.Message{
			system,
			{
				Role:    "user",
				Content: fmt.Sprintf(instruction, c.cfg.FullLang(), diff),
			},
		}
		if c.debug {
			fmt.Println("Request:", messages[1].Content)
		}
		res, err := c.client.Chat(messages)
		if err != nil {
			return err
		}
		if c.debug {
			fmt.Println("\nResponse:\n", res.Choices[0].Message.Content)
			fmt.Println("Prompt tokens:", res.Usage.PromptTokens)
			fmt.Println("Completion tokens:", res.Usage.CompletionTokens)
			fmt.Println("Total tokens:", res.Usage.TotalTokens)
		}
		prompt += res.Usage.PromptTokens
		completion += res.Usage.CompletionTokens

		_, err = outfile.WriteString(fmt.Sprintf("## Change %d\n%s\n", i+1, res.Choices[0].Message.Content))
		if err != nil {
			return err
		}

		outfile.Sync()

		fmt.Print(".")
	}

	fmt.Printf("\nDONE!\nToken usage: %d (prompt: %d, completion: %d)\n", prompt+completion, prompt, completion)

	o := fmt.Sprintf("%s_%s.txt", strings.ReplaceAll(c.tgt, "/", "__"), time.Now().Format("20060102150405"))
	err = os.Rename(outfile.Name(), o)
	if err != nil {
		return err
	}

	fmt.Println("\nOutput file:", o)

	return nil
}

func (c *cli) readCommitLog(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	buf := make([]byte, 0, 1024)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(buf, 2048*1024)

	var diff string
	for scanner.Scan() {
		diff += scanner.Text() + "\n"
	}

	return diff, nil
}
