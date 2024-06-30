package main

import (
	"log/slog"

	"github.com/ivanvanderbyl/escea-fireplace/pkg/firecontrol"
)

func main() {
	fs, err := firecontrol.SearchForFireplaces()
	if err != nil {
		slog.Error("Error searching for fireplaces", "error", err)
		return
	}

	for _, f := range fs {
		slog.Info("Found Fireplace", "IP", f.Addr, "Serial", f.Serial, "PIN", f.PIN)

		err := f.Refresh()
		if err != nil {
			slog.Error("Failed to refresh fireplace", "error", err)
			continue
		}

		slog.Info("Fireplace Status", "Status", f.Status)
	}
}
