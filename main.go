package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
)

// version, commit and date are set by goreleaser via -ldflags -X.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:8080", "address to listen on")
	socket := flag.String("incus-socket", "/var/lib/incus/unix.socket", "path to the local Incus unix socket")
	imageServer := flag.String("image-server", "https://images.linuxcontainers.org", "image server for container creation")
	groupPrefix := flag.String("tenant-group-prefix", "", "only groups with this prefix are tenants, and the project is the group name with the prefix stripped")
	versionFlag := flag.Bool("version", false, "Print version information and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("incus-lxc-provisioner version %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}

	srv := &Server{
		incus:       NewIncus(*socket),
		authz:       NewAuthorizer(*groupPrefix),
		imageServer: *imageServer,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealth)
	mux.HandleFunc("GET /containers", srv.handleList)
	mux.HandleFunc("POST /containers", srv.handleCreate)
	mux.HandleFunc("DELETE /containers/{name}", srv.handleDelete)
	mux.HandleFunc("POST /containers/{name}/start", srv.handleStart)
	mux.HandleFunc("POST /containers/{name}/stop", srv.handleStop)

	log.Printf("incus-lxc-provisioner listening on %s", *listen)
	if err := http.ListenAndServe(*listen, mux); err != nil {
		log.Fatal(err)
	}
}

// handleHealth reports only that the process is alive. It does not probe Incus,
// which is another tool's job, and needs no tenant group.
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
