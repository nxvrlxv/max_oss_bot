package domain

// Weight — вес голоса: площадь помещения, умноженная на долю в праве.
func Weight(area, share float64) float64 {
	return area * share
}

// CountVotes складывает голоса по вариантам.
// Неподтверждённые и повторные голоса не учитываются.
func CountVotes(votes []Vote) Tally {
	var tally Tally
	counted := make(map[int]bool, len(votes))

	for _, vote := range votes {
		if vote.Status != StatusConfirmed || counted[vote.OwnerID] {
			continue
		}

		switch vote.Choice {
		case ChoiceFor:
			tally.For += vote.Area
		case ChoiceAgainst:
			tally.Against += vote.Area
		case ChoiceAbstain:
			tally.Abstain += vote.Area
		default:
			continue // мусор в choice в явку не идёт
		}

		counted[vote.OwnerID] = true
		tally.Total += vote.Area
	}

	return tally
}

// Result — итог собрания на текущий момент.
type Result struct {
	Tally     Tally
	TotalArea float64
	Quorum    bool    // собрание правомочно
	Accepted  bool    // решение принято
	Required  float64 // сколько метров нужно до ближайшего невзятого порога
	Gap       float64 // сколько не хватает
}

// Evaluate считает итог собрания по правилу вопроса.
// totalArea — общая площадь дома, введённая инициатором.
func Evaluate(totalArea float64, votes []Vote, rule Rule) Result {
	tally := CountVotes(votes)
	res := Result{Tally: tally, TotalArea: totalArea}

	quorum := RuleQuorum()
	res.Quorum = quorum.Passed(tally.Total, totalArea)

	// Без кворума собрание неправомочно, решение не принято при любом «за».
	if !res.Quorum || !rule.Valid() {
		res.Required = quorum.Required(totalArea)
		res.Gap = quorum.Gap(tally.Total, totalArea)
		return res
	}

	base := rule.Denominator(totalArea, tally.Total)
	res.Accepted = rule.Passed(tally.For, base)
	res.Required = rule.Required(base)
	res.Gap = rule.Gap(tally.For, base)

	return res
}
