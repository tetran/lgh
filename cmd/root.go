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

// rootCmd represents the base command when called without any subcommands
var (
	cfgFile string

	rootCmd = &cobra.Command{
		Use:   "lgh",
		Short: "lgh is a tool to help you understand a git repository better.",
		Long:  ``,
	}
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", fmt.Sprintf("config file (default is $HOME/%s/config.yaml)", config.WorkDir))
	rootCmd.PersistentFlags().String("openai-api-key", "", "OpenAI API key")
	rootCmd.PersistentFlags().String("lang", "en", "Output language")

	viper.BindPFlag("openai-api-key", rootCmd.PersistentFlags().Lookup("openai-api-key"))
	viper.BindPFlag("lang", rootCmd.PersistentFlags().Lookup("lang"))

	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(bsCmd)
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.AddConfigPath(filepath.Join(home, config.WorkDir))
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
