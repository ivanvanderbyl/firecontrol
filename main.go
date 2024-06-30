package main

import (
	"log/slog"

	"github.com/brutella/hap"
	"github.com/brutella/hap/accessory"

	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	if err := realMain(); err != nil {
		log.Fatal(err)
	}
}

func realMain() error {

	// Create the fireplace accessory.
	// a := accessory.NewHeater(accessory.Info{
	// 	Name:         "Fireplace",
	// 	Manufacturer: "Escea",
	// 	Model:        "DF960",
	// 	SerialNumber: "123456",
	// 	Firmware:     "1.0.0",
	// })
	a := accessory.NewTemperatureSensor(accessory.Info{
		Name:         "Fireplace",
		Manufacturer: "Escea",
		Model:        "DF960",
		SerialNumber: "123456",
		Firmware:     "1.0.0",
	})
	slog.Info("Starting Fireplace server...", "info", a.Info)

	// a.Heater.Hidden = false
	// a.Heater.Active.OnCValueUpdate(func(c *characteristic.C, new, old interface{}, req *http.Request) {
	// 	log.Printf("Heater Active: %t", c.GetValue() == 1)
	// })

	// a.Heater.Active.OnValueRemoteUpdate(func(v int) {
	// 	log.Printf("Heater Active: %t", v == 1)
	// })

	// a.Heater.CurrentTemperature.Unit = characteristic.UnitCelsius
	// a.Heater.CurrentTemperature.SetValue(20)

	// a.Heater.Active.MinVal = 18
	// a.Heater.Active.MaxVal = 30

	// Store the data in the "./db" directory.
	fs := hap.NewMemStore()
	// fs := hap.NewFsStore("./db")

	// Create the hap server.
	server, err := hap.NewServer(fs, a.A)
	if err != nil {
		// stop if an error happens
		return err
	}
	// server.Pin = "00102030"

	// Setup a listener for interrupts and SIGTERM signals
	// to stop the server.
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-c
		slog.Info("Interrupt signal received")
		// Stop delivering signals.
		signal.Stop(c)
		// Cancel the context to stop the server.
		cancel()
	}()

	// Run the server.
	return server.ListenAndServe(ctx)
}
