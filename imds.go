package fog

import (
	"fmt"
	"net/http"

	"github.com/charmbracelet/log"
	"gopkg.in/yaml.v3"
)

type ImdsServer struct {
	mux *http.ServeMux
}

func NewImdsSever(machines []*Machine) *ImdsServer {
	mux := http.NewServeMux()

	for _, m := range machines {
		c := m.Conf.CloudConfig

		mux.HandleFunc(fmt.Sprintf("/%s/user-data", m.ID), func(w http.ResponseWriter, r *http.Request) {
			d, err := yaml.Marshal(&c)

			if err != nil {
				log.Error("Invalid cloud-config", "machine", m.Name, "error", err.Error())

				w.WriteHeader(500)
				w.Write([]byte(err.Error()))
				return
			}

			w.Header().Add("Content-Type", "text/yaml")
			w.Write([]byte("#cloud-config\n"))
			if c != nil {
				w.Write(d)
			}
		})

		mux.HandleFunc(fmt.Sprintf("/%s/meta-data", m.ID), func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(fmt.Sprintf("instance-id: fog/%s\n", m.Name)))
			w.Write([]byte(fmt.Sprintf("local-hostname: %s\n\n", m.Name)))
		})

		mux.HandleFunc(fmt.Sprintf("/%s/vendor-data", m.ID), func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Content-Type", "text/yaml")
			// TODO:
			// - inject ssh keys from agent automatically
			// - don't create new user, use default instead
			// - enable serial console in machine config since it can vary? Allow detaching the main console instead?
			w.Write([]byte(`#cloud-config
users:
- default
- name: fog
  sudo: ALL=(ALL) NOPASSWD:ALL
  lock_passwd: false
  shell: /bin/bash
  ssh_authorized_keys:
  - "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIPgo7auZwhc1Di7BlJA+tI2mFKe/w/mVIQlFPOKV59xV matt@destructure.co"

write_files:
- path: /etc/systemd/system/serial-getty@ttyS1.service.d/autologin.conf
  content: |
    [Service]
    ExecStart=
    ExecStart=-/sbin/agetty -o '-p -f -- \\u' --keep-baud --autologin fog 115200,57600,38400,9600 - $TERM

runcmd:
  - ["sudo", "systemctl", "enable", "--now", "serial-getty@ttyS1.service"]
`))
		})
	}

	return &ImdsServer{
		mux: mux,
	}
}

func (i *ImdsServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	i.mux.ServeHTTP(w, r)
}
