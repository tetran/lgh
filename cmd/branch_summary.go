	system = &openai.Message{
		Role:    "system",
		Content: "Act as an expert project manager. Your mission is to make a report on the changes made in the git repository for the client.",
	}
const (
	inst_d = `
	# Instruction:
	Please summarize the file change briefly, using bullet points and word-for-word descriptions.
	* Focus on the purpose of the change.
	* Just return the change of the following file.
	* Only the filename and brief changes are required.
	* Preferred language is %s.

	# Expected Output Format:
	### file.ext (ADD/MOD/DEL)
	* Add feature X
	* Change B setting
	* Fix C bug

	# File change to summarize:
	%s
	`
	inst_c = `
	# Instruction:
	Please summarize the git commit briefly, using bullet points and word-for-word descriptions, like release notes.
	* Focus on the purpose of the commit, ignore the file-level details.
	* Preferred language is %s.

	# Expected Output Format:
	* Add feature X to screen A (if the screen name is not clear, assume it based on the file name)
	* Change B setting from Y to Z
	* Fix C bug

	# Commit to summarize:
	%s
	`
	inst_b = `
	# Instruction:
	Please summarize the changes briefly, using bullet points and word-for-word descriptions, like release notes.
	* If there are any duplicate or similar commits, combine them, the first one should be the main source.
	* Combine related items in one section.
	* Preferred language is %s.

	# Expected Output Format:
	## Implement feature X
	* details of the feature and the implementation
	## Fix C bug
	* details of the bug and the fix

	# Changes to summarize:
	%s
	`
)
	err = c.summarize(outdir)
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
	prompt, completion := 0, 0
	if c.debug {
		defer func() {
			fmt.Printf("\n----- Total token usage ----- \n%d (prompt: %d, completion: %d)\n", prompt+completion, prompt, completion)
		}()
	}
	var allSummaries string
	for i, commit := range commits {
		if commit.IsMerge {
			file, err := os.Create(filepath.Join(outdir, fmt.Sprintf("CS%05d", num-i)))
			defer file.Close()

			_, err = file.WriteString(fmt.Sprintf("* Merged: %s", commit.Message))
			continue
		info, bodies, err := c.commitText(commit)
		if err != nil {
			return err
		file, err := os.Create(filepath.Join(outdir, fmt.Sprintf("CL%05d", num-i)))
		defer file.Close()
		logs := fmt.Sprintf("%s\n## Change details:\n", info)
		sps := []*openai.Message{
			system, {
				Role:    "system",
				Content: fmt.Sprintf("Below is the overview of this entire commit. Take it into account as needed:\n%s", info),
		for _, body := range bodies {
			messages := append(sps, &openai.Message{
				Role:    "user",
				Content: fmt.Sprintf(inst_d, c.cfg.FullLang(), body),
			})
			if c.debug {
				c.pp(messages)
			}
			res, err := c.client.Chat(messages)
			if err != nil {
				return err
			}

			if c.debug {
				fmt.Printf("\n----- Response -----\n%s\n", res.Choices[0].Message.Content)
				fmt.Printf("\n----- Usages -----\ntotal: %d (prompt: %d, completion: %d)\n", res.Usage.TotalTokens, res.Usage.PromptTokens, res.Usage.CompletionTokens)
			}
			prompt += res.Usage.PromptTokens
			completion += res.Usage.CompletionTokens

			logs += res.Choices[0].Message.Content + "\n"

		_, err = file.WriteString(logs)
		sum, err := c.sumCommit(logs, outdir, num-i)
		allSummaries += sum
	messages := []*openai.Message{
		system, {
			Role:    "user",
			Content: fmt.Sprintf(inst_b, c.cfg.FullLang(), allSummaries),
		},
	}
	if c.debug {
		c.pp(messages)
	}
	res, err := c.client.Chat(messages)
	if err != nil {
		return err
	}
	if c.debug {
		fmt.Printf("\n----- Response -----\n%s\n", res.Choices[0].Message.Content)
		fmt.Printf("\n----- Usages -----\ntotal: %d (prompt: %d, completion: %d)\n", res.Usage.TotalTokens, res.Usage.PromptTokens, res.Usage.CompletionTokens)
	}
	path := filepath.Join(outdir, "summary.txt")
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(res.Choices[0].Message.Content + "\n")
	if c.debug {
		fmt.Printf("\n----- Saved file ----- \n%s\n", path)
	}
	prompt += res.Usage.PromptTokens
	completion += res.Usage.CompletionTokens
func (c *cli) sumCommit(logs, dir string, fnum int) (string, error) {
	messages := []*openai.Message{
		system, {
			Role:    "user",
			Content: fmt.Sprintf(inst_c, c.cfg.FullLang(), logs),
		},
	}
	if c.debug {
		c.pp(messages)
	}

	res, err := c.client.Chat(messages)
	path := filepath.Join(dir, fmt.Sprintf("CS%05d", fnum))
	file, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	content := res.Choices[0].Message.Content + "\n"
	_, err = file.WriteString(content)
	if err != nil {
		return "", err
	}
	if c.debug {
		fmt.Printf("\n----- Saved file ----- \n%s\n", path)
	return content, nil

func (c *cli) pp(messages []*openai.Message) {
	fmt.Printf("\n----- Prompts -----\n")
	for _, m := range messages {
		fmt.Printf("--- %s --- \n%s\n", m.Role, m.Content)
	}
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