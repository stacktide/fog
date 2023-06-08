package fog

import (
	"fmt"
	"net/http"

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
			w.Write([]byte(""))
		})
	}

	return &ImdsServer{
		mux: mux,
	}
}

func (i *ImdsServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	i.mux.ServeHTTP(w, r)
}
