package homekit

import (
	"context"
	"fmt"
	syslog "log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/brutella/hap"
	"github.com/brutella/hap/accessory"
	"github.com/brutella/hap/characteristic"
	"github.com/brutella/hap/log"
	slogctx "github.com/veqryn/slog-context"

	"github.com/ivanvanderbyl/escea-fireplace/pkg/firecontrol"
	"github.com/pkg/errors"
	"github.com/sourcegraph/conc/pool"
	"github.com/urfave/cli/v2"
)

type (
	FireplaceController struct {
		fireplace *firecontrol.Fireplace
		accessory *accessory.Thermostat
	}
)

func AccessoryAction(c *cli.Context) error {
	ctx := c.Context

	h := slogctx.NewHandler(slog.NewTextHandler(os.Stdout, nil), nil)
	slog.SetDefault(slog.New(h))

	slog.Info("Starting HomeKit accessory")
	fireplaces, err := firecontrol.SearchForFireplaces()
	if err != nil {
		return errors.Wrap(err, "searching for fireplaces")
	}

	slogctx.Info(ctx, "Completed fireplace search", "found-count", len(fireplaces))

	pin := c.Int("pin")
	serial := c.Int("serial")

	p := pool.New().WithErrors().WithContext(ctx)

	for _, fireplace := range fireplaces {
		if fireplace.Serial != uint32(serial) || fireplace.PIN != uint16(pin) {
			slog.Info("Skipping fireplace", "serial", fireplace.Serial)
			continue
		}

		ctx = slogctx.Append(ctx, "ip", fireplace.Addr.IP.String(), "serial", fireplace.Serial)
		slog.InfoContext(ctx, "Starting Controller")

		controller := &FireplaceController{
			fireplace: fireplace,
		}

		p.Go(controller.Start)
	}

	return p.Wait()
}

type Instruction struct {
	Command firecontrol.CommandCode
	Value   any
}

func (fc *FireplaceController) Start(ctx context.Context) error {
	slog.InfoContext(ctx, "Starting fireplace controller")

	// Setup a listener for interrupts and SIGTERM signals
	// to stop the server.
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)

	// TODO: Move this to main
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		<-c
		slog.Info("Interrupt signal received")
		// Stop delivering signals.
		signal.Stop(c)
		// Cancel the context to stop the server.
		cancel()
	}()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	err := fc.createAccessory(ctx)
	if err != nil {
		return errors.Wrap(err, "creating accessory")
	}

	go func() {
		slog.InfoContext(ctx, "Starting fireplace controller refresh loop")
		for {
			select {
			case <-ticker.C:
				err := fc.refreshStatus(ctx)
				if err != nil {
					slog.ErrorContext(ctx, "Failed to refresh fireplace", "error", err)
					continue
				}

				slog.InfoContext(ctx, "Refreshed fireplace", "room-temperature", fc.fireplace.Status.CurrentTemperature)
			case <-ctx.Done():
				slog.InfoContext(ctx, "Stopping fireplace controller")
				return
			}
		}
	}()

	err = fc.startServer(ctx)
	if err != nil {
		return errors.Wrap(err, "starting server")
	}

	// comandChan := make(chan firecontrol.CommandCode)

	return nil
}

func (fc *FireplaceController) createAccessory(ctx context.Context) error {
	acc := accessory.NewThermostat(accessory.Info{
		Name:         "Fireplace",
		SerialNumber: fmt.Sprintf("FP-%d", fc.fireplace.Serial),
		Manufacturer: "Escea",
	})

	// Configure display units to be in Celsius
	acc.Thermostat.TemperatureDisplayUnits.SetValue(characteristic.TemperatureDisplayUnitsCelsius)

	// target := characteristic.NewTargetTemperature()
	acc.Thermostat.TargetTemperature.SetStepValue(1)
	acc.Thermostat.TargetTemperature.SetMaxValue(30)
	// target.SetMinValue(16)
	// target.SetMaxValue(30)
	// target.SetValue(22)

	acc.Thermostat.TargetTemperature.OnSetRemoteValue(func(v float64) error {
		slog.InfoContext(ctx, "Target Temperature Set", "value", v)
		err := fc.setTargetTemperature(ctx, v)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to set target temperature", "error", err, "temperature", v)
			return errors.Wrap(err, "setting target temperature")
		}
		return nil
	})

	// acc.Thermostat.TargetTemperature = target
	// acc.Thermostat.AddC(target.C)

	acc.Thermostat.TargetHeatingCoolingState.ValidVals = []int{characteristic.TargetHeatingCoolingStateHeat, characteristic.TargetHeatingCoolingStateOff}
	acc.Thermostat.TargetHeatingCoolingState.OnSetRemoteValue(func(v int) error {
		switch v {
		case characteristic.TargetHeatingCoolingStateAuto:
			slog.InfoContext(ctx, "TargetHeatingCoolingState: Auto")
		case characteristic.TargetHeatingCoolingStateHeat:
			slog.InfoContext(ctx, "TargetHeatingCoolingState: Heat")

			// Turn on the fireplace
			err := fc.fireplace.PowerOn()
			if err != nil {
				return errors.Wrap(err, "turning on fireplace")
			}

			time.Sleep(10 * time.Second)
			acc.Thermostat.TargetHeatingCoolingState.SetValue(characteristic.TargetHeatingCoolingStateHeat)
		case characteristic.TargetHeatingCoolingStateOff:
			slog.InfoContext(ctx, "TargetHeatingCoolingState: Off")

			// Turn off the fireplace
			err := fc.fireplace.PowerOff()
			if err != nil {
				return errors.Wrap(err, "turning off fireplace")
			}

			time.Sleep(10 * time.Second)
			acc.Thermostat.TargetHeatingCoolingState.SetValue(characteristic.TargetHeatingCoolingStateOff)
		}

		return nil
	})

	fc.accessory = acc
	return nil
}

func (fc *FireplaceController) refreshStatus(ctx context.Context) error {
	slog.InfoContext(ctx, "Refreshing fireplace status")

	err := fc.fireplace.Refresh()
	if err != nil {
		return errors.Wrap(err, "refreshing fireplace")
	}

	acc := fc.accessory
	acc.Thermostat.CurrentTemperature.SetValue(float64(fc.fireplace.Status.CurrentTemperature))
	acc.Thermostat.TargetTemperature.SetValue(float64(fc.fireplace.Status.TargetTempertaure))

	if fc.fireplace.Status.IsOn {
		acc.Thermostat.CurrentHeatingCoolingState.SetValue(characteristic.CurrentHeatingCoolingStateHeat)
	} else {
		acc.Thermostat.CurrentHeatingCoolingState.SetValue(characteristic.CurrentHeatingCoolingStateOff)
	}

	return nil
}

func (fc *FireplaceController) setTargetTemperature(ctx context.Context, temp float64) error {
	slog.InfoContext(ctx, "Setting target temperature", "temperature", temp)
	err := fc.fireplace.SetTemperature(int(temp))
	if err != nil {
		return errors.Wrap(err, "setting target temperature")
	}

	return nil
}

func (fc *FireplaceController) startServer(ctx context.Context) error {
	slog.Info("Starting HomeKit server")

	fs := hap.NewFsStore("./db")

	mylogger := syslog.New(os.Stdout, "SERV ", syslog.LstdFlags|syslog.Lshortfile)
	log.Debug = &log.Logger{mylogger}

	// Create the hap server.
	server, err := hap.NewServer(fs, fc.accessory.A)
	if err != nil {
		// stop if an error happens
		return errors.Wrap(err, "creating server")
	}

	// Run the server.
	return server.ListenAndServe(ctx)
}
