package internal

import (
	"os"
	"os/exec"
)

func UnbreakDocker() {
	// XXX(Xe): This is bad code. Do not do this.
	//
	// I have to do this because I'm running from inside the context of a dev
	// container. This dev container runs in a different docker network than
	// the valkey test container runs in. In order to let my dev container
	// connect to the test container, they need to share a network in common.
	// The easiest network to use for this is the default "bridge" network.
	//
	// This is a horrifying monstrosity, but the part that scares me the most
	// is the fact that it works.
	if hostname, err := os.Hostname(); err == nil {
		exec.Command("docker", "network", "connect", "bridge", hostname).Run()
	}
}
