package geoip

import (
	"context"
	"log/slog"

	"github.com/TecharoHQ/anubis/lib/config"
)

// Test hooks. These let geoip_test drive the update loop one step at a time.

func (db *DB) CheckASNFile(ctx context.Context, lg *slog.Logger) {
	db.checkFile(ctx, lg, db.asn)
}

func (db *DB) UpdateASN(ctx context.Context, cfg *config.GeoIPAutoUpdate) (bool, error) {
	up, err := newUpdater(cfg)
	if err != nil {
		return false, err
	}

	return up.update(ctx, db.asn)
}
