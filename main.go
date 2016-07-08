package main

import (
	"os"
	"strconv"

	"github.com/urfave/cli"
)

var debug bool

func checkFastlyKey(c *cli.Context) *cli.ExitError {
	if c.GlobalString("fastly-key") == "" {
		return cli.NewExitError("Error: Fastly API key must be set.", -1)
	}
	return nil
}

func main() {
	app := cli.NewApp()
	app.Name = "fastlyctl"

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "config, c",
			Value: "config.toml",
			Usage: "Load Fastly configuration from `FILE`",
		},
		cli.StringFlag{
			Name:   "fastly-key, K",
			Usage:  "Fastly API Key.",
			EnvVar: "FASTLY_KEY",
		},
		cli.BoolFlag{
			Name:  "debug, d",
			Usage: "Print more detailed info for debugging.",
		},
	}

	app.Commands = []cli.Command{
		cli.Command{
			Name:    "sync",
			Aliases: []string{"s"},
			Usage:   "Sync remote service configuration with local config file.",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "all, a",
					Usage: "Sync all services listed in config file",
				},
			},
			Before: func(c *cli.Context) error {
				if err := checkFastlyKey(c); err != nil {
					return err
				}
				if (!c.Bool("all") && !c.Args().Present()) || (c.Bool("all") && c.Args().Present()) {
					cli.ShowAppHelp(c)
					return cli.NewExitError("Error: either specify service names to be syncd, or sync all with -a", -1)
				}
				debug = c.GlobalBool("debug")
				return nil
			},
			Action: syncConfig,
		},
		cli.Command{
			Name:    "version",
			Aliases: []string{"v"},
			Usage:   "Manage service versions.",
			Before: func(c *cli.Context) error {
				if err := checkFastlyKey(c); err != nil {
					return err
				}
				// less than 2 here since the subcommand is the first Arg
				if len(c.Args()) < 2 {
					cli.ShowAppHelp(c)
					return cli.NewExitError("Please specify service.", -1)
				}
				return nil
			},
			Subcommands: cli.Commands{
				cli.Command{
					Name:   "list",
					Usage:  "List versions associated with a given service",
					Action: versionList,
				},
				cli.Command{
					Name:   "validate",
					Usage:  "Validate a specified VERSION",
					Action: versionValidate,
					Before: func(c *cli.Context) error {
						if _, err := strconv.Atoi(c.Args().Get(1)); err != nil {
							cli.ShowAppHelp(c)
							return cli.NewExitError("Please specify version to validate.", -1)
						}
						return nil
					},
				},
				cli.Command{
					Name:   "activate",
					Usage:  "Activate a specified VERSION",
					Action: versionActivate,
					Before: func(c *cli.Context) error {
						if _, err := strconv.Atoi(c.Args().Get(1)); err != nil {
							cli.ShowAppHelp(c)
							return cli.NewExitError("Please specify version to activate.", -1)
						}
						return versionValidate(c)
					},
				},
			},
		},
		cli.Command{
			Name:  "service",
			Usage: "Manage services.",
			Before: func(c *cli.Context) error {
				if err := checkFastlyKey(c); err != nil {
					return err
				}
				return nil
			},
			Subcommands: cli.Commands{
				cli.Command{
					Name:   "list",
					Usage:  "List services associated with account",
					Action: serviceList,
				},
			},
		},
	}

	app.Run(os.Args)
}
