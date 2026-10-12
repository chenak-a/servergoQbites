package database

import "github.com/chenak/hackernews/graph/model"

// Typed BSON views over the Mongo documents. Decoding into these structs (via
// the driver's bson tags) keeps the rest of the package from reaching into
// bson.M with stringly-typed keys, and the driver already tolerates
// missing/renamed fields by leaving zero values — so a Python-side schema
// migration can't panic the API.

// timeFrame routes an asset to its candle/bot collections and carries its label.
type timeFrame struct {
	Label      string `bson:"heur"`
	CandleColl string `bson:"dataid"`
	BotColl    string `bson:"botid"`
}

// gainLose holds the trailing return figures shown in the header/watchlist.
type gainLose struct {
	Day   float64 `bson:"24h"`
	Week  float64 `bson:"1w"`
	Month float64 `bson:"1M"`
}

func (g gainLose) toModel() *model.GainLoss {
	return &model.GainLoss{M: g.Month, W: g.Week, D: g.Day}
}

// datastoreDoc is a document in the `datastore` collection (the asset registry).
type datastoreDoc struct {
	Name       string    `bson:"name"`
	Projection float64   `bson:"projection"`
	TimeFrame  timeFrame `bson:"TimeFrame"`
	GainLose   gainLose  `bson:"gainlose"`
}

// candleDoc is one OHLCV bar plus the indicators and signal the API exposes.
// Indicator families are grouped into their own typed blocks rather than a flat
// list of floats. Mongo stores the columns flat, so each block is decoded with
// `,inline` (the driver flattens its bson keys into the parent document).
type rsiBlock struct {
	Value float64 `bson:"rsi"`
	K     float64 `bson:"rsiK"`
	D     float64 `bson:"rsiD"`
}

type aroonBlock struct {
	Up   float64 `bson:"aroonu"`
	Down float64 `bson:"aroond"`
}

type macdBlock struct {
	Line      float64 `bson:"macd"`
	Histogram float64 `bson:"histogram"`
}

// zscoreBlock is the 30-day causal z-score plus its Gaussian mean-reversion
// bands in price space (mid=μ, hi=μ+2σ, lo=μ-2σ).
type zscoreBlock struct {
	Value float64 `bson:"z"`
	Mid   float64 `bson:"zMid"`
	Hi    float64 `bson:"zHi"`
	Lo    float64 `bson:"zLo"`
}

type candleDoc struct {
	OpenTime    float64     `bson:"Open time"`
	Open        float64     `bson:"Open"`
	High        float64     `bson:"High"`
	Low         float64     `bson:"Low"`
	Close       float64     `bson:"Close"`
	Volume      float64     `bson:"Volume"`
	Rsi         rsiBlock    `bson:",inline"`
	Aroon       aroonBlock  `bson:",inline"`
	Macd        macdBlock   `bson:",inline"`
	Z           zscoreBlock `bson:",inline"`
	Ema         float64     `bson:"ema"`
	Signal      float64     `bson:"BUYSELL"`
	SignalLabel string      `bson:"BUYSELLTYPE"`
	Confidence  float64     `bson:"BUYSELLCONF"`
	Action      float64     `bson:"ACTION"`
	ActionConf  float64     `bson:"ACTIONCONF"`
	ActionRef   float64     `bson:"ACTIONREF"`
}

func (c candleDoc) toModel() *model.Data {
	return &model.Data{
		Hcl: &model.Hcl{
			Opentime: c.OpenTime, Open: c.Open, High: c.High,
			Low: c.Low, Close: c.Close, Volume: c.Volume,
		},
		Formula: &model.Formula{
			Rsi:   &model.Rsi{Value: c.Rsi.Value, K: c.Rsi.K, D: c.Rsi.D},
			Aroon: &model.Aroon{Up: c.Aroon.Up, Down: c.Aroon.Down},
			Macd:  &model.Macd{Line: c.Macd.Line, Histogram: c.Macd.Histogram},
			Z:     &model.Zscore{Value: c.Z.Value, Mid: c.Z.Mid, Hi: c.Z.Hi, Lo: c.Z.Lo},
			Ema:   c.Ema,
		},
		// The DNN ensemble columns (amb*, BUY*, ww*, ...) were retired; only the
		// BUYSELL signal and its label remain, exposed under Ai.Other.
		Ai: &model.Ai{
			Bigmome:   &model.Bigmome{},
			Sell:      &model.Sell{},
			Smallmome: &model.Smallmome{},
			Buy:       &model.Buy{},
			Mome:      &model.Mome{},
			Other: &model.Other{
				Buysell: c.Signal, BUYSELLevel: c.SignalLabel, BUYSELLconf: c.Confidence,
				Action: c.Action, ACTIONconf: c.ActionConf, ACTIONref: c.ActionRef,
			},
		},
	}
}

// botDoc is the precomputed bot state stored under {_key:"bot"}. Bar-index and
// timestamp arrays are stored as integers, so they are typed as such here and
// widened to float64 where the GraphQL model expects it.
type botDoc struct {
	Stance     string    `bson:"stance"`
	BotMult    float64   `bson:"botMult"`
	BhMult     float64   `bson:"bhMult"`
	EmaGapPct  float64   `bson:"emaGapPct"`
	StartIdx   int       `bson:"startIdx"`
	Time       []int64   `bson:"t"`
	Close      []float64 `bson:"close"`
	Ema        []float64 `bson:"ema"`
	DeepBreak  []float64 `bson:"deepBreak"`
	InMarket   []float64 `bson:"inMarket"`
	Equity     []float64 `bson:"equity"`
	Hold       []float64 `bson:"hold"`
	Entries    []int     `bson:"entries"`
	Trims      []int     `bson:"trims"`
	Exits      []int     `bson:"exits"`
	SwingBuys  []int     `bson:"swingBuys"`
	SwingSells []int     `bson:"swingSells"`
	CyclePhase string    `bson:"cyclePhase"`
	CycleDay   int       `bson:"cycleDay"`
}

func (b botDoc) toModel() *model.Bot {
	return &model.Bot{
		Stance:     b.Stance,
		BotMult:    b.BotMult,
		BhMult:     b.BhMult,
		EmaGapPct:  b.EmaGapPct,
		StartIdx:   b.StartIdx,
		T:          int64sToFloats(b.Time),
		Close:      b.Close,
		Ema:        b.Ema,
		DeepBreak:  b.DeepBreak,
		InMarket:   b.InMarket,
		Equity:     b.Equity,
		Hold:       b.Hold,
		Entries:    b.Entries,
		Trims:      b.Trims,
		Exits:      b.Exits,
		SwingBuys:  b.SwingBuys,
		SwingSells: b.SwingSells,
		CyclePhase: b.CyclePhase,
		CycleDay:   b.CycleDay,
	}
}

// userDoc mirrors a document in the `usercrypto` collection.
type userDoc struct {
	UserName string             `bson:"UserName"`
	Balance  map[string]float64 `bson:"balance"`
}

func (u userDoc) toModel() *model.User {
	balances := make([]*model.Balance, 0, len(u.Balance))
	for name, total := range u.Balance {
		balances = append(balances, &model.Balance{CryptoName: name, Totale: total})
	}
	return &model.User{Nameuser: u.UserName, Balance: balances}
}

func int64sToFloats(in []int64) []float64 {
	out := make([]float64, len(in))
	for i, v := range in {
		out[i] = float64(v)
	}
	return out
}
