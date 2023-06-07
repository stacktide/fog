package fog

import (
	"fmt"
	"net/http"
)

type ImdsServer struct {
	mux *http.ServeMux
}

func NewImdsSever() *ImdsServer {
	mux := http.NewServeMux()

	// TODO: dynamic based on machine ID

	// TODO: pass in
	mux.HandleFunc("/user-data", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("User data requested")

		w.Write([]byte(`#cloud-config
password: password
chpasswd:
  expire: False
  
  `))
	})

	// TODO: pass in
	mux.HandleFunc("/meta-data", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`instance-id: someid/somehostname
local-hostname: jammy

`))
	})

	mux.HandleFunc("/vendor-data", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(""))
	})

	return &ImdsServer{
		mux: mux,
	}
}

func (i *ImdsServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	i.mux.ServeHTTP(w, r)
}
