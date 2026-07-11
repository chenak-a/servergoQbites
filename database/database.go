package database

import (
	"context"
	"os"
	"time"

	"github.com/chenak/hackernews/graph/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// asFloat safely extracts a float64 from a bson value, returning 0 when the
// field is absent or not numeric (keeps resolvers from panicking during data
// migrations that add new columns).
func asFloat(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int64:
		return float64(n)
	case int32:
		return float64(n)
	default:
		return 0
	}
}

// asInt safely extracts an int from a bson value (bar indices).
func asInt(v interface{}) int {
	switch n := v.(type) {
	case int64:
		return int(n)
	case int32:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

func toFloatSlice(v interface{}) []float64 {
	arr, ok := v.(primitive.A)
	if !ok {
		return []float64{}
	}
	out := make([]float64, 0, len(arr))
	for _, e := range arr {
		out = append(out, asFloat(e))
	}
	return out
}

func toIntSlice(v interface{}) []int {
	arr, ok := v.(primitive.A)
	if !ok {
		return []int{}
	}
	out := make([]int, 0, len(arr))
	for _, e := range arr {
		out = append(out, asInt(e))
	}
	return out
}

// asString safely extracts a string, returning "" when absent/wrong type.
func asString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// candleFromDoc maps one raw MongoDB candle document to the GraphQL model,
// tolerating missing/renamed fields (asFloat/asString never panic) so a schema
// migration on the Python side can't take the API down.
func candleFromDoc(d bson.M) *model.Data {
	return &model.Data{
		Hcl: &model.Hcl{
			Opentime: asFloat(d["Close time"]), Open: asFloat(d["Open"]),
			High: asFloat(d["High"]), Low: asFloat(d["Low"]),
			Close: asFloat(d["Close"]), Volume: asFloat(d["Volume"]),
		},
		Formula: &model.Formula{
			Rsi: asFloat(d["rsi"]), RsiK: asFloat(d["rsiK"]), RsiD: asFloat(d["rsiD"]),
			Aroonu: asFloat(d["aroonu"]), Aroond: asFloat(d["aroond"]),
			Macd: asFloat(d["macd"]), Histogram: asFloat(d["histogram"]),
		},
		Ai: &model.Ai{
			Bigmome:   &model.Bigmome{Buy2: asFloat(d["BUY2"]), Buy1: asFloat(d["BUY1"]), Ambi: asFloat(d["ambi"])},
			Sell:      &model.Sell{Amb0: asFloat(d["amb0"]), Amb1: asFloat(d["amb1"]), Amb2: asFloat(d["amb2"]), Amb3: asFloat(d["amb3"]), Amb99: asFloat(d["amb99"])},
			Smallmome: &model.Smallmome{Amo: asFloat(d["amo"]), Ci: asFloat(d["ci"])},
			Buy:       &model.Buy{Ambb: asFloat(d["ambb"]), Ambb5: asFloat(d["ambb5"]), Ww6: asFloat(d["ww6"]), Ww7: asFloat(d["ww7"])},
			Mome:      &model.Mome{Amb14: asFloat(d["amb14"]), Amb15: asFloat(d["amb15"]), Amb13: asFloat(d["amb13"]), Amb55: asFloat(d["amb55"]), Amb14p: asFloat(d["amb14p"]), Amb15p: asFloat(d["amb15p"]), Amb13p: asFloat(d["amb13p"]), Amb55p: asFloat(d["amb55p"])},
			Other:     &model.Other{Amo: asFloat(d["amo"]), Amo1: asFloat(d["amo1"]), Buysell: asFloat(d["BUYSELL"]), BUYSELLevel: asString(d["BUYSELLTYPE"])},
		},
	}
}

func connect() (*mongo.Client, context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
func Data(namecrypto string) *model.Crypto {

	client, ctx, cancel := connect()
	defer cancel()
	namefilter := client.Database("cryptoDatabase").Collection("datastore").FindOne(ctx, bson.M{"name": namecrypto})
	var podcast bson.M
	namefilter.Decode(&podcast)
	tf, _ := podcast["TimeFrame"].(primitive.M)
	gl, _ := podcast["gainlose"].(primitive.M)
	name := asString(podcast["name"])
	time := asString(tf["heur"])
	Projection := asFloat(podcast["projection"])
	gainlose24h := asFloat(gl["24h"])
	gainlose1w := asFloat(gl["1w"])
	gainlose1M := asFloat(gl["1M"])

	datain := asString(tf["dataid"])
	filter, _ := client.Database("cryptoDatabase").Collection(datain).Find(ctx, bson.M{})

	var modl []*model.Data
	for filter.Next(ctx) {
		var episode bson.M
		filter.Decode(&episode)
		modl = append(modl, candleFromDoc(episode))
	}
	defer client.Disconnect(ctx)
	return &model.Crypto{Name: name, Time: time, Projection: Projection, Gainlose: &model.GainLoss{M: gainlose1M, D: gainlose24h, W: gainlose1w}, Data: modl}

}

// Bot returns the backend-computed trading-bot state for an asset. The Python
// pipeline stores it in a dedicated collection referenced by
// datastore.TimeFrame.botid; here we just read and shape it for GraphQL.
func Bot(namecrypto string) *model.Bot {
	client, ctx, cancel := connect()
	defer cancel()
	defer client.Disconnect(ctx)

	var ds bson.M
	client.Database("cryptoDatabase").Collection("datastore").
		FindOne(ctx, bson.M{"name": namecrypto}).Decode(&ds)

	tf, _ := ds["TimeFrame"].(primitive.M)
	botID, _ := tf["botid"].(string)
	if botID == "" {
		return &model.Bot{Stance: "UNKNOWN"}
	}

	var doc bson.M
	client.Database("cryptoDatabase").Collection(botID).
		FindOne(ctx, bson.M{"_key": "bot"}).Decode(&doc)
	if doc == nil {
		return &model.Bot{Stance: "UNKNOWN"}
	}

	stance, _ := doc["stance"].(string)
	return &model.Bot{
		Stance:     stance,
		BotMult:    asFloat(doc["botMult"]),
		BhMult:     asFloat(doc["bhMult"]),
		EmaGapPct:  asFloat(doc["emaGapPct"]),
		StartIdx:   asInt(doc["startIdx"]),
		T:          toFloatSlice(doc["t"]),
		Close:      toFloatSlice(doc["close"]),
		Ema:        toFloatSlice(doc["ema"]),
		DeepBreak:  toFloatSlice(doc["deepBreak"]),
		InMarket:   toFloatSlice(doc["inMarket"]),
		Equity:     toFloatSlice(doc["equity"]),
		Hold:       toFloatSlice(doc["hold"]),
		Entries:    toIntSlice(doc["entries"]),
		Trims:      toIntSlice(doc["trims"]),
		Exits:      toIntSlice(doc["exits"]),
		SwingBuys:  toIntSlice(doc["swingBuys"]),
		SwingSells: toIntSlice(doc["swingSells"]),
	}
}

func User(name string) *model.User {
	client, ctx, cancel := connect()
	defer cancel()
	namefilter := client.Database("cryptoDatabase").Collection("usercrypto").FindOne(ctx, bson.M{"UserName": name})
	var podcast bson.M
	namefilter.Decode(&podcast)
	nameuser := podcast["UserName"].(string)
	var modeuser []*model.Balance
	for key, value := range podcast["balance"].(primitive.M) {
		balancetype := &model.Balance{CryptoName: key, Totale: value.(float64)}
		modeuser = append(modeuser, balancetype)
	}
	defer client.Disconnect(ctx)
	return &model.User{Nameuser: nameuser, Balance: modeuser}
}
func ListCrypto() []string {
	client, ctx, cancel := connect()
	defer cancel()
	namefilterlist, _ := client.Database("cryptoDatabase").Collection("datastore").Find(ctx, bson.M{})
	var list []string
	for namefilterlist.Next(ctx) {
		var episode bson.M
		namefilterlist.Decode(&episode)
		list = append(list, episode["name"].(string))
	}
	defer client.Disconnect(ctx)
	return list
}
