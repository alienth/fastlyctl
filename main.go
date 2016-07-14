package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/urfave/cli"
)

var debug bool

func debugPrint(message string) {
	if debug {
		fmt.Print(message)
	}
}

func checkFastlyKey(c *cli.Context) *cli.ExitError {
	if c.GlobalString("fastly-key") == "" {
		return cli.NewExitError("Error: Fastly API key must be set.", -1)
	}
	return nil
}

func checkInteractive(c *cli.Context) *cli.ExitError {
	interactive := terminal.IsTerminal(syscall.Stdin)
	if !interactive && !c.GlobalBool("assume-yes") {
		return cli.NewExitError("In non-interactive shell and --assume-yes not used, exiting.", -1)

	}
	return nil
}

func getFastlyKey() string {
	file := "fastly_key"
	if _, err := os.Stat(file); err == nil {
		contents, _ := ioutil.ReadFile(file)
		if contents[len(contents)-1] == []byte("\n")[0] {
			contents = contents[:len(contents)-1]
		}
		return string(contents)
	}
	return ""
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
			Usage:  "Fastly API Key. Can be read from 'fastly_key' file in CWD.",
			EnvVar: "FASTLY_KEY",
			Value:  getFastlyKey(),
		},
		cli.BoolFlag{
			Name:  "debug, d",
			Usage: "Print more detailed info for debugging.",
		},
		cli.BoolFlag{
			Name:  "assume-yes, y",
			Usage: "Assume 'yes' to all prompts. USE ONLY IF YOU ARE CERTAIN YOUR COMMANDS WON'T BREAK ANYTHING!",
		},
	}

	app.Before = func(c *cli.Context) error {
		if err := checkFastlyKey(c); err != nil {
			return err
		}
		return nil
	}

	app.Commands = []cli.Command{
		cli.Command{
			Name:      "push",
			Aliases:   []string{"p"},
			Usage:     "Push locally defined service configuration options to Fastly.",
			ArgsUsage: "(<SERVICE_NAME> | <SERVICE_ID>)...",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "all, a",
					Usage: "Push all services listed in config file",
				},
				cli.BoolFlag{
					Name:  "noop, n",
					Usage: "Push new config versions, but do not activate.",
				},
			},
			Before: func(c *cli.Context) error {
				if err := checkInteractive(c); err != nil {
					return err
				}
				if (!c.Bool("all") && !c.Args().Present()) || (c.Bool("all") && c.Args().Present()) {
					return cli.NewExitError("Error: either specify service names to be pushed, or push all with -a", -1)
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
				// less than 2 here since the subcommand is the first Arg
				if len(c.Args()) < 2 {
					return cli.NewExitError("Please specify service.", -1)
				}
				return nil
			},
			Subcommands: cli.Commands{
				cli.Command{
					Name:      "list",
					Usage:     "List versions associated with a given service",
					Action:    versionList,
					ArgsUsage: "(<SERVICE_NAME> | <SERVICE_ID>)",
				},
				cli.Command{
					Name:      "validate",
					Usage:     "Validate a specified VERSION",
					ArgsUsage: "(<SERVICE_NAME> | <SERVICE_ID>) <VERSION>",
					Action:    versionValidate,
					Before: func(c *cli.Context) error {
						if _, err := strconv.Atoi(c.Args().Get(1)); err != nil {
							return cli.NewExitError("Please specify version to validate.", -1)
						}
						return nil
					},
				},
				cli.Command{
					Name:      "activate",
					Usage:     "Activate a specified VERSION",
					ArgsUsage: "(<SERVICE_NAME> | <SERVICE_ID>) <VERSION>",
					Action:    versionActivate,
					Before: func(c *cli.Context) error {
						if err := checkInteractive(c); err != nil {
							return err
						}
						if _, err := strconv.Atoi(c.Args().Get(1)); err != nil {
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
			Subcommands: cli.Commands{
				cli.Command{
					Name:   "list",
					Usage:  "List services associated with account",
					Action: serviceList,
				},
			},
		},
		cli.Command{
			Name:    "dictionary",
			Aliases: []string{"d"},
			Usage:   "Manage dictionaries.",
			Before: func(c *cli.Context) error {
				// less than 2 here since the subcommand is the first Arg
				if len(c.Args()) < 2 {
					return cli.NewExitError("Please specify service.", -1)
				}
				return nil
			},
			Subcommands: cli.Commands{
				cli.Command{
					Name:      "list",
					Usage:     "List dictionaries associated with a given service",
					Action:    dictionaryList,
					ArgsUsage: "(<SERVICE_NAME> | <SERVICE_ID>)",
				},
				cli.Command{
					Name:      "item-add",
					Usage:     "Add an item to a dictionary",
					Action:    dictionaryAddItem,
					ArgsUsage: "(<SERVICE_NAME> | <SERVICE_ID>) <DICTIONARY_NAME> <ITEM_KEY> <ITEM_VALUE>",
				},
				cli.Command{
					Name:      "item-rm",
					Usage:     "Remove an item from a dictionary",
					Action:    dictionaryRemoveItem,
					ArgsUsage: "(<SERVICE_NAME> | <SERVICE_ID>) <DICTIONARY_NAME> <ITEM_KEY>",
				},
				cli.Command{
					Name:      "item-ls",
					Usage:     "List items in a dictionary",
					Action:    dictionaryListItems,
					ArgsUsage: "(<SERVICE_NAME> | <SERVICE_ID>) <DICTIONARY_NAME>",
				},
			},
		},
	}

	app.Run(os.Args)
}
