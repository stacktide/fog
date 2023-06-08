package main

import (
	"github.com/adrg/xdg"
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "fog",
	Short: "Generate small local clouds of virtual machines",
	Long: `Fog is a CLI tool for generating small local clouds of virtual machines.

Fog uses QEMU under the hood to create and manage VMs and provisions instances with Cloud Init.
`,
	SilenceUsage: true, // this prevents the usage from being shown when Command.RunE returns an error
}

var globalConfig = viper.New()
var projectConfig = viper.New()

func init() {
	cobra.OnInitialize(initGlobalConfig)
	cobra.OnInitialize(initProjectConfig)
}

// initGlobalConfig reads in config file and ENV variables if set for global configuration settings.
func initGlobalConfig() {
	configFilePath, err := xdg.ConfigFile("fog/config.yaml")

	cobra.CheckErr(err)

	globalConfig.SetConfigFile(configFilePath)

	globalConfig.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := globalConfig.ReadInConfig(); err == nil {
		log.Debug("Loaded global config", "file", globalConfig.ConfigFileUsed())
	}
}

// initProjectConfig reads in config file and ENV variables if set for per project configuration settings.
func initProjectConfig() {
	projectConfig.SetConfigName("fog")
	projectConfig.SetConfigType("yaml")
	projectConfig.AddConfigPath(".")
	projectConfig.AddConfigPath(".fog")

	projectConfig.AutomaticEnv() // read in environment variables that match

	if err := projectConfig.ReadInConfig(); err == nil {
		log.Debug("Loaded project config", "file", projectConfig.ConfigFileUsed())
	}
}
