package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/hashicorp/mdns"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/sync/errgroup"
)

// sshCmd represents the ssh command
var sshCmd = &cobra.Command{
	Use:   "ssh",
	Short: "Connect to a VM over SSH",
	Long: `Connects the local console to the SSH daemon of a VM.
	
SSH settings must be specified via Cloud Config and the machine image must be
configured to start a SSH daemon.`,
	Example: "fog ssh lunar",
	Args:    cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		fmt.Printf("Connecting to %s...", name)

		entriesCh := make(chan *mdns.ServiceEntry)

		entryCh := make(chan *mdns.ServiceEntry, 1)

		go func() {
			for entry := range entriesCh {
				for _, f := range entry.InfoFields {
					if f == fmt.Sprintf("fog=%s", name) {
						entryCh <- entry

						return
					}
				}
			}
		}()

		mdns.Lookup("_ssh._tcp", entriesCh)

		// TODO: timeout
		entry := <-entryCh

		close(entriesCh)
		close(entryCh)

		log.Debug("Found MDNS entry", "entry", entry)

		port := entry.Port

		username := ""
		pw := ""
		for _, v := range entry.InfoFields {
			if strings.HasPrefix(v, "u=") {
				username = strings.TrimPrefix(v, "u=")
			}

			if strings.HasPrefix(v, "p=") {
				pw = strings.TrimPrefix(v, "p=")
			}
		}

		log.Debug("Dialing SSH agent...")

		sock, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))

		if err != nil {
			return fmt.Errorf("dialing SSH agent: %w", err)
		}

		ag := agent.NewClient(sock)

		auths := []ssh.AuthMethod{
			ssh.PublicKeysCallback(ag.Signers),
		}

		if pw != "" {
			auths = append(auths, ssh.Password(pw))
		}

		config := &ssh.ClientConfig{
			User: username,
			Auth: auths,
			// TODO: use phone home module to get these from cloud init
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}

		addr := fmt.Sprintf("127.0.0.1:%d", port)

		log.Debug("Dialing SSH daemon...", "port", port, "username", username)

		client, err := ssh.Dial("tcp", addr, config)
		if err != nil {
			return fmt.Errorf("dialing SSH agent: %w", err)
		}

		session, err := client.NewSession()

		if err != nil {
			return fmt.Errorf("creating SSH session: %w", err)
		}

		modes := ssh.TerminalModes{
			ssh.ECHO:          0,     // disable echoing
			ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
			ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
		}

		// TODO: get these from the parent term?
		if err := session.RequestPty("xterm", 80, 40, modes); err != nil {
			session.Close()
			return fmt.Errorf("requesting pty: %w", err)
		}

		ctx := cmd.Context()

		eg, ctx := errgroup.WithContext(ctx)

		defer session.Close()

		stdin, err := session.StdinPipe()

		if err != nil {
			return fmt.Errorf("opening stdin pipe: %w", err)
		}

		eg.Go(func() error {
			_, err := io.Copy(stdin, os.Stdin)

			return err
		})

		stdout, err := session.StdoutPipe()

		if err != nil {
			return fmt.Errorf("opening stdout pipe: %w", err)
		}

		eg.Go(func() error {
			_, err := io.Copy(os.Stdout, stdout)

			return err
		})

		stderr, err := session.StderrPipe()

		if err != nil {
			return fmt.Errorf("opening stderr pipe: %w", err)
		}

		eg.Go(func() error {
			_, err := io.Copy(os.Stderr, stderr)

			return err
		})

		sshCmd := "/bin/sh"

		if len(args) > 1 {
			sshCmd = strings.Join(args[1:], " ")
		}

		if err := session.Run(sshCmd); err != nil {
			return err
		}

		err = ctx.Err()

		if err == context.Canceled {
			return nil
		}

		return err
	},
}

func init() {
	rootCmd.AddCommand(sshCmd)
}
