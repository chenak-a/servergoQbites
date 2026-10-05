package database

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/chenak/hackernews/graph/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	databaseName       = "cryptoDatabase"
	datastoreColl      = "datastore"
	userColl           = "usercrypto"
	openTimeField      = "Open time"
	defaultCandleLimit = 1500
	maxCandleLimit     = 5000
	stanceUnknown      = "UNKNOWN"
	connectTimeout     = 10 * time.Second
)

// ErrNotFound is returned when an asset is not registered in the datastore.
var ErrNotFound = errors.New("not found")

// botDocKey selects the single precomputed bot-state document in a bot collection.
var botDocKey = bson.M{"_key": "bot"}

// Store reads the collections the Python pipeline writes. It holds one
// connection pool for the life of the process and is safe for concurrent use.
type Store struct {
	db *mongo.Database
}

// Connect opens the shared client and verifies the server is reachable.
func Connect(ctx context.Context, uri string) (*Store, func(context.Context) error, error) {
	ctx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, nil, fmt.Errorf("connect mongo: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, nil, fmt.Errorf("ping mongo: %w", err)
	}
	return &Store{db: client.Database(databaseName)}, client.Disconnect, nil
}

// Ping reports whether the database is currently reachable.
func (s *Store) Ping(ctx context.Context) error {
	return s.db.Client().Ping(ctx, nil)
}

// loadDatastore fetches the asset-registry document for an asset.
func (s *Store) loadDatastore(ctx context.Context, name string) (datastoreDoc, error) {
	var ds datastoreDoc
	err := s.db.Collection(datastoreColl).FindOne(ctx, bson.M{"name": name}).Decode(&ds)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return ds, fmt.Errorf("asset %q: %w", name, ErrNotFound)
	}
	return ds, err
}

func decodeCandles(ctx context.Context, cursor *mongo.Cursor) ([]*model.Data, error) {
	defer cursor.Close(ctx)
	var out []*model.Data
	for cursor.Next(ctx) {
		var c candleDoc
		if err := cursor.Decode(&c); err != nil {
			return nil, err
		}
		out = append(out, c.toModel())
	}
	return out, cursor.Err()
}

// Data returns an asset's header metadata together with its most recent
// `maxCandleLimit` candles (ascending).
func (s *Store) Data(ctx context.Context, name string) (*model.Crypto, error) {
	ds, err := s.loadDatastore(ctx, name)
	if err != nil {
		return nil, err
	}
	page, err := s.Candles(ctx, name, nil, maxCandleLimit)
	if err != nil {
		return nil, err
	}
	return &model.Crypto{
		Name:       ds.Name,
		Time:       ds.TimeFrame.Label,
		Projection: ds.Projection,
		Gainlose:   ds.GainLose.toModel(),
		Data:       page.Data,
	}, nil
}

// CryptoMeta returns just the per-asset header metadata (name, timeframe,
// projection, gain/loss) without loading any candles.
func (s *Store) CryptoMeta(ctx context.Context, name string) (*model.CryptoMeta, error) {
	ds, err := s.loadDatastore(ctx, name)
	if err != nil {
		return nil, err
	}
	return &model.CryptoMeta{
		Name:       ds.Name,
		Time:       ds.TimeFrame.Label,
		Projection: ds.Projection,
		Gainlose:   ds.GainLose.toModel(),
	}, nil
}

// Candles returns a page of candles for lazy / infinite-scroll loading. With
// `before == nil` it returns the most recent `limit` bars; pass the oldest
// loaded opentime as `before` to backfill the next older page. Sort and limit
// run in Mongo on the "Open time" index the pipeline maintains.
func (s *Store) Candles(ctx context.Context, name string, before *float64, limit int) (*model.CandlePage, error) {
	ds, err := s.loadDatastore(ctx, name)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = defaultCandleLimit
	}
	limit = min(limit, maxCandleLimit)
	coll := s.db.Collection(ds.TimeFrame.CandleColl)

	var first candleDoc
	err = coll.FindOne(ctx, bson.M{},
		options.FindOne().SetSort(bson.D{{Key: openTimeField, Value: 1}})).Decode(&first)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return &model.CandlePage{Data: []*model.Data{}}, nil
	}
	if err != nil {
		return nil, err
	}

	filter := bson.M{}
	if before != nil {
		filter = bson.M{openTimeField: bson.M{"$lt": *before}}
	}
	cursor, err := coll.Find(ctx, filter, options.Find().
		SetSort(bson.D{{Key: openTimeField, Value: -1}}).
		SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	desc, err := decodeCandles(ctx, cursor)
	if err != nil {
		return nil, err
	}
	// The query is newest-first; reverse to ascending for the chart.
	data := make([]*model.Data, len(desc))
	for i := range desc {
		data[len(desc)-1-i] = desc[i]
	}
	hasMore := len(data) > 0 && data[0].Hcl.Opentime > first.OpenTime
	return &model.CandlePage{Data: data, HasMore: hasMore, Earliest: first.OpenTime}, nil
}

// Bot returns the backend-computed trading-bot state for an asset, or an
// UNKNOWN stance when the pipeline has not produced one.
func (s *Store) Bot(ctx context.Context, name string) (*model.Bot, error) {
	ds, err := s.loadDatastore(ctx, name)
	if err != nil {
		return nil, err
	}
	if ds.TimeFrame.BotColl == "" {
		return &model.Bot{Stance: stanceUnknown}, nil
	}
	var bot botDoc
	err = s.db.Collection(ds.TimeFrame.BotColl).FindOne(ctx, botDocKey).Decode(&bot)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return &model.Bot{Stance: stanceUnknown}, nil
	}
	if err != nil {
		return nil, err
	}
	return bot.toModel(), nil
}

// User returns a user's crypto balances.
func (s *Store) User(ctx context.Context, name string) (*model.User, error) {
	var u userDoc
	err := s.db.Collection(userColl).FindOne(ctx, bson.M{"UserName": name}).Decode(&u)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, fmt.Errorf("user %q: %w", name, ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	return u.toModel(), nil
}

// ListSymbols returns the distinct registered assets split by class: crypto
// pairs are quoted in USDT, everything else is an equity/ETF. Both slices are
// non-nil so the GraphQL fields marshal as [] rather than null.
func (s *Store) ListSymbols(ctx context.Context) (crypto []string, stocks []string, err error) {
	names, err := s.db.Collection(datastoreColl).Distinct(ctx, "name", bson.M{})
	if err != nil {
		return nil, nil, err
	}
	crypto, stocks = []string{}, []string{}
	for _, n := range names {
		name, ok := n.(string)
		if !ok || name == "" {
			continue
		}
		if isCrypto(name) {
			crypto = append(crypto, name)
		} else {
			stocks = append(stocks, name)
		}
	}
	sort.Strings(crypto)
	sort.Strings(stocks)
	return crypto, stocks, nil
}

func isCrypto(name string) bool {
	const quote = "USDT"
	return len(name) > len(quote) && name[len(name)-len(quote):] == quote
}
