package main

import (
	"log/slog"

	"github.com/ivanvanderbyl/escea-fireplace/pkg/firecontrol"
)

func main() {
	fs, err := firecontrol.SearchForFireplaces()
	if err != nil {
		panic(err)
	}

	for _, f := range fs {
		slog.Info("Found Fireplace", "IP", f.Addr, "Serial", f.Serial, "PIN", f.PIN)
	}
}
