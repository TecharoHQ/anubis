package expressions

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/load"
)

type loadAvg struct {
	data *load.AvgStat
	lock sync.RWMutex
}

func (l *loadAvg) updateThread(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	l.update()

	for {
		select {
		case <-ticker.C:
			l.update()
		case <-ctx.Done():
			return
		}
	}
}

func (l *loadAvg) update() {
	l.lock.Lock()
	defer l.lock.Unlock()

	var err error
	l.data, err = load.Avg()
	if err != nil {
		slog.Debug("can't get load average", "err", err)
	}
}

var (
	globalLoadAvg *loadAvg
)

func init() {
	globalLoadAvg = &loadAvg{}
	go globalLoadAvg.updateThread(context.Background())
}

func Load1() float64 {
	globalLoadAvg.lock.RLock()
	defer globalLoadAvg.lock.RUnlock()
	return globalLoadAvg.data.Load1
}

func Load5() float64 {
	globalLoadAvg.lock.RLock()
	defer globalLoadAvg.lock.RUnlock()
	return globalLoadAvg.data.Load5
}

func Load15() float64 {
	globalLoadAvg.lock.RLock()
	defer globalLoadAvg.lock.RUnlock()
	return globalLoadAvg.data.Load15
}
