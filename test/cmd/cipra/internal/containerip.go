package internal

import (
	"context"
	"fmt"

	"github.com/docker/docker/client"
)

// GetContainerIPAddress returns the first non-empty IP address of the container with the given name.
// It returns the IP address as a string or an error.
func GetContainerIPAddress(containerName string) (string, error) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return "", err
	}

	// Get container details
	containerJSON, err := cli.ContainerInspect(ctx, containerName)
	if err != nil {
		return "", err
	}

	// Loop through all networks and return the first IP address found
	for _, net := range containerJSON.NetworkSettings.Networks {
		if net.IPAddress != "" {
			return net.IPAddress, nil
		}
	}

	return "", fmt.Errorf("no IP address found for container %q", containerName)
}
