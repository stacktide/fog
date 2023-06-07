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
	"sync"
	"time"

	"github.com/adrg/xdg"
)

// Machine is a virtual machine managed by fog.
type Machine struct {
	id      string
	name    string
	conf    *MachineConfig
	img     *Image
	imgPath string
	addr    string
	connMu  sync.Mutex
	conn    net.Conn
}

func NewMachine(name string, conf *MachineConfig, img *Image, imgPath string) *Machine {
	id := generateMachineID()

	return &Machine{
		id:      id,
		name:    name,
		conf:    conf,
		img:     img,
		imgPath: imgPath,
	}
}

// Start boots the virtual machine
func (m *Machine) Start(ctx context.Context) error {
	bin, err := exec.LookPath("qemu-system-x86_64")

	if err != nil {
		return fmt.Errorf("finding qemu binary: %w", err)
	}

	addr, err := xdg.RuntimeFile("fog/" + m.id + ".sock")

	if err != nil {
		return fmt.Errorf("generating socket file path: %w", err)
	}

	m.addr = addr

	log.Printf("Using socket: %s\n", addr)

	args := []string{
		"-net",
		"nic",
		"-net",
		"user",
		"-machine",
		"accel=kvm:tcg",
		"-cpu",
		"host",
		"-m",
		"512",
		"-nographic",
		"-hda",
		m.imgPath,
		"-snapshot",
		"-chardev",
		"socket,id=serial0,path=" + addr + ",server,nowait",
		"-serial",
		"chardev:serial0",
		"-smbios",
		"type=1,serial=ds=nocloud-net;s=http://10.0.2.2:8090/",
	}

	fmt.Printf("Starting %s...\n", m.name)

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
