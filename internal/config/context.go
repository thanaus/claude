package config

import "context"

type natsConfigContextKey struct{}

// WithNATSConfig stores a resolved NATS configuration in the context.
func WithNATSConfig(ctx context.Context, cfg NATSConfig) context.Context {
	return context.WithValue(ctx, natsConfigContextKey{}, cfg)
}

// NATSConfigFromContext returns the resolved NATS configuration from the context.
func NATSConfigFromContext(ctx context.Context) (NATSConfig, bool) {
	cfg, ok := ctx.Value(natsConfigContextKey{}).(NATSConfig)
	return cfg, ok
}

// SyncPaths holds the resolved source and destination directories for a sync job.
type SyncPaths struct {
	Source      string
	Destination string
}

type syncPathsContextKey struct{}

// WithSyncPaths stores resolved sync paths in the context.
func WithSyncPaths(ctx context.Context, paths SyncPaths) context.Context {
	return context.WithValue(ctx, syncPathsContextKey{}, paths)
}

// SyncPathsFromContext returns the resolved sync paths from the context.
func SyncPathsFromContext(ctx context.Context) (SyncPaths, bool) {
	paths, ok := ctx.Value(syncPathsContextKey{}).(SyncPaths)
	return paths, ok
}
