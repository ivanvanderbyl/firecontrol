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
		fireplace           *firecontrol.Fireplace
		accessory           *accessory.Thermostat
		debugLoggingEnabled bool
		queue               chan Envelope
	}

	Instruction interface {
		// Execute(ctx context.Context, fireplace *firecontrol.Fireplace) error
		isInstruction()
	}

	internalInstruction struct {
		responseChan chan error
	}

	SetTemperatureInstruction struct {
		*internalInstruction
		Temperature int
	}

	SetPowerInstruction struct {
		*internalInstruction
		Power bool
	}

	Envelope struct {
		Instruction  Instruction
		responseChan chan error
	}
)

func (i SetTemperatureInstruction) isInstruction() {}
func (i SetPowerInstruction) isInstruction()       {}

func NewMessageEnvelope(instruction Instruction) Envelope {
	return Envelope{Instruction: instruction, responseChan: make(chan error, 1)}
}
func (i Envelope) Complete(err error) {
	if i.responseChan == nil {
		return
	}
	i.responseChan <- err
}

func NewTemperatureInstruction(temp int) Instruction {
	return SetTemperatureInstruction{
		internalInstruction: &internalInstruction{responseChan: make(chan error, 1)},
		Temperature:         temp,
	}
}

func NewPowerInstruction(power bool) Instruction {
	return SetPowerInstruction{
		internalInstruction: &internalInstruction{responseChan: make(chan error, 1)},
		Power:               power,
	}
}

const refreshInterval = 30 * time.Second

func AccessoryAction(c *cli.Context) error {
	ctx := c.Context

	h := slogctx.NewHandler(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}), nil)

	slog.SetDefault(slog.New(h))
	slog.SetLogLoggerLevel(slog.LevelDebug)

	slog.Debug("Starting HomeKit accessory with debug logging enabled", "debug", c.Bool("debug"))

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
			fireplace:           fireplace,
			debugLoggingEnabled: c.Bool("debug"),
			queue:               make(chan Envelope, 10),
		}

		// Start the controller in a new goroutine
		p.Go(controller.Start)
	}

	return p.Wait()
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
				close(fc.queue)
				slog.InfoContext(ctx, "Stopping fireplace controller")
				return

			case msg := <-fc.queue:
				slog.Debug("Received instruction", "instruction", msg.Instruction)
				switch i := msg.Instruction.(type) {
				case SetTemperatureInstruction:
					msg.Complete(fc.setTargetTemperature(ctx, float64(i.Temperature)))
				case SetPowerInstruction:
					if i.Power {
						msg.Complete(fc.fireplace.PowerOn())
					} else {
						msg.Complete(fc.fireplace.PowerOff())
					}
				}
			default:
				// To avoid busy waiting
				time.Sleep(500 * time.Millisecond)
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
	// acc.Thermostat.TargetTemperature.SetMinValue(16)
	// acc.Thermostat.TargetTemperature.SetValue(22)

	acc.Thermostat.TargetTemperature.OnSetRemoteValue(func(v float64) error {
		slog.InfoContext(ctx, "Target Temperature Set", "value", v)

		msg := NewMessageEnvelope(NewTemperatureInstruction(int(v)))
		fc.queue <- msg

		err := <-msg.responseChan
		if err != nil {
			slog.ErrorContext(ctx, "Failed to set target temperature", "error", err, "temperature", v)
			return errors.Wrap(err, "setting target temperature")
		}
		slog.InfoContext(ctx, "Successfully set target temperature", "temperature", v)
		return nil
	})

	acc.Thermostat.TargetHeatingCoolingState.ValidVals = []int{characteristic.TargetHeatingCoolingStateHeat, characteristic.TargetHeatingCoolingStateOff}
	acc.Thermostat.TargetHeatingCoolingState.OnSetRemoteValue(func(targetState int) error {
		switch targetState {
		case characteristic.TargetHeatingCoolingStateAuto:
			slog.InfoContext(ctx, "TargetHeatingCoolingState: Auto")
		case characteristic.TargetHeatingCoolingStateHeat:
			slog.InfoContext(ctx, "TargetHeatingCoolingState: Heat")

			msg := NewMessageEnvelope(NewPowerInstruction(true))
			fc.queue <- msg

			err := <-msg.responseChan
			if err != nil {
				slog.ErrorContext(ctx, "Failed to set power state to on", "error", err, "temperature", targetState)
				return errors.Wrap(err, "set power state")
			}
			slog.InfoContext(ctx, "Successfully set power state to on", "temperature", targetState)
			return nil
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
