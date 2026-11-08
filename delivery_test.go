package main

import (
	"encoding/json"
	"testing"
)

func TestDeliverySlotsLocalTime(t *testing.T) {
	s, _ := setup(t, true)
	text, isErr := call(t, s, "ah_get_delivery_slots", nil)
	if isErr {
		t.Fatalf("error: %s", text)
	}
	var out struct {
		Slots []deliverySlot `json:"slots"`
	}
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Slots) != 2 {
		t.Fatalf("slots: %+v", out.Slots)
	}
	got := out.Slots[0]
	if got.Date != "2026-11-17" || got.Start != "16:00" || got.End != "22:00" || got.Price != 5.99 || got.Full {
		t.Fatalf("slot 0: %+v", got)
	}
	if !out.Slots[1].Full {
		t.Fatalf("slot 1 should be full: %+v", out.Slots[1])
	}
}

func TestOrdersOpen(t *testing.T) {
	s, _ := setup(t, true)
	text, isErr := call(t, s, "ah_get_orders_open", nil)
	if isErr {
		t.Fatalf("error: %s", text)
	}
	var out struct {
		Orders []openOrder `json:"orders"`
	}
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Orders) != 1 || out.Orders[0].Date != "2026-11-17" || out.Orders[0].Start != "16:00" || !out.Orders[0].Modifiable {
		t.Fatalf("orders: %+v", out.Orders)
	}
	if o := out.Orders[0]; o.OrderID != 42 || o.Status != "PLANNED" || o.Total != 52.1 || o.End != "22:00" {
		t.Fatalf("order fields: %+v", o)
	}
}

func slotsJSON(start, end string) string {
	return `{"data":{"orderDeliverySlots":[{"slots":[{"startTime":"` + start + `","endTime":"` + end + `","isFullyBooked":false,"serviceCharge":{"price":{"amount":5.99}}}]}]}}`
}

func TestDeliverySlotsLocalDateRollover(t *testing.T) {
	s, f := setup(t, true)
	f.slotsJSON = slotsJSON("2026-11-17T23:30:00.000Z", "2026-11-18T01:30:00.000Z")
	text, isErr := call(t, s, "ah_get_delivery_slots", nil)
	if isErr {
		t.Fatalf("error: %s", text)
	}
	var out struct {
		Slots []deliverySlot `json:"slots"`
	}
	if err := json.Unmarshal([]byte(text), &out); err != nil || len(out.Slots) != 1 {
		t.Fatalf("%v %s", err, text)
	}
	if g := out.Slots[0]; g.Date != "2026-11-18" || g.Start != "00:30" || g.End != "02:30" {
		t.Fatalf("slot: %+v", g)
	}
}

func TestDeliverySlotsMalformedTimeIsError(t *testing.T) {
	for _, c := range [][2]string{{"niet-een-tijd", "2026-11-17T21:00:00.000Z"}, {"2026-11-17T15:00:00.000Z", "kapot"}} {
		s, f := setup(t, true)
		f.slotsJSON = slotsJSON(c[0], c[1])
		text, isErr := call(t, s, "ah_get_delivery_slots", nil)
		if !isErr {
			t.Fatalf("want error for %v, got %s", c, text)
		}
	}
}
