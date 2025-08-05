package expressions

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func BenchmarkFilter(b *testing.B) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	filter, err := NewFilter(log, "benchmark", `msg == "hello"`)
	if err != nil {
		b.Fatalf("NewFilter() error = %v", err)
	}

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "hello", 0)
	record.AddAttrs(slog.String("foo", "bar"))

	ctx := context.Background()

	b.ReportAllocs()

	for b.Loop() {
		filter.Filter(ctx, record)
	}
}

func BenchmarkFilterAttributes(b *testing.B) {
	for _, numAttrs := range []int{1, 2, 4, 8, 16, 32} {
		b.Run(fmt.Sprintf("%d_attributes", numAttrs), func(b *testing.B) {
			log := slog.New(slog.NewTextHandler(io.Discard, nil))

			var sb strings.Builder
			sb.WriteString(`msg == "hello" && "foo" in attrs`)

			attrs := make([]slog.Attr, numAttrs)
			for i := range numAttrs {
				key := fmt.Sprintf("foo%d", i)
				val := "bar"
				attrs[i] = slog.String(key, val)
			}

			filter, err := NewFilter(log, "benchmark", sb.String())
			if err != nil {
				b.Fatalf("NewFilter() error = %v", err)
			}

			record := slog.NewRecord(time.Now(), slog.LevelInfo, "hello", 0)
			record.AddAttrs(attrs...)

			ctx := context.Background()

			b.ResetTimer()
			b.ReportAllocs()

			for b.Loop() {
				filter.Filter(ctx, record)
			}
		})
	}
}
