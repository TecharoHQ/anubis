package internal

import (
	"context"
	"log"
	"os"

	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

// UnbreakDocker connects the container named after the current hostname
// to the specified Docker network.
func UnbreakDocker(networkName string) error {
	ctx := context.Background()

	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return err
	}

	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	err = cli.NetworkConnect(ctx, networkName, hostname, &network.EndpointSettings{})
	if err != nil {
		return err
	}

	log.Printf("Connected container %q to network %q\n", hostname, networkName)
	return nil
}
