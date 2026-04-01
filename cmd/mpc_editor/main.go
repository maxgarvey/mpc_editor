package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	"github.com/maxgarvey/mpc_editor/internal/server"
	"github.com/maxgarvey/mpc_editor/web"
)

func main() {
	port := "8080"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	templateFS, staticFS := web.FS()
	srv := server.New(templateFS, staticFS)

	addr := "127.0.0.1:" + port
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}

	url := "http://" + addr
	fmt.Printf("MPC Editor running at %s\n", url)

	if runtime.GOOS == "darwin" {
		go exec.Command("open", url).Start()
	}

	log.Fatal(http.Serve(ln, srv.Handler()))
}
