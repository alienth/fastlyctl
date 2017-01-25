package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/alienth/fastlyctl/log"
	"github.com/alienth/fastlyctl/util"
	"github.com/urfave/cli"
)

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
			Value:  util.GetFastlyKey(),
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
		if err := util.CheckFastlyKey(c); err != nil {
			return err
		}
		return nil
	}

	app.Commands = []cli.Command{
		cli.Command{
			Name:      "push",
			Aliases:   []string{"p"},
			Usage:     "Push locally defined service configuration options to Fastly.",
			ArgsUsage: "<SERVICE_NAME>...",
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
				if !util.IsInteractive() && !c.GlobalBool("assume-yes") {
					return cli.NewExitError(util.ErrNonInteractive.Error(), -1)
				}
				if (!c.Bool("all") && !c.Args().Present()) || (c.Bool("all") && c.Args().Present()) {
					return cli.NewExitError("Error: either specify service names to be pushed, or push all with -a", -1)
				}
				if c.GlobalBool("debug") {
					log.EnableDebug()
				}
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
					ArgsUsage: "<SERVICE_NAME>",
				},
				cli.Command{
					Name:      "validate",
					Usage:     "Validate a specified VERSION",
					ArgsUsage: "<SERVICE_NAME> <VERSION>",
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
					ArgsUsage: "<SERVICE_NAME> <VERSION>",
					Action:    versionActivate,
					Before: func(c *cli.Context) error {
						if !util.IsInteractive() && !c.GlobalBool("assume-yes") {
							return cli.NewExitError(util.ErrNonInteractive.Error(), -1)
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
					ArgsUsage: "<SERVICE_NAME>",
				},
				cli.Command{
					Name:      "item-add",
					Usage:     "Add an item to a dictionary",
					Action:    dictionaryAddItem,
					ArgsUsage: "<SERVICE_NAME> <DICTIONARY_NAME> <ITEM_KEY> <ITEM_VALUE>",
				},
				cli.Command{
					Name:      "item-rm",
					Usage:     "Remove an item from a dictionary",
					Action:    dictionaryRemoveItem,
					ArgsUsage: "<SERVICE_NAME> <DICTIONARY_NAME> <ITEM_KEY>",
				},
				cli.Command{
					Name:      "item-ls",
					Usage:     "List items in a dictionary",
					Action:    dictionaryListItems,
					ArgsUsage: "<SERVICE_NAME> <DICTIONARY_NAME>",
				},
			},
		},
		cli.Command{
			Name:  "acl",
			Usage: "Manage Edge ACLs.",
			Before: func(c *cli.Context) error {
				// less than 2 here since the subcommand is the first Arg
				if len(c.Args()) < 2 {
					cli.ShowAppHelp(c)
					return cli.NewExitError("Please specify service.", -1)
				}
				return nil
			},
			Subcommands: cli.Commands{
				cli.Command{
					Name:      "list",
					Usage:     "List acls associated with a given service",
					Action:    aclList,
					ArgsUsage: "<SERVICE_NAME>",
				},
				cli.Command{
					Name:      "entry-add",
					Usage:     "Add an entry to a acl",
					Action:    aclAddEntry,
					ArgsUsage: "<SERVICE_NAME> <ACL_NAME> <IP>[/<MASK>]",
				},
				cli.Command{
					Name:      "entry-rm",
					Usage:     "Remove an entry from an acl",
					Action:    aclRemoveEntry,
					ArgsUsage: "<SERVICE_NAME> <ACL_NAME> <IP>[/<MASK>]",
				},
				cli.Command{
					Name:      "entry-ls",
					Usage:     "List entries in an acl",
					Action:    aclListEntries,
					ArgsUsage: "<SERVICE_NAME> <ACL_NAME>",
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Printf("Error starting app: %s", err)
	}

}
