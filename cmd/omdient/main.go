package main

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/rs/zerolog/log"
	altsrc "github.com/urfave/cli-altsrc/v3"
	"github.com/urfave/cli/v3"

	"github.com/tzrikka/omdient/internal/thrippy"
	"github.com/tzrikka/omdient/pkg/etcd"
	"github.com/tzrikka/omdient/pkg/http"
	"github.com/tzrikka/xdg"
)

const (
	ConfigDirName  = "omdient"
	ConfigFileName = "config.toml"
)

func main() {
	bi, _ := debug.ReadBuildInfo()

	cmd := &cli.Command{
		Name:    "omdient",
		Usage:   "Listen to events notifications over HTTP webhooks, WebSockets, and Pub/Sub",
		Version: bi.Main.Version,
		Flags:   flags(),
		Action:  http.Start,
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func flags() []cli.Flag {
	fs := []cli.Flag{
		&cli.BoolFlag{
			Name:  "dev",
			Usage: "simple setup, but unsafe for production",
		},
	}

	path := configFile()
	fs = append(fs, http.Flags(path)...)
	fs = append(fs, thrippy.Flags(path)...)
	fs = append(fs, etcd.Flags(path)...)
	return fs
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
