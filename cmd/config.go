/*
Copyright © 2024 Koichi Kaneshige <coarse.ground@gmail.com>
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
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
	configCmd.Flags().StringP("openai-model", "", "", "OpenAI model")
	configCmd.Flags().StringP("lang", "", "", "Output language")
}

func configure(cmd *cobra.Command, args []string) {
	key, err := cmd.Flags().GetString("openai-api-key")
	cobra.CheckErr(err)

	model, err := cmd.Flags().GetString("openai-model")
	cobra.CheckErr(err)

	lng, err := cmd.Flags().GetString("lang")
	cobra.CheckErr(err)

	if key == "" && lng == "" && model == "" {
		fmt.Println("Please enter the OpenAI API key and the output language.")
		fmt.Print("OpenAI API key: ")
		fmt.Scanln(&key)
		fmt.Print("Please enter the OpenAI model (default: gpt-3.5-turbo): ")
		fmt.Scanln(&model)
		if model == "" {
			model = "gpt-3.5-turbo"
		}
		fmt.Print("Output language (available: [en/ja], default: en): ")
		fmt.Scanln(&lng)
		if lng == "" {
			lng = "en"
		}
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

	_, err = f.WriteString(fmt.Sprintf("openai-api-key: %s\nlang: %s\nopenai-model: %s\n", key, lng, model))
	cobra.CheckErr(err)
}
