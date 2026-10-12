package database

import "testing"

func TestToModelCarriesAction(t *testing.T) {
	d := candleDoc{Action: 2, ActionConf: 0.73, ActionRef: 1790000000000}
	o := d.toModel().Ai.Other
	if o.Action != 2 || o.ACTIONconf != 0.73 || o.ACTIONref != 1790000000000 {
		t.Fatalf("action fields not mapped: %+v", o)
	}
}

func TestToModelDefaultsMissingActionToHold(t *testing.T) {
	if o := (candleDoc{}).toModel().Ai.Other; o.Action != 0 || o.ACTIONref != 0 {
		t.Fatalf("want HOLD for documents without ACTION, got %+v", o)
	}
}
