package main

import (
	"github.com/spf13/cobra"
	"go.destructure.co/fog"
)

// pullCmd represents the pull command
var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull an image from a remote source",
	Long: `Pulls an image from a remote source and stores it locally.
	
An image is pulled by name and tag. If the tag is not specified the "latest" tag is pulled.`,
	Example: "fog pull ubuntu:latest",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {

		ctx := cmd.Context()

		r := fog.NewImageRepository()

		rawImg := args[0]

		err := r.LoadManifests()

		if err != nil {
			return err
		}

		img, err := r.Find(ctx, rawImg)

		if err != nil {
			return err
		}

		err = r.Pull(ctx, img, fog.ImagePullOptions{})

		if err != nil {
			return err
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(pullCmd)
}
