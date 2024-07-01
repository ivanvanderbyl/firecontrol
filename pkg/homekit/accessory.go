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

const refreshInterval = 30 * time.Second

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

	// Setup a listener for interrupts and SIGTERM signals
	// to stop the server.
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, os.Interrupt)
	signal.Notify(sigChan, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		<-sigChan
		slog.Info("Interrupt signal received")
		// Stop delivering signals.
		signal.Stop(sigChan)
		// Cancel the context to stop the server.
		cancel()
	}()

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

		// Start the controller in a new goroutine
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

	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	err := fc.createAccessory(ctx)
	if err != nil {
		return errors.Wrap(err, "creating accessory")
	}

	err = fc.refreshStatus(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to refresh fireplace", "error", err)
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

				slog.InfoContext(ctx, "Refreshed fireplace",
					"room-temperature", fc.fireplace.Status.CurrentTemperature,
					"target-temperature", fc.fireplace.Status.TargetTempertaure,
					"status", fireplaceStatusString(fc.fireplace.Status),
				)
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

	// TODO: For some reason enabling this causes the 'Home' app to stop responding to this accessory
	// acc.Thermostat.TargetTemperature.SetMaxValue(16)
	// acc.Thermostat.TargetTemperature.SetValue(22)

	acc.Thermostat.TargetTemperature.OnSetRemoteValue(func(v float64) error {
		slog.InfoContext(ctx, "Target Temperature Set", "value", v)
		err := fc.setTargetTemperature(ctx, v)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to set target temperature", "error", err, "temperature", v)
			return errors.Wrap(err, "setting target temperature")
		}
		return nil
	})

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

			time.Sleep(1 * time.Second)
		case characteristic.TargetHeatingCoolingStateOff:
			slog.InfoContext(ctx, "TargetHeatingCoolingState: Off")

			// Turn off the fireplace
			err := fc.fireplace.PowerOff()
			if err != nil {
				return errors.Wrap(err, "turning off fireplace")
			}

			time.Sleep(1 * time.Second)
		}

		return nil
	})

	fc.accessory = acc
	return nil
}

func (fc *FireplaceController) refreshStatus(_ context.Context) error {
	// slog.InfoContext(ctx, "Refreshing fireplace status")

	err := fc.fireplace.Refresh()
	if err != nil {
		return errors.Wrap(err, "refreshing fireplace")
	}

	acc := fc.accessory
	th := acc.Thermostat
	th.CurrentTemperature.SetValue(float64(fc.fireplace.Status.CurrentTemperature))

	if fc.fireplace.Status.IsOn {
		th.TargetTemperature.SetValue(float64(fc.fireplace.Status.TargetTempertaure))
		err = th.TargetHeatingCoolingState.SetValue(characteristic.TargetHeatingCoolingStateHeat)
		if err != nil {
			return errors.Wrap(err, "setting target heating cooling state")
		}
		err = th.CurrentHeatingCoolingState.SetValue(characteristic.CurrentHeatingCoolingStateHeat)
		if err != nil {
			return errors.Wrap(err, "setting current heating cooling state")
		}
	} else {
		err = th.TargetHeatingCoolingState.SetValue(characteristic.TargetHeatingCoolingStateOff)
		if err != nil {
			return errors.Wrap(err, "setting target heating cooling state")
		}
		err = th.CurrentHeatingCoolingState.SetValue(characteristic.CurrentHeatingCoolingStateOff)
		if err != nil {
			return errors.Wrap(err, "setting current heating cooling state")
		}
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

	newLogger := syslog.New(os.Stdout, "SERV ", syslog.LstdFlags|syslog.Lshortfile)
	log.Debug = &log.Logger{newLogger}

	// Create the hap server.
	server, err := hap.NewServer(fs, fc.accessory.A)
	if err != nil {
		// stop if an error happens
		return errors.Wrap(err, "creating server")
	}

	// Run the server.
	return server.ListenAndServe(ctx)
}

func fireplaceStatusString(status *firecontrol.Status) string {
	if status.IsOn {
		return "On"
	}
	return "Off"
}
