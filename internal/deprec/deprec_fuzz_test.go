package deprec

import (
	"math"
	"testing"
	"time"
)

// FuzzBookValue asserts the invariants for arbitrary inputs: never panics, never
// NaN/Inf for finite inputs, and for a depreciable asset the result stays within
// [salvageFloor, cost].
func FuzzBookValue(f *testing.F) {
	f.Add(1000.0, 100.0, 5, 0.4, int64(0), int64(86400*400))
	f.Add(0.0, 0.0, 0, 0.0, int64(0), int64(0))
	f.Add(-5.0, 10.0, 3, 2.0, int64(100), int64(-100))
	f.Add(1e12, -1.0, 100, 0.01, int64(1), int64(1<<40))
	f.Fuzz(func(t *testing.T, cost, salvage float64, life int, rate float64, pd, now int64) {
		if math.IsNaN(cost) || math.IsInf(cost, 0) || math.IsNaN(salvage) || math.IsInf(salvage, 0) || math.IsNaN(rate) || math.IsInf(rate, 0) {
			t.Skip()
		}
		if life > 100000 || life < -100000 {
			t.Skip()
		}
		p := time.Unix(pd, 0).UTC()
		n := time.Unix(now, 0).UTC()
		got := BookValue(cost, salvage, life, rate, p, n)
		if math.IsNaN(got) || math.IsInf(got, 0) {
			t.Fatalf("non-finite result %v", got)
		}
		if cost <= 0 || life <= 0 || p.IsZero() {
			if got != cost {
				t.Fatalf("non-depreciable must return cost: %v vs %v", got, cost)
			}
			return
		}
		floor := salvageFloor(salvage, cost)
		if got < floor-1e-9 || got > cost+1e-9 {
			t.Fatalf("result %v outside [%v,%v]", got, floor, cost)
		}
	})
}

func FuzzAnnualSchedule(f *testing.F) {
	f.Add(1000.0, 10.0, 5, 0.4)
	f.Add(0.0, 0.0, 0, 0.0)
	f.Fuzz(func(t *testing.T, cost, salvage float64, life int, rate float64) {
		if math.IsNaN(cost) || math.IsInf(cost, 0) || math.IsNaN(salvage) || math.IsInf(salvage, 0) || math.IsNaN(rate) || math.IsInf(rate, 0) {
			t.Skip()
		}
		if life > 10000 {
			t.Skip()
		}
		s := AnnualSchedule(cost, salvage, life, rate)
		if life <= 0 {
			if s != nil {
				t.Fatal("expected nil")
			}
			return
		}
		if len(s) != life+1 {
			t.Fatalf("len %d want %d", len(s), life+1)
		}
		for i := 1; i < len(s); i++ {
			if s[i] > s[i-1]+1e-9 && cost > 0 {
				t.Fatalf("schedule not monotone at %d: %v", i, s)
			}
		}
	})
}
