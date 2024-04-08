/*
Copyright © 2024 Koichi Kaneshige <coarse.ground@gmail.com>
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	system = &openai.Message{
		Role:    "system",
		Content: "You are an expert project manager. Your mission is to make a report on the changes made in the git repository for the client.",
	}
)

const (
	inst_d = `
	# Instruction:
	Please summarize the file change briefly, using bullet points and word-for-word descriptions.
	* Focus on the purpose of the change.
	* Only the filename and changes are required.
	* Preferred language is %s.

	# Expected Output Format:
	### file.ext (ADD/MOD/DEL)
		* Add feature X to screen A (if the screen name is not clear, assume it based on the file name)
		* Change B setting from Y to Z
		* Fix C bug

	# File change to summarize:
	%s
	`
	inst_c = `
	# Instruction:
	Please summarize the git commit briefly, using bullet points and word-for-word descriptions, such as release notes.
	* Focus on the purpose of the commit, ignore the file-level details.
	* Preferred language is %s.

	# Expected Output Format:
	### Summary
		* Add feature X to screen A (if the screen name is not clear, assume it based on the file name)
		* Change B setting from Y to Z
		* Fix C bug

	# Commit to summarize:
	%s
	`
)

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
	model := viper.GetString("openai-model")

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
		client: &openai.Client{ApiKey: cfg.ApiKey, Model: model},
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

	outdir := filepath.Join(work, "out")
	err = os.MkdirAll(outdir, 0700)
	if err != nil {
		return err
	}

	err = c.summarize(outdir)
	if err != nil {
		return err
	}

	return nil
}

func (c *cli) commitText(commit git.Commit) (string, []string, error) {
	info := fmt.Sprintf("## Message\n%s\n## All change list:\n", strings.TrimSpace(commit.Message))
	bodies := make([]string, 0, len(commit.Diffs))
	for _, diff := range commit.Diffs {
		dcs := make([]string, 0, len(diff.DiffContents))
		st := "MOD"
		var bytes int
		for _, dc := range diff.DiffContents {
			if strings.HasPrefix(dc, "new file mode ") {
				st = "ADD"
			} else if strings.HasPrefix(dc, "deleted file mode ") {
				st = "DEL"
			} else if strings.HasPrefix(dc, "Binary files ") || strings.HasSuffix(diff.Path, ".svg") {
				// skip binary files
			} else {
				// Limit the size of the diff contents to 40KB because of the token limit.
				if bytes+len(dc) > 40*1024 {
					break
				}
				dcs = append(dcs, strings.TrimSpace(dc))
				bytes += len(dc)
			}
		}
		b := fmt.Sprintf("### File: %s\n", diff.Path)
		if len(dcs) > 0 {
			b += "```\n" + strings.Join(dcs, "\n") + "\n```\n"
		}
		bodies = append(bodies, b)
		info += fmt.Sprintf("%s %s\n", st, diff.Path)
	}

	return info, bodies, nil
}

func (c *cli) summarize(outdir string) error {
	commits, err := c.repo.CommitsOnBranch(c.tgt, c.base)
	if err != nil {
		return err
	}
	num := len(commits)
	fmt.Println("Number of commits:", num)

	prompt, completion := 0, 0
	for i, commit := range commits {
		info, bodies, err := c.commitText(commit)
		if err != nil {
			return err
		}
		if c.debug {
			fmt.Printf("----- Commit -----\n---- num ---\n%d\n--- info ---\n%s", i+1, info)
		}

		if commit.IsMerge {
			file, err := os.Create(filepath.Join(outdir, fmt.Sprintf("c2-%03d", num-i)))
			if err != nil {
				return err
			}
			defer file.Close()

			_, err = file.WriteString(fmt.Sprintf("[This is a merge commit]\n%s", info))
			if err != nil {
				return err
			}

			continue
		}

		file, err := os.Create(filepath.Join(outdir, fmt.Sprintf("c1-%03d", num-i)))
		if err != nil {
			return err
		}
		defer file.Close()

		logs := fmt.Sprintf("%s\n## Change details:\n", info)
		sps := []*openai.Message{
			system, {
				Role:    "system",
				Content: fmt.Sprintf("Below is the overview of this entire commit. Take it into account as needed:\n%s", info),
			},
		}
		for _, body := range bodies {
			messages := append(sps, &openai.Message{
				Role:    "user",
				Content: fmt.Sprintf(inst_d, c.cfg.FullLang(), body),
			})
			if c.debug {
				fmt.Printf("\n----- Prompts -----\n")
				for _, m := range messages {
					fmt.Printf("--- %s --- \n%s\n", m.Role, m.Content)
				}
			}
			res, err := c.client.Chat(messages)
			if err != nil {
				return err
			}

			if c.debug {
				fmt.Println("\n----- Response -----\n", res.Choices[0].Message.Content)
				fmt.Printf("\n----- Usages -----\ntotal: %d (prompt: %d, completion: %d)", res.Usage.TotalTokens, res.Usage.PromptTokens, res.Usage.CompletionTokens)
			}
			prompt += res.Usage.PromptTokens
			completion += res.Usage.CompletionTokens

			logs += res.Choices[0].Message.Content + "\n"
		}

		_, err = file.WriteString(logs)
		if err != nil {
			return err
		}

		err = c.sumCommit(logs, outdir, num-i)
		if err != nil {
			return err
		}
	}

	// TODO: Summary of the branch (all commits)

	if c.debug {
		fmt.Printf("\n----- Total token usage ----- \n%d (prompt: %d, completion: %d)\n", prompt+completion, prompt, completion)
	}

	return nil
}

func (c *cli) sumCommit(logs, dir string, fnum int) error {
	csum := []*openai.Message{
		system, {
			Role:    "user",
			Content: fmt.Sprintf(inst_c, c.cfg.FullLang(), logs),
		},
	}

	res, err := c.client.Chat(csum)
	if err != nil {
		return err
	}

	file, err := os.Create(filepath.Join(dir, fmt.Sprintf("c2-%03d", fnum)))
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(res.Choices[0].Message.Content + "\n")
	if err != nil {
		return err
	}
	if c.debug {
		fmt.Printf("\n----- Saved file ----- \n%s\n", filepath.Join(dir, fmt.Sprintf("c2-%03d", fnum)))
	}

	return nil
}

// func (c *cli) read(path string) (string, error) {
// 	file, err := os.Open(path)
// 	if err != nil {
// 		return "", err
// 	}
// 	defer file.Close()

// 	scanner := bufio.NewScanner(file)
// 	buf := make([]byte, 10000)
// 	scanner.Buffer(buf, 1000000)
// 	var lines []string
// 	var totalBytes int
// 	for scanner.Scan() {
// 		totalBytes += len(scanner.Bytes())
// 		if totalBytes > 40*1024 {
// 			break
// 		}
// 		lines = append(lines, scanner.Text())
// 	}
// 	if err := scanner.Err(); err != nil {
// 		return "", err
// 	}

// 	return strings.Join(lines, "\n"), nil
// }
