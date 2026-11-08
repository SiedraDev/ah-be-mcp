package main

import (
	"context"
	"fmt"
	"time"
	_ "time/tzdata" // embed zone data: no panic without system tzdata
)

const slotsQuery = `query FetchDeliveryOrderSlotDays($address: MemberAddressInput!) {
  orderDeliverySlots(address: $address) {
    date isFullyBooked
    slots { startTime endTime isFullyBooked serviceCharge { price { amount } } }
  }
}`

var brussels = func() *time.Location {
	l, err := time.LoadLocation("Europe/Brussels")
	if err != nil {
		return time.FixedZone("CET", 3600) // fallback; tzdata is embedded so this should not happen
	}
	return l
}()

type deliverySlot struct {
	Date  string  `json:"date"`
	Start string  `json:"start"`
	End   string  `json:"end"`
	Price float64 `json:"price"`
	Full  bool    `json:"full"`
}

func localHM(iso string) (date, hm string, err error) {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return "", "", err
	}
	t = t.In(brussels)
	return t.Format("2006-01-02"), t.Format("15:04"), nil
}

func deliverySlots(ctx context.Context) ([]deliverySlot, error) {
	m, err := client.GetMember(ctx)
	if err != nil {
		return nil, fmt.Errorf("profiel: %w", err)
	}
	a := m.Address
	addr := map[string]any{"street": a.Street, "houseNumber": a.HouseNumber, "postalCode": a.PostalCode, "city": a.City, "countryCode": a.CountryCode}
	if a.HouseNumberExtra != "" {
		addr["houseNumberExtra"] = a.HouseNumberExtra
	}
	var res struct {
		Days []struct {
			Slots []struct {
				Start string `json:"startTime"`
				End   string `json:"endTime"`
				Full  bool   `json:"isFullyBooked"`
				Fee   struct {
					Price struct {
						Amount float64 `json:"amount"`
					} `json:"price"`
				} `json:"serviceCharge"`
			} `json:"slots"`
		} `json:"orderDeliverySlots"`
	}
	if err := client.DoGraphQL(ctx, slotsQuery, map[string]any{"address": addr}, &res); err != nil {
		return nil, fmt.Errorf("leveringsmomenten: %w", err)
	}
	out := []deliverySlot{}
	for _, d := range res.Days {
		for _, s := range d.Slots {
			date, start, err := localHM(s.Start)
			if err != nil {
				return nil, fmt.Errorf("leveringsmoment met ongeldige starttijd %q: %w", s.Start, err)
			}
			_, end, err := localHM(s.End)
			if err != nil {
				return nil, fmt.Errorf("leveringsmoment met ongeldige eindtijd %q: %w", s.End, err)
			}
			out = append(out, deliverySlot{Date: date, Start: start, End: end, Price: s.Fee.Price.Amount, Full: s.Full})
		}
	}
	return out, nil
}

type openOrder struct {
	OrderID      int     `json:"order_id"`
	Status       string  `json:"status"`
	ShoppingType string  `json:"shopping_type"`
	Date         string  `json:"date"`
	Start        string  `json:"start"`
	End          string  `json:"end"`
	Modifiable   bool    `json:"modifiable"`
	Total        float64 `json:"total"`
}

func openOrders(ctx context.Context) ([]openOrder, error) {
	fs, err := client.GetFulfillments(ctx)
	if err != nil {
		return nil, fmt.Errorf("bestellingen: %w", err)
	}
	out := make([]openOrder, 0, len(fs))
	for _, f := range fs {
		out = append(out, openOrder{OrderID: f.OrderID, Status: f.Status, ShoppingType: f.ShoppingType,
			Date: f.Delivery.Slot.Date, Start: f.Delivery.Slot.StartTime, End: f.Delivery.Slot.EndTime,
			Modifiable: f.Modifiable, Total: f.TotalPrice})
	}
	return out, nil
}
