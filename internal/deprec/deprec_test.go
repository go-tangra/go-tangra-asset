package deprec

import (
	"math"
	"testing"
	"time"
)

var (
	bought = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
)

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestBookValue_NonDepreciable(t *testing.T) {
	now := bought.AddDate(2, 0, 0)
	cases := []struct {
		name string
		cost float64
		life int
		pd   time.Time
	}{
		{"zero cost", 0, 5, bought},
		{"negative cost", -10, 5, bought},
		{"no life", 1000, 0, bought},
		{"negative life", 1000, -1, bought},
		{"no purchase date", 1000, 5, time.Time{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := BookValue(c.cost, 100, c.life, 0.4, c.pd, now); got != c.cost {
				t.Fatalf("got %v want cost %v", got, c.cost)
			}
		})
	}
}

func TestBookValue_BrandNewReadsCost(t *testing.T) {
	if got := BookValue(1000, 100, 5, 0.4, bought, bought); !approx(got, 1000) {
		t.Fatalf("at purchase: %v", got)
	}
	if got := BookValue(1000, 100, 5, 0.4, bought, bought.Add(-time.Hour)); !approx(got, 1000) {
		t.Fatalf("before purchase: %v", got)
	}
}

func TestBookValue_DDBCurve(t *testing.T) {
	// One full year at 40%: 1000 * 0.6 = 600. Two years: 360. Three: 216, floored at 250.
	y1 := bought.Add(time.Duration(yearSeconds * float64(time.Second)))
	if got := BookValue(1000, 0, 5, 0.4, bought, y1); !approx(got, 600) {
		t.Fatalf("y1: %v", got)
	}
	y2 := bought.Add(2 * time.Duration(yearSeconds*float64(time.Second)))
	if got := BookValue(1000, 0, 5, 0.4, bought, y2); !approx(got, 360) {
		t.Fatalf("y2: %v", got)
	}
	y3 := bought.Add(3 * time.Duration(yearSeconds*float64(time.Second)))
	if got := BookValue(1000, 250, 5, 0.4, bought, y3); !approx(got, 250) {
		t.Fatalf("y3 floored: %v", got)
	}
}

func TestBookValue_PastUsefulLifeSitsAtSalvage(t *testing.T) {
	now := bought.AddDate(6, 0, 0)
	if got := BookValue(1000, 120, 5, 0.4, bought, now); got != 120 {
		t.Fatalf("got %v want salvage 120", got)
	}
	if got := BookValue(1000, 0, 5, 0.4, bought, now); got != 0 {
		t.Fatalf("no salvage: got %v want 0", got)
	}
}

func TestBookValue_RateHandling(t *testing.T) {
	now := bought.AddDate(1, 0, 0)
	def := BookValue(1000, 0, 5, 0, bought, now)
	exp := BookValue(1000, 0, 5, DefaultRate, bought, now)
	if !approx(def, exp) {
		t.Fatalf("default rate: %v vs %v", def, exp)
	}
	if got := BookValue(1000, 50, 5, 7, bought, now); got != 50 {
		t.Fatalf("rate clamped to 1 → salvage floor: %v", got)
	}
}

func TestBookValue_SalvageNeverAboveCostOrBelowZero(t *testing.T) {
	now := bought.AddDate(10, 0, 0)
	if got := BookValue(500, 900, 3, 0.4, bought, now); got != 500 {
		t.Fatalf("salvage above cost clamps to cost: %v", got)
	}
	if got := BookValue(500, -900, 3, 0.4, bought, now); got != 0 {
		t.Fatalf("negative salvage clamps to zero: %v", got)
	}
}

func TestAnnualSchedule(t *testing.T) {
	if AnnualSchedule(1000, 0, 0, 0.4) != nil {
		t.Fatal("no life → nil")
	}
	s := AnnualSchedule(1000, 100, 3, 0.5)
	want := []float64{1000, 500, 250, 100}
	if len(s) != len(want) {
		t.Fatalf("len %d", len(s))
	}
	for i := range want {
		if !approx(s[i], want[i]) {
			t.Fatalf("year %d: %v want %v", i, s[i], want[i])
		}
	}
	// default + clamped rate paths
	if d := AnnualSchedule(100, 0, 2, 0); !approx(d[1], 60) {
		t.Fatalf("default rate schedule: %v", d)
	}
	if c := AnnualSchedule(100, 10, 2, 3); c[1] != 10 || c[2] != 10 {
		t.Fatalf("clamped rate schedule: %v", c)
	}
}
