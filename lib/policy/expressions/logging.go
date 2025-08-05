package expressions

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	timestamp "google.golang.org/protobuf/types/known/timestamppb"
)

var (
	filterInvocations = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "techaro",
		Subsystem: "anubis",
		Name:      "slog_filter_invocations",
	}, []string{"name"})

	filterExecutionTime = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "techaro",
		Subsystem: "anubis",
		Name:      "slog_filter_execution_time_nanoseconds",
		Buckets:   []float64{10, 50, 100, 200, 500, 1000, 2000, 5000, 10000, 20000, 50000, 100000, 200000, 500000, 1000000, 2000000, 5000000, 10000000}, // 10 nanoseconds to 10 milliseconds
	}, []string{"name"})
)

func LogFilter(opts ...cel.EnvOption) (*cel.Env, error) {
	return New(
		// Slog record metadata
		cel.Variable("time", cel.TimestampType),
		cel.Variable("msg", cel.StringType),
		cel.Variable("level", cel.StringType),
		cel.Variable("attrs", cel.MapType(cel.StringType, cel.StringType)),
	)
}

func NewFilter(lg *slog.Logger, name, src string) (*Filter, error) {
	env, err := LogFilter()
	if err != nil {
		return nil, fmt.Errorf("logging: can't create CEL env: %w", err)
	}

	program, err := Compile(env, src)
	if err != nil {
		return nil, fmt.Errorf("logging: can't compile expression: Compile(%q): %w", src, err)
	}

	return &Filter{
		program: program,
		name:    name,
		src:     src,
		log:     lg.With("filter", name),
	}, nil
}

type Filter struct {
	program cel.Program
	name    string
	src     string
	log     *slog.Logger
}

func (f Filter) Filter(ctx context.Context, r slog.Record) bool {
	t0 := time.Now()

	result, _, err := f.program.ContextEval(ctx, &Record{
		Record: r,
	})
	if err != nil {
		f.log.Error("error executing log filter", "err", err, "src", f.src)
		return false
	}
	dur := time.Since(t0)
	filterExecutionTime.WithLabelValues(f.name).Observe(float64(dur.Nanoseconds()))
	filterInvocations.WithLabelValues(f.name).Inc()
	//f.log.Debug("filter execution", "dur", dur.Nanoseconds())

	if val, ok := result.(types.Bool); ok {
		return !bool(val)
	}

	return false
}

type Record struct {
	slog.Record
	attrs map[string]string
}

func (r *Record) Parent() cel.Activation { return nil }

func (r *Record) ResolveName(name string) (any, bool) {
	switch name {
	case "time":
		return &timestamp.Timestamp{Seconds: r.Time.Unix()}, true
	case "msg":
		return r.Message, true
	case "level":
		return r.Level.String(), true
	case "attrs":
		if r.attrs == nil {
			attrs := map[string]string{}

			r.Attrs(func(attr slog.Attr) bool {
				attrs[attr.Key] = attr.Value.String()
				return true
			})

			r.attrs = attrs
			return attrs, true
		}
		return r.attrs, true
	default:
		return nil, false
	}
}
