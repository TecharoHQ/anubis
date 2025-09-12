package logging

import (
	"context"
	"log/slog"
)

// Filterer is the shape of any type that can perform log filtering. This takes
// the context of the log filtering call and the log record to be filtered.
type Filterer interface {
	Filter(ctx context.Context, r slog.Record) bool
}

// FilterFunc lets you make inline log filters with plain functions.
type FilterFunc func(ctx context.Context, r *slog.Record) bool

// Filter implements Filterer for FilterFunc.
func (ff FilterFunc) Filter(ctx context.Context, r *slog.Record) bool {
	return ff(ctx, r)
}

// FilterHandler wraps a slog Handler with one or more filters, enabling administrators
// to customize the logging subsystem of Anubis.
type FilterHandler struct {
	next    slog.Handler
	filters []Filterer
}

// NewFilterHandler creates a new filtering handler with the given base handler and filters.
func NewFilterHandler(handler slog.Handler, filters ...Filterer) *FilterHandler {
	return &FilterHandler{
		next:    handler,
		filters: filters,
	}
}

// Enabled passes through to the upstream slog Handler.
func (h *FilterHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

// Handle implements slog.Handler and applies all filters before delegating to the base handler.
func (h *FilterHandler) Handle(ctx context.Context, r slog.Record) error {
	// Apply all filters - if any filter returns false, skip the log
	for _, filter := range h.filters {
		if !filter.Filter(ctx, r) {
			return nil // Skip this log record
		}
	}
	return h.next.Handle(ctx, r)
}

// WithAttrs implements slog.Handler.
func (h *FilterHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &FilterHandler{
		next:    h.next.WithAttrs(attrs),
		filters: h.filters,
	}
}

// WithGroup implements slog.Handler.
func (h *FilterHandler) WithGroup(name string) slog.Handler {
	return &FilterHandler{
		next:    h.next.WithGroup(name),
		filters: h.filters,
	}
}
