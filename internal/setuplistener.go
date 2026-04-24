package internal

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// parseBindNetFromAddr determine bind network and address based on the given network and address.
func parseBindNetFromAddr(address string) (string, string, error) {
	defaultScheme := "http://"
	if !strings.Contains(address, "://") {
		if strings.HasPrefix(address, ":") {
			address = defaultScheme + "localhost" + address
		} else {
			address = defaultScheme + address
		}
	}

	bindUri, err := url.Parse(address)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse bind URL: %w", err)
	}

	switch bindUri.Scheme {
	case "unix":
		return "unix", bindUri.Path, nil
	case "tcp", "http", "https":
		return "tcp", bindUri.Host, nil
	default:
		return "", "", fmt.Errorf("unsupported network scheme %s in address %s", bindUri.Scheme, address)
	}
}

// SetupListener sets up a network listener based on the input from configuration
// envvars. It returns a network listener and the URL to that listener or an error.
func SetupListener(network, address, socketMode string) (net.Listener, string, error) {
	formattedAddress := ""
	var err error

	if network == "" {
		// keep compatibility
		network, address, err = parseBindNetFromAddr(address)
	}

	if err != nil {
		return nil, "", fmt.Errorf("can't parse bind and network: %w", err)
	}

	switch network {
	case "unix":
		formattedAddress = "unix:" + address
	case "tcp":
		if strings.HasPrefix(address, ":") { // assume it's just a port e.g. :4259
			formattedAddress = "http://localhost" + address
		} else {
			formattedAddress = "http://" + address
		}
	default:
		formattedAddress = fmt.Sprintf(`(%s) %s`, network, address)
	}

	ln, err := net.Listen(network, address)
	if err != nil {
		return nil, "", fmt.Errorf("failed to bind to %s: %w", formattedAddress, err)
	}

	// additional permission handling for unix sockets
	if network == "unix" {
		mode, err := strconv.ParseUint(socketMode, 8, 0)
		if err != nil {
			ln.Close()
			return nil, "", fmt.Errorf("could not parse socket mode %s: %w", socketMode, err)
		}

		err = os.Chmod(address, os.FileMode(mode))
		if err != nil {
			err := fmt.Errorf("could not change socket mode: %w", err)
			clErr := ln.Close()
			if clErr != nil {
				return nil, "", errors.Join(err, clErr)
			}
			return nil, "", err
		}
	}

	return ln, formattedAddress, nil
}
