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

const instruction = `
## Instruction:
Please summarize the file change briefly, using bullet points and word-for-word descriptions.
* Focus on the purpose of the change.
* Only the filename and changes are required.
* Preferred language is %s.

## Expected Output Format:
### Filename: file.ext (added/modified/deleted)
### Changes:
* Add feature X to screen A (if the screen name is not clear, assume it based on the file name)
* Change B setting from Y to Z
* Fix C bug

## File change to summarize:
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
	info := fmt.Sprintf("### Message: %s\n### Change list:\n", strings.TrimSpace(commit.Message))
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
		outd := filepath.Join(outdir, fmt.Sprintf("c-%03d", i+1))
		err = os.MkdirAll(outd, 0700)
		if err != nil {
			return err
		}

		info, bodies, err := c.commitText(commit)
		if err != nil {
			return err
		}
		if c.debug {
			fmt.Println("Commit:", i+1)
			fmt.Println(info)
		}

		infofile, err := os.Create(filepath.Join(outd, "info.txt"))
		if err != nil {
			return err
		}
		if commit.IsMerge {
			_, err = infofile.WriteString(fmt.Sprintf("[This is a merge commit]\n%s", info))
			if err != nil {
				return err
			}
			continue
		}

		_, err = infofile.WriteString(info)
		if err != nil {
			return err
		}

		numDiffs := len(commit.Diffs)
		sps := []*openai.Message{
			system, {
				Role:    "system",
				Content: fmt.Sprintf("Below is the overview of this entire commit. Take it into account as needed:\n%s\n", info),
			},
		}
		for j, body := range bodies {
			outfile, err := os.Create(filepath.Join(outd, fmt.Sprintf("%03d.txt", numDiffs-j)))
			if err != nil {
				return err
			}

			res, err := c.askOpenai(sps, body)
			if err != nil {
				return err
			}
			if c.debug {
				fmt.Println("\nResponse:\n", res.Choices[0].Message.Content)
				fmt.Println("Usages:\n", res.Usage)
			}
			prompt += res.Usage.PromptTokens
			completion += res.Usage.CompletionTokens

			_, err = outfile.WriteString(res.Choices[0].Message.Content)
			if err != nil {
				return err
			}
		}

		// TODO: Summary of the commit
	}

	// TODO: Summary of the branch (all commits)

	fmt.Printf("\nDONE!\nToken usage: %d (prompt: %d, completion: %d)\n", prompt+completion, prompt, completion)

	return nil
}

func (c *cli) askOpenai(system []*openai.Message, diff string) (*openai.ChatResponse, error) {
	messages := append(system, &openai.Message{
		Role:    "user",
		Content: fmt.Sprintf(instruction, c.cfg.FullLang(), diff),
	})

	if c.debug {
		fmt.Printf("Prompt:\n%s\n%s", messages[1].Content, messages[2].Content)
	}

	return c.client.Chat(messages)
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
