package main

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	reNplusM  = regexp.MustCompile(`^(\d+)\s*\+\s*(\d+)\s*gratis$`)
	reNthHalf = regexp.MustCompile(`^(\d+)e\s*halve\s*prijs$`)
	reNthFree = regexp.MustCompile(`^(\d+)e\s*gratis$`)
	reXvoorY  = regexp.MustCompile(`^(\d+)\s*voor\s*(\d+[.,]\d+)$`)
	rePct     = regexp.MustCompile(`^(\d+)%\s*korting$`)
	rePctPerN = regexp.MustCompile(`^(\d+)%\s*korting\s*per\s*(\d+)\s*stuks?$`)
	reEuroOff = regexp.MustCompile(`^(\d+(?:[.,]\d+)?)\s*euro\s*korting$`)
	reVoorY   = regexp.MustCompile(`^voor\s*(\d+[.,]\d+)$`)
)

func atof(s string) float64 {
	f, _ := strconv.ParseFloat(strings.Replace(s, ",", ".", 1), 64)
	return f
}
func atoi(s string) int { i, _ := strconv.Atoi(s); return i }

// parseMechanism zet een AH-bonusomschrijving om naar de minimale aankoop
// (minQty) en de effectieve prijs per stuk bij die aankoop. fromPrice is de
// normale stukprijs. ok=false als de omschrijving onbekend is.
func parseMechanism(desc string, fromPrice float64) (int, float64, bool) {
	d := strings.ToLower(strings.TrimSpace(desc))
	if m := reNplusM.FindStringSubmatch(d); m != nil {
		n, f := atoi(m[1]), atoi(m[2])
		return n + f, fromPrice * float64(n) / float64(n+f), true
	}
	if m := reNthHalf.FindStringSubmatch(d); m != nil {
		n := atoi(m[1])
		return n, fromPrice * (float64(n) - 0.5) / float64(n), true
	}
	if m := reNthFree.FindStringSubmatch(d); m != nil {
		n := atoi(m[1])
		return n, fromPrice * float64(n-1) / float64(n), true
	}
	if m := reXvoorY.FindStringSubmatch(d); m != nil {
		n := atoi(m[1])
		return n, atof(m[2]) / float64(n), true
	}
	if m := rePctPerN.FindStringSubmatch(d); m != nil {
		return atoi(m[2]), fromPrice * (1 - atof(m[1])/100), true
	}
	if m := rePct.FindStringSubmatch(d); m != nil {
		return 1, fromPrice * (1 - atof(m[1])/100), true
	}
	if m := reEuroOff.FindStringSubmatch(d); m != nil {
		return 1, fromPrice - atof(m[1]), true
	}
	if m := reVoorY.FindStringSubmatch(d); m != nil {
		return 1, atof(m[1]), true
	}
	return 0, 0, false
}

type bonusItem struct {
	SegmentID          int     `json:"segment_id,omitempty"`
	ProductID          int     `json:"product_id,omitempty"`
	Title              string  `json:"title"`
	Category           string  `json:"category"`
	Mechanism          string  `json:"mechanism"`
	Size               string  `json:"size,omitempty"`
	PriceBefore        float64 `json:"price_before"`
	PricePromo         float64 `json:"price_promo"`
	MinQty             int     `json:"min_qty,omitempty"`
	EffectiveUnitPrice float64 `json:"effective_unit_price,omitempty"`
	PeriodStart        string  `json:"period_start"`
	PeriodEnd          string  `json:"period_end"`
}

type bonusMeta struct {
	Periods []struct {
		Start string `json:"bonusStartDate"`
		End   string `json:"bonusEndDate"`
		Tabs  []struct {
			List []struct {
				BonusType   string `json:"bonusType"`
				Description string `json:"description"`
			} `json:"urlMetadataList"`
		} `json:"tabs"`
	} `json:"periods"`
}

type bonusSection struct {
	Items []struct {
		Group *struct {
			SegmentID   int     `json:"segmentId"`
			Description string  `json:"segmentDescription"`
			Discount    string  `json:"discountDescription"`
			Category    string  `json:"category"`
			Size        string  `json:"salesUnitSize"`
			From        float64 `json:"exampleFromPrice"`
			For         float64 `json:"exampleForPrice"`
			Start       string  `json:"bonusStartDate"`
			End         string  `json:"bonusEndDate"`
		} `json:"bonusGroup"`
		Product *struct {
			ID        int     `json:"webshopId"`
			Title     string  `json:"title"`
			Size      string  `json:"salesUnitSize"`
			Before    float64 `json:"priceBeforeBonus"`
			Current   float64 `json:"currentPrice"`
			Mechanism string  `json:"bonusMechanism"`
			Category  string  `json:"mainCategory"`
			Start     string  `json:"bonusStartDate"`
			End       string  `json:"bonusEndDate"`
		} `json:"product"`
	} `json:"bonusGroupOrProducts"`
}

// bonusFor geeft de nationale Bonus die geldt op date (YYYY-MM-DD). Een
// categorie die faalt wordt overgeslagen en gemeld in skipped.
func bonusFor(ctx context.Context, date string) ([]bonusItem, []string, error) {
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return nil, nil, fmt.Errorf("date moet YYYY-MM-DD zijn: %w", err)
	}
	var meta bonusMeta
	if err := client.DoRequest(ctx, "GET", "/mobile-services/bonuspage/v3/metadata", nil, &meta); err != nil {
		return nil, nil, fmt.Errorf("bonus metadata: %w", err)
	}
	var cats []string
	for _, p := range meta.Periods {
		if date < p.Start || date > p.End {
			continue
		}
		for _, tab := range p.Tabs {
			for _, m := range tab.List {
				if m.BonusType == "NATIONAL" {
					cats = append(cats, m.Description)
				}
			}
		}
	}
	if len(cats) == 0 {
		return nil, nil, fmt.Errorf("geen bonusperiode bekend voor %s", date)
	}
	items := []bonusItem{}
	skipped := []string{}
	seen := map[string]bool{}
	var lastErr error
	succeeded := 0
	for _, cat := range cats {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		v := url.Values{}
		v.Set("application", "AHBEWEBSHOP")
		v.Set("date", date)
		v.Set("promotionType", "NATIONAL")
		v.Set("category", cat)
		var sec bonusSection
		if err := client.DoRequest(ctx, "GET", "/mobile-services/bonuspage/v2/section?"+v.Encode(), nil, &sec); err != nil {
			lastErr = err
			skipped = append(skipped, cat)
			continue
		}
		succeeded++
		for _, it := range sec.Items {
			var b bonusItem
			switch {
			case it.Group != nil:
				g := it.Group
				b = bonusItem{SegmentID: g.SegmentID, Title: g.Description, Category: g.Category, Mechanism: g.Discount,
					Size: g.Size, PriceBefore: g.From, PricePromo: g.For, PeriodStart: g.Start, PeriodEnd: g.End}
			case it.Product != nil:
				p := it.Product
				b = bonusItem{ProductID: p.ID, Title: p.Title, Category: p.Category, Mechanism: p.Mechanism,
					Size: p.Size, PriceBefore: p.Before, PricePromo: p.Current, PeriodStart: p.Start, PeriodEnd: p.End}
			default:
				continue
			}
			key := fmt.Sprintf("%d/%d/%s", b.SegmentID, b.ProductID, b.Title)
			if seen[key] {
				continue
			}
			seen[key] = true
			if q, u, ok := parseMechanism(b.Mechanism, b.PriceBefore); ok {
				b.MinQty, b.EffectiveUnitPrice = q, float64(int(u*100+0.5))/100
			}
			items = append(items, b)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if succeeded == 0 {
		return nil, nil, fmt.Errorf("alle %d bonuscategorieën faalden voor %s: %w", len(cats), date, lastErr)
	}
	return items, skipped, nil
}
