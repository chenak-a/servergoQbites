// Package mongodb implements repository interfaces for MongoDB.
// Following Google Go best practices:
// - Clear separation of domain models and BSON documents
// - Type-safe conversions via explicit serializers
// - Context-aware operations (cancellation, timeouts)
package mongodb

import (
	"context"
	"time"

	"github.com/chenak/hackernews/internal/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	collectionAnalyzedCandles = "analyzed_candles"
	collectionBotState        = "bot_state"
	collectionAssets          = "assets"
	collectionDatastore       = "datastore" // Legacy
)

// CandleRepo implements repository.CandleRepository for MongoDB.
type CandleRepo struct {
	db *mongo.Database
}

// NewCandleRepo creates a new MongoDB candle repository.
func NewCandleRepo(db *mongo.Database) *CandleRepo {
	return &CandleRepo{db: db}
}

// LoadAnalyzedCandles loads analyzed candles from MongoDB (new schema).
// Falls back to legacy schema if new schema has no data.
func (r *CandleRepo) LoadAnalyzedCandles(
	ctx context.Context,
	symbol string,
	timeframe string,
	startTime *time.Time,
	limit int,
) ([]domain.AnalyzedCandle, error) {
	// Try new schema first
	candles, err := r.loadFromNewSchema(ctx, symbol, timeframe, startTime, limit)
	if err == nil && len(candles) > 0 {
		return candles, nil
	}

	// Fallback to legacy schema
	return r.loadFromLegacySchema(ctx, symbol, timeframe, startTime, limit)
}

// loadFromNewSchema queries the new analyzed_candles collection.
func (r *CandleRepo) loadFromNewSchema(
	ctx context.Context,
	symbol string,
	timeframe string,
	startTime *time.Time,
	limit int,
) ([]domain.AnalyzedCandle, error) {
	coll := r.db.Collection(collectionAnalyzedCandles)

	filter := bson.M{
		"meta.symbol":    symbol,
		"meta.timeframe": timeframe,
	}

	if startTime != nil {
		filter["open_time"] = bson.M{"$gte": *startTime}
	}

	opts := options.Find().SetSort(bson.D{{Key: "open_time", Value: 1}})
	if limit > 0 {
		opts.SetLimit(int64(limit))
	}

	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var candles []domain.AnalyzedCandle
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		candle := deserializeAnalyzedCandle(doc)
		candles = append(candles, candle)
	}

	return candles, nil
}

// loadFromLegacySchema queries the old {symbol}{timeframe} collections.
func (r *CandleRepo) loadFromLegacySchema(
	ctx context.Context,
	symbol string,
	timeframe string,
	startTime *time.Time,
	limit int,
) ([]domain.AnalyzedCandle, error) {
	// First, find collection name from datastore
	var ds struct {
		TimeFrame struct {
			DataID string `bson:"dataid"`
		} `bson:"TimeFrame"`
	}

	err := r.db.Collection(collectionDatastore).FindOne(
		ctx,
		bson.M{"name": symbol, "TimeFrame.heur": timeframe},
	).Decode(&ds)

	if err != nil || ds.TimeFrame.DataID == "" {
		return nil, err
	}

	// Query legacy collection
	coll := r.db.Collection(ds.TimeFrame.DataID)
	filter := bson.M{}

	if startTime != nil {
		filter["Open time"] = bson.M{"$gte": startTime.UnixMilli()}
	}

	opts := options.Find().SetSort(bson.D{{Key: "Open time", Value: 1}})
	if limit > 0 {
		opts.SetLimit(int64(limit))
	}

	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var candles []domain.AnalyzedCandle
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		candle := deserializeLegacyCandle(doc, symbol, timeframe)
		candles = append(candles, candle)
	}

	return candles, nil
}

// FindSignals finds candles with matching signal type.
func (r *CandleRepo) FindSignals(
	ctx context.Context,
	signalType domain.SignalType,
	timeframe *string,
	startTime *time.Time,
	limit int,
) ([]domain.AnalyzedCandle, error) {
	coll := r.db.Collection(collectionAnalyzedCandles)

	filter := bson.M{"signals.primary_signal": string(signalType)}

	if timeframe != nil {
		filter["meta.timeframe"] = *timeframe
	}

	if startTime != nil {
		filter["open_time"] = bson.M{"$gte": *startTime}
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "open_time", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var candles []domain.AnalyzedCandle
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		candle := deserializeAnalyzedCandle(doc)
		candles = append(candles, candle)
	}

	return candles, nil
}

// deserializeAnalyzedCandle converts a MongoDB document (new schema) to domain model.
func deserializeAnalyzedCandle(doc bson.M) domain.AnalyzedCandle {
	meta := doc["meta"].(bson.M)
	ohlcv := doc["ohlcv"].(bson.M)
	indicators := doc["indicators"].(bson.M)
	signals := doc["signals"].(bson.M)

	momentum := indicators["momentum"].(bson.M)
	trend := indicators["trend"].(bson.M)
	volatility := indicators["volatility"].(bson.M)
	volume := indicators["volume"].(bson.M)

	zscore := signals["zscore"].(bson.M)

	return domain.AnalyzedCandle{
		OpenTime: doc["open_time"].(time.Time),
		Meta: domain.CandleMeta{
			Symbol:          getString(meta, "symbol"),
			Timeframe:       getString(meta, "timeframe"),
			Exchange:        getString(meta, "exchange"),
			AssetType:       domain.AssetType(getString(meta, "type")),
			PipelineVersion: getString(meta, "pipeline_version"),
		},
		OHLCV: domain.OHLCV{
			Open:   getFloat(ohlcv, "open"),
			High:   getFloat(ohlcv, "high"),
			Low:    getFloat(ohlcv, "low"),
			Close:  getFloat(ohlcv, "close"),
			Volume: getFloat(ohlcv, "volume"),
		},
		Indicators: domain.Indicators{
			Momentum: domain.MomentumIndicators{
				RSI:           getFloat(momentum, "rsi"),
				MACD:          getFloat(momentum, "macd"),
				MACDSignal:    getFloat(momentum, "macd_signal"),
				MACDHistogram: getFloat(momentum, "macd_histogram"),
			},
			Trend: domain.TrendIndicators{
				AroonUp:   getFloat(trend, "aroon_up"),
				AroonDown: getFloat(trend, "aroon_down"),
				EMA200:    getFloat(trend, "ema_200"),
			},
			Volatility: domain.VolatilityIndicators{
				BbandsUpper:  getFloat(volatility, "bbands_upper"),
				BbandsMiddle: getFloat(volatility, "bbands_middle"),
				BbandsLower:  getFloat(volatility, "bbands_lower"),
			},
			Volume: domain.VolumeIndicators{},
		},
		Signals: domain.Signals{
			PrimarySignal:   domain.SignalType(getString(signals, "primary_signal")),
			BuySellStrength: domain.SignalStrength(getString(signals, "buy_sell_strength")),
			Regime:          domain.RegimeType(getString(signals, "regime")),
			ZScore: domain.ZScore{
				Value:     getFloat(zscore, "value"),
				UpperBand: getFloat(zscore, "upper_band"),
				LowerBand: getFloat(zscore, "lower_band"),
			},
		},
		ComputedAt: doc["computed_at"].(time.Time),
	}
}

// deserializeLegacyCandle converts a legacy MongoDB document to domain model.
func deserializeLegacyCandle(doc bson.M, symbol, timeframe string) domain.AnalyzedCandle {
	openTime := time.UnixMilli(int64(getFloat(doc, "Open time")))

	// Convert legacy integer signal to enum
	signalInt := int(getFloat(doc, "ai.SIGNAL"))
	signal := domain.SignalType("").FromLegacyInt(signalInt)

	return domain.AnalyzedCandle{
		OpenTime: openTime,
		Meta: domain.CandleMeta{
			Symbol:    symbol,
			Timeframe: timeframe,
			Exchange:  "UNKNOWN",
			AssetType: domain.AssetStock, // Default
		},
		OHLCV: domain.OHLCV{
			Open:   getFloat(doc, "Open"),
			High:   getFloat(doc, "High"),
			Low:    getFloat(doc, "Low"),
			Close:  getFloat(doc, "Close"),
			Volume: getFloat(doc, "Volume"),
		},
		Indicators: domain.Indicators{
			Momentum: domain.MomentumIndicators{
				RSI:           getFloat(doc, "formula.rsi"),
				MACD:          getFloat(doc, "formula.macd"),
				MACDSignal:    getFloat(doc, "formula.macd_signal"),
				MACDHistogram: getFloat(doc, "formula.macd_histogram"),
			},
			Trend: domain.TrendIndicators{
				AroonUp:   getFloat(doc, "formula.aroonu"),
				AroonDown: getFloat(doc, "formula.aroond"),
				EMA200:    getFloat(doc, "ema"),
			},
			Volatility: domain.VolatilityIndicators{},
			Volume:     domain.VolumeIndicators{},
		},
		Signals: domain.Signals{
			PrimarySignal: signal,
			ZScore: domain.ZScore{
				Value: getFloat(doc, "ai.other.zscore"),
			},
		},
	}
}

// Helper functions to safely extract values from bson.M
func getString(m bson.M, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getFloat(m bson.M, key string) float64 {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case float64:
			return val
		case int:
			return float64(val)
		case int32:
			return float64(val)
		case int64:
			return float64(val)
		}
	}
	return 0.0
}
