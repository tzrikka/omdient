package main

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/rs/zerolog/log"
	altsrc "github.com/urfave/cli-altsrc/v3"
	"github.com/urfave/cli/v3"

	"github.com/tzrikka/omdient/pkg/etcd"
	"github.com/tzrikka/omdient/pkg/http"
	"github.com/tzrikka/omdient/pkg/thrippy"
	"github.com/tzrikka/xdg"
)

const (
	ConfigDirName  = "omdient"
	ConfigFileName = "config.toml"
)

func main() {
	buildInfo, _ := debug.ReadBuildInfo()
	configFilePath := configFile()

	flags := []cli.Flag{
		&cli.BoolFlag{
			Name:  "dev",
			Usage: "simple setup, but unsafe for production",
		},
	}
	flags = append(flags, http.Flags(configFilePath)...)
	flags = append(flags, thrippy.Flags(configFilePath)...)
	flags = append(flags, etcd.Flags(configFilePath)...)

	cmd := &cli.Command{
		Name:    "omdient",
		Usage:   "Listen to events notifications over HTTP webhooks, WebSockets, and Pub/Sub",
		Version: buildInfo.Main.Version,
		Flags:   flags,
		Action:  http.Start,
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

// configFile returns the path to the app's configuration file.
// It also creates an empty file if it doesn't already exist.
func configFile() altsrc.StringSourcer {
	path, err := xdg.CreateFile(xdg.ConfigHome, ConfigDirName, ConfigFileName)
	if err != nil {
		log.Fatal().Err(err).Caller().Send()
	}
	return altsrc.StringSourcer(path)
}
