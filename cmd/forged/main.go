package main

import (
	"log/slog"

	"github.com/waste3d/forge/internal/server"
)

func main() {
	err := server.InitializeServer()
	if err != nil {
		slog.Error("ошибка инициализации сервера", "error", err)
	}
}
