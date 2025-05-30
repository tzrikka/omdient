package etcd

import (
	altsrc "github.com/urfave/cli-altsrc/v3"
	"github.com/urfave/cli-altsrc/v3/toml"
	"github.com/urfave/cli/v3"
)

const (
	DefaultEndpoint = "http://localhost:2379"
)

// Flags defines CLI flags to configure an etcd gRPC client. These flags can also
// be set using environment variables and the application's configuration file.
func Flags(configFilePath altsrc.StringSourcer) []cli.Flag {
	return []cli.Flag{
		&cli.StringSliceFlag{
			Name:  "etcd-endpoint-urls",
			Usage: "one or more etcd server endpoint URLs",
			Value: []string{DefaultEndpoint},
			Sources: cli.NewValueSourceChain(
				cli.EnvVar("ETCD_ENDPOINTS"),
				toml.TOML("etcd.endpoint_urls", configFilePath),
			),
		},
	}
}
