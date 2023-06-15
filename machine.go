package fog

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/adrg/xdg"
	"github.com/charmbracelet/log"
)

// Machine is a virtual machine managed by fog.
type Machine struct {
	ID      string
	Name    string
	Conf    *MachineConfig
	Img     *Image
	ImgPath string
	addr    string
	qmpAddr string
	connMu  sync.Mutex
	conn    net.Conn
	cmd     *exec.Cmd
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
	output   io.Writer
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

	qmpAddr, err := xdg.RuntimeFile("fog/" + m.ID + "_qmp.sock")

	if err != nil {
		return fmt.Errorf("generating monitor socket file path: %w", err)
	}

	m.qmpAddr = qmpAddr

	dsUrl := fmt.Sprintf("http://10.0.2.2:%d/%s/", opts.imdsPort, m.ID)

	fwds := ""

	if len(m.Conf.Ports) > 0 {
		fwds = fmt.Sprintf(",hostfwd=%s", strings.Join(m.Conf.Ports, ","))
	}

	args := []string{
		// Machine settings
		"-machine",
		// TODO: only enable KVM when supported
		"accel=kvm:tcg",
		// System resources
		"-cpu",
		"host",
		"-m",
		m.Conf.Memory,
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
		// Stdio
		"-chardev",
		"socket,id=serdev,path=" + addr + ",server=on,wait=off",
		"-serial",
		"chardev:serdev",
		// QMP
		"-chardev",
		"socket,id=qmpdev,path=" + qmpAddr + ",server=on,wait=off",
		"-mon",
		"qmpdev",
		// Cloud init
		"-smbios",
		"type=1,serial=ds=nocloud-net;s=" + dsUrl,
	}

	log.Debug("Starting machine", "name", m.Name, "sock", addr, "mon", qmpAddr)

	cmd := exec.Command(bin, args...)

	m.cmd = cmd

	if out := opts.output; out != nil {
		cmd.Stdout = out
		cmd.Stderr = out
	}

	err = cmd.Start()

	if err != nil {
		return fmt.Errorf("executing QEMU command: %w", err)
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

		time.Sleep(time.Duration(i) * time.Second)
	}

	return nil, errors.New("failed to open connection")
}

// Conn returns a connection to the machine's primary socket.
func (m *Machine) Conn() (net.Conn, error) {
	conn, err := m.openConn()

	if err != nil {
		return nil, err
	}

	return conn, nil
}

// generateMachineID generates a random machine ID.
func generateMachineID() string {
	b := make([]byte, 32)
	r := rand.Reader

	if _, err := io.ReadFull(r, b); err != nil {
		panic(err) // This shouldn't happen
	}

	return hex.EncodeToString(b)
}

// findFreeTcpPort finds a free TCP port.
func findFreeTcpPort() (int, error) {
	var port int

	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")

	if err != nil {
		return port, fmt.Errorf("resolving tcp address: %w", err)
	}

	l, err := net.ListenTCP("tcp", addr)

	if err != nil {
		return port, fmt.Errorf("listening to TCP address: %w", err)
	}

	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
