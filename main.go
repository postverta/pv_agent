package main

import (
	"github.com/postverta/pv_agent/client"
	"github.com/postverta/pv_agent/server"
	"gopkg.in/urfave/cli.v2"
	"os"
)

func daemonAction(c *cli.Context) error {
	if _, err := os.Stat(c.String("exec")); err != nil {
		return cli.Exit("Cannot find pv_exec binary at "+c.String("exec"), 1)
	}

	err := server.Start(c.String("host"), c.Uint("p"), c.String("exec"))
	if err != nil {
		return cli.Exit("Failed to start server, err:"+err.Error(), 1)
	} else {
		return nil
	}
}

func openAction(c *cli.Context) error {
	if c.Bool("stress") {
		client.StressTest(
			c.String("host"),
			c.Uint("p"),
			c.IntSlice("exposed-ports"),
			c.StringSlice("env"),
			c.StringSlice("exec-config-root"),
			c.String("image"),
			c.String("account-name"),
			c.String("account-key"),
			c.String("container"),
			c.String("source-worktree"),
			c.String("worktree"),
			c.String("mount-point"),
			c.Uint("autosave-interval"),
		)
		return nil
	} else {
		err := client.OpenContext(
			c.String("host"),
			c.Uint("p"),
			c.IntSlice("exposed-ports"),
			c.StringSlice("env"),
			c.StringSlice("exec-config-root"),
			c.String("image"),
			c.String("account-name"),
			c.String("account-key"),
			c.String("container"),
			c.String("source-worktree"),
			c.String("worktree"),
			c.String("mount-point"),
			c.Uint("autosave-interval"),
		)
		if err != nil {
			return cli.Exit("Failed to open context, err:"+err.Error(), 1)
		} else {
			return nil
		}
	}
}

func closeAction(c *cli.Context) error {
	if c.NArg() < 1 {
		return cli.Exit("Not enough parameters for closing a context", 1)
	}

	contextId := c.Args().Get(0)
	err := client.CloseContext(c.String("host"), c.Uint("p"), contextId)
	if err != nil {
		return cli.Exit("Failed to close context, err:"+err.Error(), 1)
	} else {
		return nil
	}
}

func main() {
	app := &cli.App{
		Name: "pv_agent",
		Flags: []cli.Flag{
			&cli.UintFlag{
				Name:  "p",
				Usage: "Port to listen/connect to",
				Value: 50000,
			},
			&cli.StringFlag{
				Name:  "host",
				Usage: "Host to listen/connect to",
				Value: "127.0.0.1",
			},
		},
		Commands: []*cli.Command{
			&cli.Command{
				Name:  "daemon",
				Usage: "pv_agent [-p PORT] [-host HOST] daemon [-exec PV_EXEC_BIN_PATH]",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "exec",
						Usage: "pv_exec path",
						Value: "/usr/bin/pv_exec",
					},
				},
				Action: daemonAction,
			},
			&cli.Command{
				Name:  "open",
				Usage: "pv_agent [-p PORT] [-host HOST] open [-exposed-port PORT0...] [-env ENV0...] [-exec-config-root EXEC_CONFIG_ROOT0...] [-stress] [-image IMAGE] [-account-name AZURE_ACCOUNT_NAME] [-account-key AZURE_ACCOUNT_KEY] [-container AZURE_CONTAINER] [-source-worktree SOURCE_WORKTREE_ID] [-worktree WORKTREE_ID] [-mount-point MOUNT_POINT] [-autosave-interval AUTOSAVE_INTERVAL]",
				Flags: []cli.Flag{
					&cli.IntSliceFlag{
						Name:  "exposed-ports",
						Usage: "Ports to be exposed",
					},
					&cli.StringSliceFlag{
						Name:  "env",
						Usage: "Environmental variables",
					},
					&cli.StringSliceFlag{
						Name:  "exec-config-root",
						Usage: "List of pv_exec exec service config roots",
					},
					&cli.BoolFlag{
						Name:  "stress",
						Usage: "Stressing open/close loop. Testing only",
					},
					&cli.StringFlag{
						Name:  "image",
						Usage: "Base image",
					},
					&cli.StringFlag{
						Name:  "account-name",
						Usage: "Azure storage account name",
					},
					&cli.StringFlag{
						Name:  "account-key",
						Usage: "Azure storage account key",
					},
					&cli.StringFlag{
						Name:  "container",
						Usage: "Azure blob container name",
					},
					&cli.StringFlag{
						Name:  "source-worktree",
						Usage: "Source worktree ID (optional)",
						Value: "",
					},
					&cli.StringFlag{
						Name:  "worktree",
						Usage: "Worktree ID",
					},
					&cli.StringFlag{
						Name:  "mount-point",
						Usage: "Mount point",
					},
					&cli.UintFlag{
						Name:  "autosave-interval",
						Usage: "Autosave interval (in seconds)",
					},
				},
				Action: openAction,
			},
			&cli.Command{
				Name:   "close",
				Usage:  "pv_agent [-p PORT] [-host HOST] close CONTEXT_ID",
				Action: closeAction,
			},
		},
	}

	app.Run(os.Args)
}
