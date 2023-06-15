package fog

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"

	"github.com/charmbracelet/log"
	"github.com/hashicorp/mdns"
	"golang.org/x/sync/errgroup"
)

// Cluster is a cluster of virtual machines.
type Cluster struct {
	conf     *Config
	r        *ImageRepository
	machines []*Machine
	imdsSrv  *http.Server
	mdnsSrvs []*mdns.Server
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

	portChan := make(chan int)

	eg.Go(func() error {
		return c.startImdsServer(portChan)
	})

	port := <-portChan

	log.Debug("Started IMDS server", "port", port)

	mux := NewLogMux(ctx, os.Stderr)

	log.Debug("Opened mux logger")

	out := mux.Stream("qemu")

	opts := &StartOptions{
		imdsPort: port,
		output:   out,
	}

	for _, m := range c.machines {
		m := m

		err := m.Start(ctx, opts)

		if err != nil {
			return fmt.Errorf("starting machine %s: %w", m.Name, err)
		}

		eg.Go(func() error {
			return m.cmd.Wait()
		})
	}

	for _, m := range c.machines {
		m := m

		s := mux.Stream(m.Name)

		log.Debug("Created stream", "name", m.Name)

		eg.Go(func() error {
			c, err := m.Conn()

			log.Debug("Opened machine connection", "name", m.Name)

			if err != nil {
				return fmt.Errorf("getting machine socket connection: %w", err)
			}

			io.Copy(s, c)

			return nil
		})
	}

	err := c.startMdnsServers()

	if err != nil {
		return fmt.Errorf("starting Mdns server: %w", err)
	}

	log.Debug("Started MDNS server")

	<-ctx.Done()

	return ctx.Err()
}

func (c *Cluster) Shutdown(ctx context.Context) (err error) {
	err = c.imdsSrv.Shutdown(ctx)

	if err != nil {
		return fmt.Errorf("shutting down IMDS server: %w", err)
	}

	for _, s := range c.mdnsSrvs {
		// preserve the first error
		if err == nil {
			err = s.Shutdown()
		} else {
			s.Shutdown()
		}
	}

	return err
}

func (c *Cluster) startImdsServer(portChan chan<- int) error {
	imds := NewImdsSever(c.machines)

	l, err := net.Listen("tcp", "127.0.0.1:0")

	if err != nil {
		return fmt.Errorf("opening IMDS server TCP connection: %w", err)
	}

	port := l.Addr().(*net.TCPAddr).Port

	portChan <- port

	srv := &http.Server{Handler: imds}

	c.imdsSrv = srv

	return srv.Serve(l)
}

func (c *Cluster) startMdnsServers() error {
	for _, m := range c.machines {
		// TODO: loop through ports to find services and port bindings instead of hardcoding SSH
		// The format is: [tcp|udp]:[hostaddr]:hostport-[guestaddr]:guestport

		info := []string{fmt.Sprintf("Fog machine SSH for %s", m.Name)}

		svc, err := mdns.NewMDNSService(m.Name, "_ssh._tcp", "", "", 2222, nil, info)

		if err != nil {
			return fmt.Errorf("creating mdns service: %w", err)
		}

		server, err := mdns.NewServer(&mdns.Config{Zone: svc})

		if err != nil {
			return fmt.Errorf("creating mdns server: %w", err)
		}

		c.mdnsSrvs = append(c.mdnsSrvs, server)
	}

	return nil
}
