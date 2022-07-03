package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/chenak/hackernews/graph/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func Data(namecrypto string) []*model.Crypto {
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		panic(err)
	}
	defer client.Disconnect(ctx)

	namefilter := client.Database("cryptoDatabase").Collection("datastore").FindOne(ctx, bson.M{"name": namecrypto})
	var podcast bson.M
	namefilter.Decode(&podcast)
	name := podcast["name"].(string)
	time := podcast["TimeFrame"].(primitive.M)["heur"].(string)

	//opts := options.Find().SetProjection(bson.M{"Open": 1, "_id": 0})
	//filter, _ := client.Database("cryptoDatabase").Collection("BTCUSDT4h").Find(ctx, bson.M{}, opts)

	filter, _ := client.Database("cryptoDatabase").Collection("BTCUSDT4h").Find(ctx, bson.M{})

	var modl []*model.Crypto
	for filter.Next(ctx) {
		var episode bson.M
		if err = filter.Decode(&episode); err != nil {
			log.Fatal(err)
		}
		fmt.Println(episode)

		dummyLink := model.Crypto{
			Name: name,
			Time: time,
			Data: &model.Data{Hcl: &model.Hcl{Opentime: episode["Close time"].(float64), Open: episode["Open"].(float64), High: episode["High"].(float64), Low: episode["Low"].(float64), Close: episode["Close"].(float64), Volume: episode["Volume"].(float64)}, Formula: &model.Formula{Rsi: episode["rsi"].(float64), RsiK: episode["rsiK"].(float64), RsiD: episode["rsiD"].(float64), Aroonu: episode["aroonu"].(float64), Aroond: episode["aroond"].(float64), Macd: episode["macd"].(float64), Histogram: episode["histogram"].(float64)}, Ai: &model.Ai{Bigmome: &model.Bigmome{Buy2: episode["BUY2"].(float64), Buy1: episode["BUY1"].(float64), Ambi: episode["ambi"].(float64)}, Sell: &model.Sell{Amb0: episode["amb0"].(float64), Amb1: episode["amb1"].(float64), Amb2: episode["amb2"].(float64), Amb3: episode["amb3"].(float64), Amb99: episode["amb99"].(float64)}, Smallmome: &model.Smallmome{Amo: episode["amo"].(float64), Ci: episode["ci"].(float64)}, Buy: &model.Buy{Ambb: episode["ambb"].(float64), Ambb5: episode["ambb5"].(float64), Ww6: episode["ww6"].(float64), Ww7: episode["ww7"].(float64)}, Mome: &model.Mome{Amb14: episode["amb14"].(float64), Amb15: episode["amb15"].(float64), Amb13: episode["amb13"].(float64), Amb55: episode["amb55"].(float64)}, Other: &model.Other{Amo: episode["amo"].(float64), Amo1: episode["amo1"].(float64), Buysell: episode["BUYSELL"].(float64)}}},
		}
		modl = append(modl, &dummyLink)
	}
	return modl

}
