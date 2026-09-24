// Package deprec is a PURE depreciation library: no I/O, no clock, no globals —
// every result is a deterministic function of its arguments. It implements
// double-declining-balance (DDB) book value for an asset and the per-year
// schedule behind it. The asset service wires BookValue into the asset read path
// (Asset.BookValue) and the tenant statistics rollup (total depreciated value).
package deprec

import (
	"math"
	"time"
)

// DefaultRate is the DDB rate applied when the caller supplies a non-positive
// rate. 0.40 is a common straight-line-doubled rate for a 5-year life.
const DefaultRate = 0.40

const yearSeconds = 365.25 * 24 * 3600

// BookValue returns the depreciated book value of an asset under
// double-declining-balance.
//
// Rules:
//   - A non-depreciable asset (cost<=0, usefulLifeYears<=0 or a zero
//     purchaseDate) returns cost unchanged.
//   - A non-positive rate defaults to DefaultRate; a rate above 1 is clamped to
//     1 (full first-period write-down).
//   - Elapsed time is measured in fractional years from purchaseDate to now, and
//     is 0 when now is at or before purchaseDate (a brand-new asset reads ~cost).
//   - The value is floored at the salvage value (never below max(salvage,0), and
//     never above cost); once the asset is at or past its useful life it sits at
//     that salvage floor.
func BookValue(cost, salvage float64, usefulLifeYears int, rate float64, purchaseDate, now time.Time) float64 {
	if cost <= 0 || usefulLifeYears <= 0 || purchaseDate.IsZero() {
		return cost
	}
	if rate <= 0 {
		rate = DefaultRate
	}
	if rate > 1 {
		rate = 1
	}
	floor := salvageFloor(salvage, cost)

	years := 0.0
	if now.After(purchaseDate) {
		years = now.Sub(purchaseDate).Seconds() / yearSeconds
	}
	if years >= float64(usefulLifeYears) {
		return floor
	}
	value := cost * math.Pow(1-rate, years)
	if value < floor {
		return floor
	}
	return value
}

// AnnualSchedule returns the end-of-year book values under DDB, index 0 being the
// original cost and index usefulLifeYears the salvage floor. It returns nil for a
// non-positive useful life. Rate handling matches BookValue.
func AnnualSchedule(cost, salvage float64, usefulLifeYears int, rate float64) []float64 {
	if usefulLifeYears <= 0 {
		return nil
	}
	if rate <= 0 {
		rate = DefaultRate
	}
	if rate > 1 {
		rate = 1
	}
	floor := salvageFloor(salvage, cost)

	out := make([]float64, usefulLifeYears+1)
	out[0] = cost
	v := cost
	for y := 1; y <= usefulLifeYears; y++ {
		v *= (1 - rate)
		if v < floor {
			v = floor
		}
		out[y] = v
	}
	out[usefulLifeYears] = floor
	return out
}

// salvageFloor is the book-value floor: never below zero and never above cost.
func salvageFloor(salvage, cost float64) float64 {
	floor := salvage
	if floor < 0 {
		floor = 0
	}
	if floor > cost {
		floor = cost
	}
	return floor
}
