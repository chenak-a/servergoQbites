// Package repository defines data access interfaces.
// Following Hexagonal Architecture:
// - Domain logic depends on these interfaces (not concrete implementations)
// - MongoDB/PostgreSQL adapters implement these interfaces
// - Enables easy testing with mock repositories
package repository

import (
	"context"
	"time"

	"github.com/chenak/hackernews/internal/domain"
)

// CandleRepository provides access to analyzed candle data.
type CandleRepository interface {
	// LoadAnalyzedCandles loads analyzed candles for a given symbol/timeframe.
	LoadAnalyzedCandles(
		ctx context.Context,
		symbol string,
		timeframe string,
		startTime *time.Time,
		limit int,
	) ([]domain.AnalyzedCandle, error)

	// FindSignals finds candles matching signal criteria.
	FindSignals(
		ctx context.Context,
		signalType domain.SignalType,
		timeframe *string,
		startTime *time.Time,
		limit int,
	) ([]domain.AnalyzedCandle, error)
}

// BotStateRepository provides access to bot state snapshots.
type BotStateRepository interface {
	// LoadBotState loads bot state for a given symbol/timeframe.
	LoadBotState(
		ctx context.Context,
		symbol string,
		timeframe string,
	) (*domain.BotState, error)
}

// AssetRepository provides access to asset metadata.
type AssetRepository interface {
	// LoadAsset loads asset metadata by symbol.
	LoadAsset(ctx context.Context, symbol string) (*domain.Asset, error)

	// ListAssets lists all assets with optional filters.
	ListAssets(
		ctx context.Context,
		assetType *domain.AssetType,
		enabledOnly bool,
	) ([]domain.Asset, error)

	// ListTrackedSymbols lists symbols tracked for a given timeframe.
	ListTrackedSymbols(
		ctx context.Context,
		timeframe string,
		assetType *domain.AssetType,
	) ([]string, error)
}
