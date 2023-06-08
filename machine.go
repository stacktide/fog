package fog

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/adrg/xdg"
)

// Machine is a virtual machine managed by fog.
type Machine struct {
	ID      string
	Name    string
	Conf    *MachineConfig
	Img     *Image
	ImgPath string
	addr    string
	monAddr string
	connMu  sync.Mutex
	conn    net.Conn
}

func NewMachine(name string, conf *MachineConfig, img *Image, imgPath string) *Machine {
	id := generateMachineID()

	return &Machine{
		ID:      id,
		Name:    name,
		Conf:    conf,
		Img:     img,
		ImgPath: imgPath,
	}
}

type StartOptions struct {
	imdsPort int
}

// Start boots the virtual machine
func (m *Machine) Start(ctx context.Context, opts *StartOptions) error {
	bin, err := exec.LookPath("qemu-system-x86_64")

	if err != nil {
		return fmt.Errorf("finding qemu binary: %w", err)
	}

	addr, err := xdg.RuntimeFile("fog/" + m.ID + ".sock")

	if err != nil {
		return fmt.Errorf("generating socket file path: %w", err)
	}

	m.addr = addr

	log.Printf("Using socket: %s\n", addr)

	monAddr, err := xdg.RuntimeFile("fog/" + m.ID + "_monitor.sock")

	if err != nil {
		return fmt.Errorf("generating monitor socket file path: %w", err)
	}

	m.monAddr = monAddr

	log.Printf("Using monitor socket: %s\n", monAddr)

	dsUrl := fmt.Sprintf("http://10.0.2.2:%d/%s/", opts.imdsPort, m.ID)

	fwds := ""

	if len(m.Conf.Ports) > 0 {
		fwds = fmt.Sprintf(",hostfwd=%s", strings.Join(m.Conf.Ports, ","))
	}

	args := []string{
		// Machine settings
		"-machine",
		"accel=kvm:tcg",
		// System resources
		"-cpu",
		"host",
		"-m",
		"512",
		// Graphics
		"-nographic",
		"-vga",
		"none",
		// Boot image
		"-hda",
		m.ImgPath,
		"-snapshot",
		// Networking
		"-net",
		"nic",
		"-net",
		"user" + fwds,
		// Serial socket
		"-chardev",
		"socket,id=serial,path=" + addr + ",server,nowait",
		"-serial",
		"chardev:serial",
		// Monitor socket (only used for debugging ATM)
		"-chardev",
		"socket,id=monitor,path=" + monAddr + ",server,nowait",
		"-monitor",
		"chardev:monitor",
		// Cloud init
		"-smbios",
		"type=1,serial=ds=nocloud-net;s=" + dsUrl,
	}

	fmt.Printf("Starting %s...\n", m.Name)

	cmd := exec.Command(bin, args...)

	err = cmd.Start()

	if err != nil {
		return fmt.Errorf("starting machine: %w", err)
	}

	return nil
}

func (m *Machine) openConn() (net.Conn, error) {
	m.connMu.Lock()

	defer m.connMu.Unlock()

	if conn := m.conn; conn != nil {
		return conn, nil
	}

	// Retry in case QEMU has not finished booting yet
	for i := 0; i < 3; i++ {
		conn, err := net.Dial("unix", m.addr)

		if err == nil {
			m.conn = conn

			return conn, err
		}

		time.Sleep(time.Second)
	}

	return nil, errors.New("failed to open connection")
}

func (m *Machine) Conn() (net.Conn, error) {
	conn, err := m.openConn()

	if err != nil {
		return nil, err
	}

	return conn, nil
}

// generateMachineID generates a random machine ID using a similar format as Docker and Podman.
func generateMachineID() string {
	b := make([]byte, 32)
	r := rand.Reader

	if _, err := io.ReadFull(r, b); err != nil {
		panic(err) // This shouldn't happen
	}

	return hex.EncodeToString(b)
}
