package lib

import "sync"

type metricHostLabels struct {
	mu    sync.Mutex
	hosts map[string]struct{}
}

var proxiedHostLabels metricHostLabels

func (h *metricHostLabels) label(host string) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.hosts[host]; ok {
		return host
	}
	if len(h.hosts) >= 64 {
		return "_other"
	}
	if h.hosts == nil {
		h.hosts = make(map[string]struct{}, 64)
	}
	h.hosts[host] = struct{}{}
	return host
}
