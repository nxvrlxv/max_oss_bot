package domain

import "sort"

// UnitGap — помещение и метры, которые с него ещё можно собрать.
type UnitGap struct {
	Unit Unit
	Area float64 // сумма долей собственников без учтённого голоса
}

// Uncovered возвращает помещения, по которым голос учтён не полностью.
// Неподтверждённый голос считается неучтённым: обходить придётся.
func Uncovered(units []Unit, owners []Owner, votes []Vote) []UnitGap {
	counted := make(map[int]bool, len(votes))
	for _, vote := range votes {
		if vote.Status == StatusConfirmed {
			counted[vote.OwnerID] = true
		}
	}

	left := make(map[int]float64, len(units))
	for _, owner := range owners {
		if !counted[owner.OwnerID] {
			left[owner.UnitID] += owner.Share
		}
	}

	gaps := make([]UnitGap, 0, len(units))
	for _, unit := range units {
		if share := left[unit.UnitID]; share > 0 {
			gaps = append(gaps, UnitGap{Unit: unit, Area: Weight(unit.Area, share)})
		}
	}

	return gaps
}

// Route подбирает помещения для обхода: берёт самые крупные,
// пока не закроется разрыв. Возвращает набор и остаток разрыва.
//
// Подбор жадный, минимальность набора не гарантирована: точное решение
// требует перебора, а разница на практике — одно помещение.
func Route(gaps []UnitGap, gap float64) ([]UnitGap, float64) {
	if gap <= 0 {
		return nil, 0
	}

	sorted := make([]UnitGap, len(gaps))
	copy(sorted, gaps)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Area > sorted[j].Area
	})

	route := make([]UnitGap, 0, len(sorted))
	for _, candidate := range sorted {
		if gap <= 0 {
			break
		}
		route = append(route, candidate)
		gap -= candidate.Area
	}

	if gap < 0 {
		gap = 0
	}

	return route, gap
}
