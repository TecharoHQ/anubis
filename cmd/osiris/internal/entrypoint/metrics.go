package entrypoint

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"sort"

	"github.com/TecharoHQ/anubis/internal"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
)

func healthz(w http.ResponseWriter, r *http.Request) {
	services, err := internal.HealthSrv.List(r.Context(), nil)
	if err != nil {
		slog.Error("can't get list of services", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var keys []string
	for k := range services.Statuses {
		if k == "" {
			continue
		}
		keys = append(keys, k)
	}

	sort.Strings(keys)

	var msg bytes.Buffer

	var healthy bool = true

	for _, k := range keys {
		st := services.Statuses[k].GetStatus()
		fmt.Fprintf(&msg, "%s: %s\n", k, st)
		switch st {
		case healthv1.HealthCheckResponse_SERVING:
			// do nothing
		default:
			healthy = false
		}
	}

	if !healthy {
		w.WriteHeader(http.StatusInternalServerError)
	}

	w.Write(msg.Bytes())
}

func readyz(w http.ResponseWriter, r *http.Request) {
	st, ok := internal.GetHealth("osiris")
	if !ok {
		slog.Error("health service osiris does not exist, file a bug")
		http.Error(w, "health service osiris does not exist", http.StatusExpectationFailed)
	}

	switch st {
	case healthv1.HealthCheckResponse_NOT_SERVING:
		http.Error(w, "NOT OK", http.StatusInternalServerError)
		return
	case healthv1.HealthCheckResponse_SERVING:
		fmt.Fprintln(w, "OK")
		return
	default:
		http.Error(w, "UNKNOWN", http.StatusFailedDependency)
		return
	}
}
