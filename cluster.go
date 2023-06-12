package fog

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"

	"github.com/charmbracelet/log"
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

	return nil
}

func (c *Cluster) Start(ctx context.Context) error {
	eg, ctx := errgroup.WithContext(ctx)

	imds := NewImdsSever(c.machines)

	var port int

	portAssigned := make(chan bool, 1)

	// Run the IMDS server in the background on a random port
	eg.Go(func() error {
		l, err := net.Listen("tcp", "127.0.0.1:0")

		if err != nil {
			return fmt.Errorf("opening IMDS server TCP connection: %w", err)
		}

		port = l.Addr().(*net.TCPAddr).Port

		portAssigned <- true

		return http.Serve(l, imds)
	})

	<-portAssigned

	log.Debug("Started IMDS server", "port", port)

	opts := &StartOptions{
		imdsPort: port,
	}

	mux := NewLogMux(ctx, os.Stderr)

	log.Debug("Opened mux logger")

	for _, m := range c.machines {
		m := m

		s := mux.Stream(m.Name)

		log.Debug("Created stream", "name", m.Name)

		eg.Go(func() error {
			err := m.Start(ctx, opts)

			if err != nil {
				return err
			}

			c, err := m.Conn()

			log.Debug("Opened connection", "name", m.Name)

			if err != nil {
				return err
			}

			io.Copy(s, c)

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	return nil
}
