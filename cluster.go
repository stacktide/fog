package fog

import (
	"bufio"
	"context"
	"fmt"
	"net/http"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/sync/errgroup"
)

// Cluster is a cluster of virtual machines.
type Cluster struct {
	conf     *Config
	r        *ImageRepository
	machines []*Machine
}

func NewCluster(conf *Config, r *ImageRepository) *Cluster {
	return &Cluster{
		conf: conf,
		r:    r,
	}
}

func (c *Cluster) Init(ctx context.Context) error {
	err := c.r.LoadManifests()

	if err != nil {
		return err
	}

	eg, ctx := errgroup.WithContext(ctx)

	for n, m := range c.conf.Machines {
		n := n
		m := m

		eg.Go(func() error {
			img, err := c.r.Find(ctx, m.Image)

			if err != nil {
				return err
			}

			p := c.r.ImagePath(img)

			c.machines = append(c.machines, NewMachine(n, m, img, p))

			return c.r.Pull(ctx, img, ImagePullOptions{})
		})
	}

	if err = eg.Wait(); err != nil {
		return err
	}

	// TODO: Do we create a control socket here?

	return nil
}

func (c *Cluster) Start(ctx context.Context) error {
	eg, ctx := errgroup.WithContext(ctx)

	imds := NewImdsSever()

	// Run the IMDS server in the background
	eg.Go(func() error {
		// TODO: bind to localhost only if possible with QEMU networking
		return http.ListenAndServe("127.0.0.1:8090", imds)
	})

	for _, m := range c.machines {
		m := m

		eg.Go(func() error {
			err := m.Start(ctx)

			if err != nil {
				return err
			}

			c, err := m.Conn()

			if err != nil {
				return err
			}

			scanner := bufio.NewScanner(c)

			// TODO: pad to max name length, use different colors per machine, etc.
			nameStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#04B575")).
				BorderStyle(lipgloss.NormalBorder()).
				PaddingRight(4).
				BorderForeground(lipgloss.Color("#3C3C3C")).
				BorderRight(true)

			// TODO: try and buffer lines for a few ms to reduce interleaving
			for scanner.Scan() {
				fmt.Printf("%s %s\n", nameStyle.Render(m.name), scanner.Text())
			}

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	return nil
}
