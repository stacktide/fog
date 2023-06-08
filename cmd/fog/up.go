package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.destructure.co/fog"
)

// upCmd represents the up command
var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Boot virtual machines",
	Long: `Boots one or more virtual machines according to the fog.yaml specification and any provided arguments.
	
If a required base image does not exist locally it will be pulled automatically.`,
	Example: "fog up",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		var err error

		conf := &fog.Config{}

		projectConfig.Unmarshal(conf)

		for n, m := range conf.Machines {
			m.CloudConfig = projectConfig.GetStringMap(fmt.Sprintf("machines.%s.cloud_config", n))
		}

		r := fog.NewImageRepository()

		ctx := cmd.Context()

		c := fog.NewCluster(conf, r)

		err = c.Init(ctx)

		if err != nil {
			return err
		}

		err = c.Start(ctx)

		if err != nil {
			return err
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(upCmd)
}
