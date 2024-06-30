package main

import (
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"

	"github.com/ivanvanderbyl/escea-fireplace/pkg/firecontrol"
	"github.com/urfave/cli/v2"
)

func main() {
	commonFlags := []cli.Flag{
		&cli.StringFlag{
			Name:     "ip",
			Usage:    "IP address of the fireplace",
			Required: true,
		},
	}

	app := &cli.App{
		Name:  "firecontrol",
		Usage: "Remote control for Escea fireplaces",
		Commands: []*cli.Command{
			{
				Name:  "search",
				Usage: "Search for fireplaces on the network",
				Action: func(c *cli.Context) error {
					fs, err := firecontrol.SearchForFireplaces()
					if err != nil {
						slog.Error("Error searching for fireplaces", "error", err)
						return err
					}

					for _, f := range fs {
						slog.Info("Found Fireplace", "IP", f.Addr, "Serial", f.Serial, "PIN", f.PIN)
					}
					return nil
				},
			},
			{
				Name:  "status",
				Usage: "Get the status of a fireplace",
				Flags: commonFlags,
				Action: func(c *cli.Context) error {
					addr := net.ParseIP(c.String("ip"))

					fp := firecontrol.NewFireplace(addr)
					err := fp.Refresh()
					if err != nil {
						slog.Error("Failed to refresh fireplace", "error", err)
						return err
					}

					fmt.Printf("Fireplace: %s\n\tFire Status: %s\n\tFlame Effect: %s\n\tFan Boost: %s\n\tDesired Temperature: %dºC\n\tRoom Temperature: %dºC\n",
						fp.Addr.IP.String(),
						formatBoolean(fp.Status.IsOn),
						formatBoolean(fp.Status.FlameEffectIsOn),
						formatBoolean(fp.Status.FanBoostIsOn),
						fp.Status.DesiredTempertaure, fp.Status.RoomTemperature,
					)
					return nil
				},
			},
			{
				Name:  "power-on",
				Usage: "Power on the fireplace",
				Flags: commonFlags,
				Action: func(c *cli.Context) error {
					fmt.Println("Powering on the fireplace...")

					addr := net.ParseIP(c.String("ip"))

					fp := firecontrol.NewFireplace(addr)
					err := fp.PowerOn()
					if err != nil {
						slog.Error("Failed to power on fireplace", "error", err)
						return err
					}

					slog.Info("Fireplace powered on", "IP", c.String("ip"))
					return nil
				},
			},
			{
				Name:  "power-off",
				Usage: "Power off the fireplace",
				Flags: commonFlags,
				Action: func(c *cli.Context) error {
					fmt.Println("Powering off the fireplace...")

					addr := net.ParseIP(c.String("ip"))

					fp := firecontrol.NewFireplace(addr)
					err := fp.PowerOff()
					if err != nil {
						slog.Error("Failed to power off fireplace", "error", err)
						return err
					}

					slog.Info("Fireplace powered off", "IP", c.String("ip"))
					return nil
				},
			},
			{
				Name:  "set-temp",
				Usage: "Set the temperature of the fireplace",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "ip",
						Usage:    "IP address of the fireplace",
						Required: true,
					},
					&cli.IntFlag{
						Name:     "temp",
						Usage:    "Temperature to set",
						Required: true,
					},
				},
				Action: func(c *cli.Context) error {
					fmt.Println("Setting temperature...")

					addr := net.ParseIP(c.String("ip"))

					fp := firecontrol.NewFireplace(addr)
					err := fp.SetTemperature(c.Int("temp"))
					if err != nil {
						slog.Error("Failed to set temperature", "error", err)
						return err
					}

					slog.Info("Temperature set", "IP", c.String("ip"), "Temperature", c.Int("temp"))
					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func formatBoolean(b bool) string {
	if b {
		return "On"
	}
	return "Off"
}
