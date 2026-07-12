package database

import (
	"context"
	"os"
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
	closeTimeField     = "Close time"
	defaultCandleLimit = 1500
	stanceUnknown      = "UNKNOWN"
	connectTimeout     = 10 * time.Second
)

// botDocKey selects the single precomputed bot-state document in a bot collection.
var botDocKey = bson.M{"_key": "bot"}

// byName is the standard filter for looking an asset up in the datastore.
func byName(name string) bson.M { return bson.M{"name": name} }

func connect() (*mongo.Client, context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		cancel()
		panic(err)
	}
	return client, ctx, cancel
}

// loadDatastore fetches the asset-registry document for an asset. A missing
// document decodes to the zero value, so callers can safely inspect it.
func loadDatastore(ctx context.Context, client *mongo.Client, name string) datastoreDoc {
	var ds datastoreDoc
	client.Database(databaseName).Collection(datastoreColl).
		FindOne(ctx, byName(name)).Decode(&ds)
	return ds
}

// Data returns an asset's header metadata together with its full candle series.
func Data(namecrypto string) *model.Crypto {
	client, ctx, cancel := connect()
	defer cancel()
	defer client.Disconnect(ctx)

	ds := loadDatastore(ctx, client, namecrypto)

	var candles []*model.Data
	cursor, err := client.Database(databaseName).Collection(ds.TimeFrame.CandleColl).Find(ctx, bson.M{})
	if err == nil {
		for cursor.Next(ctx) {
			var c candleDoc
			cursor.Decode(&c)
			candles = append(candles, c.toModel())
		}
	}

	return &model.Crypto{
		Name:       ds.Name,
		Time:       ds.TimeFrame.Label,
		Projection: ds.Projection,
		Gainlose:   ds.GainLose.toModel(),
		Data:       candles,
	}
}

// CryptoMeta returns just the per-asset header metadata (name, timeframe,
// projection, gain/loss) without loading any candles — used by the header and
// the watchlist so the UI paints instantly.
func CryptoMeta(namecrypto string) *model.CryptoMeta {
	client, ctx, cancel := connect()
	defer cancel()
	defer client.Disconnect(ctx)

	ds := loadDatastore(ctx, client, namecrypto)
	return &model.CryptoMeta{
		Name:       ds.Name,
		Time:       ds.TimeFrame.Label,
		Projection: ds.Projection,
		Gainlose:   ds.GainLose.toModel(),
	}
}

// Candles returns a time-windowed page of candles for lazy / infinite-scroll
// loading. With `before == nil` it returns the most recent `limit` bars; pass
// the oldest loaded opentime as `before` to backfill the next older page. The
// DB does the sort+limit (indexed on "Close time"), so the server never
// materialises the whole multi-year series for a single view.
func Candles(namecrypto string, before *float64, limit int) *model.CandlePage {
	empty := &model.CandlePage{Data: []*model.Data{}, HasMore: false, Earliest: 0}
	client, ctx, cancel := connect()
	defer cancel()
	defer client.Disconnect(ctx)

	ds := loadDatastore(ctx, client, namecrypto)
	if ds.TimeFrame.CandleColl == "" {
		return empty
	}
	coll := client.Database(databaseName).Collection(ds.TimeFrame.CandleColl)
	// Idempotent; keeps the descending sort+limit index-backed as history grows.
	coll.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: closeTimeField, Value: 1}}})

	if limit <= 0 {
		limit = defaultCandleLimit
	}

	// Backfill floor: opentime of the very first stored bar.
	var earliestDoc candleDoc
	coll.FindOne(ctx, bson.M{},
		options.FindOne().SetSort(bson.D{{Key: closeTimeField, Value: 1}})).Decode(&earliestDoc)
	earliest := earliestDoc.CloseTime

	filter := bson.M{}
	if before != nil {
		filter = bson.M{closeTimeField: bson.M{"$lt": *before}}
	}
	cursor, err := coll.Find(ctx, filter, options.Find().
		SetSort(bson.D{{Key: closeTimeField, Value: -1}}).
		SetLimit(int64(limit)))
	if err != nil {
		return empty
	}
	var desc []*model.Data
	for cursor.Next(ctx) {
		var c candleDoc
		cursor.Decode(&c)
		desc = append(desc, c.toModel())
	}
	// The query is newest-first; reverse to ascending for the chart.
	data := make([]*model.Data, len(desc))
	for i := range desc {
		data[len(desc)-1-i] = desc[i]
	}
	hasMore := len(data) > 0 && data[0].Hcl.Opentime > earliest
	return &model.CandlePage{Data: data, HasMore: hasMore, Earliest: earliest}
}

// Bot returns the backend-computed trading-bot state for an asset. The Python
// pipeline stores it in a dedicated collection referenced by
// datastore.TimeFrame.botid; here we just read and shape it for GraphQL.
func Bot(namecrypto string) *model.Bot {
	client, ctx, cancel := connect()
	defer cancel()
	defer client.Disconnect(ctx)

	ds := loadDatastore(ctx, client, namecrypto)
	if ds.TimeFrame.BotColl == "" {
		return &model.Bot{Stance: stanceUnknown}
	}

	var bot botDoc
	if err := client.Database(databaseName).Collection(ds.TimeFrame.BotColl).
		FindOne(ctx, botDocKey).Decode(&bot); err != nil {
		return &model.Bot{Stance: stanceUnknown}
	}
	return bot.toModel()
}

// User returns a user's crypto balances.
func User(name string) *model.User {
	client, ctx, cancel := connect()
	defer cancel()
	defer client.Disconnect(ctx)

	var u userDoc
	client.Database(databaseName).Collection(userColl).
		FindOne(ctx, bson.M{"UserName": name}).Decode(&u)
	return u.toModel()
}

// ListCrypto returns the names of every registered asset.
func ListCrypto() []string {
	client, ctx, cancel := connect()
	defer cancel()
	defer client.Disconnect(ctx)

	cursor, err := client.Database(databaseName).Collection(datastoreColl).Find(ctx, bson.M{})
	if err != nil {
		return nil
	}
	var list []string
	for cursor.Next(ctx) {
		var ds datastoreDoc
		cursor.Decode(&ds)
		list = append(list, ds.Name)
	}
	return list
}
