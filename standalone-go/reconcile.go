package main

// Exobiology value/sold/lost reconciliation -- ported from this project's earlier Python
// version's reconcile.py, itself ported from an already-proven _reconcile_sale_status()/
// value-formula implementation. SellOrganicData can't shortcut this reconciliation -- its
// BioData array has no body/system reference at all.

import "sort"

const footfallBonusMultiplier = 4
const firstLoggedBonusMultiplier = 4

// Mirrors reconcile.py's LOSS_RISK_RESURRECTION_TYPES exactly -- this commander's own real
// Resurrect events are all "rebuy", which is NOT in this set.
var lossRiskResurrectionTypes = map[string]bool{"escape": true, "recover": true, "rejoin": true}

type FloraValue struct {
	SystemAddress    int64
	BodyID           int
	Genus            string
	Species          string
	Variant          string
	GenusName        string
	SpeciesName      string
	Count            int
	WasLogged        *bool
	ScannedAt        string
	BaseValue        int64
	HasBaseValue     bool
	FootfallBonus    bool
	FirstLoggedBonus bool
	Value            int64
	HasValue         bool
	Sold             bool
	Lost             bool
	PredictedMin     int64
	PredictedMax     int64
	HasPredicted     bool
}

// bisectRight mirrors Python's bisect.bisect_right(sortedTimes, target): the index of the
// first element strictly greater than target (i.e. insertion point keeping the slice sorted,
// to the right of any existing equal entries).
func bisectRight(sortedTimes []string, target string) int {
	return sort.Search(len(sortedTimes), func(i int) bool { return sortedTimes[i] > target })
}

func reconcileSaleStatus(scannedAt string, saleTimes, lossTimes []string) (sold, lost bool) {
	if scannedAt == "" {
		return false, false
	}
	lostIdx := bisectRight(lossTimes, scannedAt)
	var lostDate string
	hasLostDate := false
	if lostIdx < len(lossTimes) {
		lostDate = lossTimes[lostIdx]
		hasLostDate = true
	}
	saleIdx := bisectRight(saleTimes, scannedAt)
	var nextSale string
	hasNextSale := false
	if saleIdx < len(saleTimes) {
		nextSale = saleTimes[saleIdx]
		hasNextSale = true
	}
	if hasLostDate {
		sold = hasNextSale && nextSale < lostDate
		return sold, !sold
	}
	return hasNextSale, false
}

// ComputeFloraValues resolves value/sold/lost for every recorded flora scan across the whole
// store, batching the sale/loss timestamp loads once (same approach as the Python version).
func ComputeFloraValues(store *Store) []FloraValue {
	saleTimes := append([]string(nil), store.SaleTimes...)
	sort.Strings(saleTimes)

	lossTimes := append([]string(nil), store.DeathTimes...)
	for _, r := range store.Resurrections {
		if lossRiskResurrectionTypes[r.Option] {
			lossTimes = append(lossTimes, r.ResurrectedAt)
		}
	}
	sort.Strings(lossTimes)

	var results []FloraValue
	for systemAddress, sys := range store.Systems {
		for bodyID, pl := range sys.Planets {
			wasFootfalled := pl.WasFootfalled != nil && *pl.WasFootfalled
			population := sys.Population
			for _, fs := range pl.Flora {
				fv := FloraValue{
					SystemAddress: systemAddress, BodyID: bodyID,
					Genus: fs.Genus, Species: fs.Species, Variant: fs.Variant,
					GenusName: genusDisplayName(fs.Genus),
					Count:     fs.Count,
					WasLogged: fs.WasLogged,
					ScannedAt: fs.ScannedAt,
				}
				info, ok := lookupSpecies(fs.Genus, fs.Species)
				if ok {
					fv.BaseValue = info.Value
					fv.HasBaseValue = true
					fv.SpeciesName = info.Name
				}

				footfallBonus := !wasFootfalled && population == 0
				firstLoggedBonus := fv.WasLogged != nil && !*fv.WasLogged
				bonusUnits := int64(0)
				if footfallBonus {
					bonusUnits += footfallBonusMultiplier
				}
				if firstLoggedBonus {
					bonusUnits += firstLoggedBonusMultiplier
				}

				if fs.Count == 3 && fv.HasBaseValue {
					fv.Value = fv.BaseValue * (1 + bonusUnits)
					fv.HasValue = true
					fv.Sold, fv.Lost = reconcileSaleStatus(fs.ScannedAt, saleTimes, lossTimes)
				}
				fv.FootfallBonus = footfallBonus && fs.Count == 3
				fv.FirstLoggedBonus = firstLoggedBonus && fs.Count == 3

				if fs.Species == "" {
					if lo, hi, ok := speciesValueRange(fs.Genus); ok {
						fv.PredictedMin, fv.PredictedMax, fv.HasPredicted = lo, hi, true
					}
				}

				results = append(results, fv)
			}
		}
	}
	return results
}

func speciesValueRange(genus string) (min, max int64, ok bool) {
	species, exists := valueTable[genus]
	if !exists || len(species) == 0 {
		return 0, 0, false
	}
	first := true
	for _, info := range species {
		if first {
			min, max = info.Value, info.Value
			first = false
			continue
		}
		if info.Value < min {
			min = info.Value
		}
		if info.Value > max {
			max = info.Value
		}
	}
	return min, max, true
}
