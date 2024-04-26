/*
Copyright © 2024 Koichi Kaneshige <coarse.ground@gmail.com>
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tetran/lgh/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configure the application",
	Long:  `Configure the application by setting values required for the application to run.`,
	Run:   configure,
}

func init() {
	configCmd.Flags().StringP("openai-api-key", "", "", "OpenAI API key")
	configCmd.Flags().StringP("claude-api-key", "", "", "Claude API key")
	configCmd.Flags().StringP("lang", "", "", "Output language")
}

func configure(cmd *cobra.Command, args []string) {
	okey, err := cmd.Flags().GetString("openai-api-key")
	cobra.CheckErr(err)

	ckey, err := cmd.Flags().GetString("claude-api-key")
	cobra.CheckErr(err)

	lng, err := cmd.Flags().GetString("lang")
	cobra.CheckErr(err)

	if okey == "" && ckey == "" && lng == "" {
		okey = read("openai-api-key", "OpenAI API key: ", "Please enter the OpenAI API key and the output language.", "")
		ckey = read("claude-api-key", "Claude API key: ", "Please enter the Claude API key.", "")
		lng = read("lang", "Output language (available: [en/ja], default: en): ", "", "en")
	}

	file := filepath.Join(os.Getenv("HOME"), config.WorkDir, "config.yaml")
	err = os.MkdirAll(filepath.Dir(file), 0700)
	cobra.CheckErr(err)

	if _, err := os.Stat(file); err == nil {
		err = os.Remove(file)
		cobra.CheckErr(err)
	}

	f, err := os.Create(file)
	cobra.CheckErr(err)
	defer f.Close()

	_, err = f.WriteString(fmt.Sprintf("openai-api-key: %s\nclaude-api-key: %s\nlang: %s\n", okey, ckey, lng))
	cobra.CheckErr(err)
}

func read(name, label, desc, defaultV string) string {
	if desc != "" {
		fmt.Println(desc)
	}
	fmt.Print(label)

	var val string
	fmt.Scanln(&val)
	if val == "" {
		val = viper.GetString(name)
		if val == "" && defaultV != "" {
			val = defaultV
		}
	}
	return val
}
