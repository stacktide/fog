package main

import (
	"fmt"
	"os"

	"github.com/adrg/xdg"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "fog",
	Short: "Generate small local clouds of virtual machines",
	Long: `Fog is a CLI tool for generating small local clouds of virtual machines.

Fog uses QEMU under the hood to create and manage VMs and provisions instances with Cloud Init.
`,
	SilenceUsage: true,
}

func init() {
	cobra.OnInitialize(initConfig)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	configFilePath, err := xdg.ConfigFile("fog/config.yaml")

	cobra.CheckErr(err)

	viper.SetConfigFile(configFilePath)

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		// TODO: use logger instead
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
