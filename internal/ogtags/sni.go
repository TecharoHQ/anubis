package ogtags

import (
	"crypto/tls"
	"net/http"
)

// clientForSNI returns a cached client for the given server name, creating one if needed.
func (c *OGTagCache) clientForSNI(serverName string) *http.Client {
	if !c.targetSNIAuto || serverName == "" {
		return c.client
	}

	c.transportMu.RLock()
	cli, ok := c.sniClients[serverName]
	c.transportMu.RUnlock()
	if ok {
		return cli
	}

	c.transportMu.Lock()
	defer c.transportMu.Unlock()
	if cli, ok := c.sniClients[serverName]; ok {
		return cli
	}

	tr := c.transport.Clone()
	if tr.TLSClientConfig == nil {
		tr.TLSClientConfig = &tls.Config{}
	}
	tr.TLSClientConfig.ServerName = serverName
	if c.insecureSkipVerify {
		tr.TLSClientConfig.InsecureSkipVerify = true
	}

	cli = &http.Client{
		Timeout:   httpTimeout,
		Transport: tr,
	}
	c.sniClients[serverName] = cli
	return cli
}
