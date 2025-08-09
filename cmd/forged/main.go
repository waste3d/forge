package main

import (
	"flag"
	"log"
	"log/slog"
	"os"

	"github.com/waste3d/forge/internal/constants"
	"github.com/waste3d/forge/internal/server"
)

func main() {
	addrFlag := flag.String("addr", "", "Address for the daemon to listen on. Overrides FORGE_DAEMON_ADDR.")
	flag.Parse()

	listenAddr := *addrFlag
	if listenAddr == "" {
		listenAddr = os.Getenv(constants.DaemonAddrEnvVar)
		if listenAddr == "" {
			listenAddr = constants.DefaultDemonAddress
		}
	}

	log.SetFlags(0)

	if err := server.InitializeServer(listenAddr); err != nil {
		slog.Error("ошибка инициализации сервера", "error", err)
		os.Exit(1)
	}
}
