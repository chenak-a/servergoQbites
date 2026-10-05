// Package domain defines core business models and enums.
// Following Google Go best practices:
// - Typed enums (not magic strings/integers)
// - Immutable structs with validation
// - Clear separation from database concerns
package domain

import "time"

// SignalType represents the primary trading signal.
type SignalType string

const (
	SignalBuy   SignalType = "BUY"
	SignalSell  SignalType = "SELL"
	SignalHold  SignalType = "HOLD"
	SignalWatch SignalType = "WATCH"
)

// FromLegacyInt converts legacy integer signal to typed enum.
func (s SignalType) FromLegacyInt(val int) SignalType {
	switch val {
	case 1:
		return SignalBuy
	case 2:
		return SignalSell
	case 3:
		return SignalHold
	case 4:
		return SignalWatch
	default:
		return SignalHold
	}
}

// ToLegacyInt converts typed enum to legacy integer for backward compat.
func (s SignalType) ToLegacyInt() int {
	switch s {
	case SignalBuy:
		return 1
	case SignalSell:
		return 2
	case SignalHold:
		return 3
	case SignalWatch:
		return 4
	default:
		return 3
	}
}

// SignalStrength represents signal conviction level.
type SignalStrength string

const (
	StrengthWeak       SignalStrength = "WEAK"
	StrengthModerate   SignalStrength = "MODERATE"
	StrengthStrong     SignalStrength = "STRONG"
	StrengthVeryStrong SignalStrength = "VERY_STRONG"
)

// RegimeType represents market regime classification.
type RegimeType string

const (
	RegimeBullish RegimeType = "BULLISH"
	RegimeBearish RegimeType = "BEARISH"
	RegimeNeutral RegimeType = "NEUTRAL"
)

// AssetType represents asset classification.
type AssetType string

const (
	AssetStock  AssetType = "STOCK"
	AssetCrypto AssetType = "CRYPTO"
)

// OHLCV represents open-high-low-close-volume data.
type OHLCV struct {
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
}

// MomentumIndicators groups momentum-based technical indicators.
type MomentumIndicators struct {
	RSI           float64
	MACD          float64
	MACDSignal    float64
	MACDHistogram float64
	StochK        *float64 // Optional
	StochD        *float64 // Optional
}

// TrendIndicators groups trend-following indicators.
type TrendIndicators struct {
	AroonUp   float64
	AroonDown float64
	EMA200    float64
	SMA50     *float64 // Optional
}

// VolatilityIndicators groups volatility and risk indicators.
type VolatilityIndicators struct {
	BbandsUpper  float64
	BbandsMiddle float64
	BbandsLower  float64
	ATR          *float64 // Optional
}

// VolumeIndicators groups volume-based indicators.
type VolumeIndicators struct {
	OBV  *float64 // Optional
	VWAP *float64 // Optional
}

// Indicators is the complete set of technical indicators.
type Indicators struct {
	Momentum   MomentumIndicators
	Trend      TrendIndicators
	Volatility VolatilityIndicators
	Volume     VolumeIndicators
}

// ZScore represents z-score statistics for regime detection.
type ZScore struct {
	Value     float64
	UpperBand float64
	LowerBand float64
}

// Signals represents trading signals and regime classification.
type Signals struct {
	PrimarySignal   SignalType
	BuySellStrength SignalStrength
	Regime          RegimeType
	ZScore          ZScore
}

// CandleMeta contains metadata for shard key and routing.
type CandleMeta struct {
	Symbol          string
	Timeframe       string
	Exchange        string
	AssetType       AssetType
	PipelineVersion string
}

// AnalyzedCandle is an enriched candle with indicators and signals.
type AnalyzedCandle struct {
	OpenTime   time.Time
	Meta       CandleMeta
	OHLCV      OHLCV
	Indicators Indicators
	Signals    Signals
	MLFeatures map[string]float64
	ComputedAt time.Time
}

// BotState represents per-symbol/timeframe bot state snapshot.
type BotState struct {
	Symbol    string
	Timeframe string
	UpdatedAt time.Time
	// ... (extend as needed)
}

// Asset represents asset metadata registry.
type Asset struct {
	Symbol             string
	Name               string
	AssetType          AssetType
	Exchange           string
	Sector             *string
	MarketCap          *float64
	TrackedTimeframes  []string
	DataSourceProvider string
	DataSourceSymbol   string
	Enabled            bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
