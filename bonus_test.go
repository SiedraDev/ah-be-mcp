package main

import (
	"math"
	"testing"
)

func TestParseMechanism(t *testing.T) {
	cases := []struct {
		desc     string
		from     float64
		wantQty  int
		wantUnit float64
		wantOK   bool
	}{
		{"1 + 1 GRATIS", 3.49, 2, 1.745, true},
		{"1 + 1 gratis", 3.49, 2, 1.745, true},
		{"1 + 2 GRATIS", 3.19, 3, 1.0633, true},
		{"2 + 2 GRATIS", 1.29, 4, 0.645, true},
		{"2 + 5 GRATIS", 3.99, 7, 1.14, true},
		{"2E HALVE PRIJS", 1.99, 2, 1.4925, true},
		{"2E GRATIS", 1.69, 2, 0.845, true},
		{"2 VOOR 3.49", 2.15, 2, 1.745, true},
		{"3 VOOR 7.00", 2.99, 3, 2.3333, true},
		{"25% KORTING", 3.95, 1, 2.9625, true},
		{"30% korting", 1.99, 1, 1.393, true},
		{"10% korting per 2 stuks", 2.99, 2, 2.691, true},
		{"1 EURO KORTING", 3.99, 1, 2.99, true},
		{"VOOR 7.99", 12.99, 1, 7.99, true},
		{"Combineren mogelijk", 2.00, 0, 0, false},
	}
	for _, c := range cases {
		q, u, ok := parseMechanism(c.desc, c.from)
		if ok != c.wantOK || q != c.wantQty || math.Abs(u-c.wantUnit) > 0.001 {
			t.Errorf("%q from %.2f: got (%d, %.4f, %v), want (%d, %.4f, %v)", c.desc, c.from, q, u, ok, c.wantQty, c.wantUnit, c.wantOK)
		}
	}
}
