package domain

import "testing"

func vote(ownerID int, choice Choice, area float64, status Status) Vote {
	return Vote{OwnerID: ownerID, Choice: choice, Area: area, Status: status}
}

func TestWeight(t *testing.T) {
	if got := Weight(60, 0.5); got != 30 {
		t.Errorf("половина от 60 = %v, хотели 30", got)
	}
	if got := Weight(60, 1); got != 60 {
		t.Errorf("целая квартира = %v, хотели 60", got)
	}
}

func TestCountVotes(t *testing.T) {
	votes := []Vote{
		vote(1, ChoiceFor, 100, StatusConfirmed),
		vote(2, ChoiceAgainst, 50, StatusConfirmed),
		vote(3, ChoiceAbstain, 30, StatusConfirmed),
		vote(4, ChoiceFor, 900, StatusPending),       // не подтверждён
		vote(1, ChoiceAgainst, 100, StatusConfirmed), // повтор того же собственника
		vote(5, "мусор", 70, StatusConfirmed),
	}

	tally := CountVotes(votes)

	if tally.For != 100 {
		t.Errorf("For = %v, хотели 100", tally.For)
	}
	if tally.Against != 50 {
		t.Errorf("Against = %v, хотели 50", tally.Against)
	}
	if tally.Abstain != 30 {
		t.Errorf("Abstain = %v, хотели 30", tally.Abstain)
	}
	// Явка: воздержавшийся входит, pending и мусор — нет.
	if tally.Total != 180 {
		t.Errorf("Total = %v, хотели 180", tally.Total)
	}
}

func TestEvaluateNoQuorum(t *testing.T) {
	votes := []Vote{vote(1, ChoiceFor, 2000, StatusConfirmed)}

	res := Evaluate(houseArea, votes, RuleSoft())

	if res.Quorum {
		t.Error("2000 из 6000 — кворума нет")
	}
	if res.Accepted {
		t.Error("без кворума решение не принимается")
	}
	if res.Gap != 1000 {
		t.Errorf("до кворума не хватает %v, хотели 1000", res.Gap)
	}
}

// Главный случай: явка 70%, против почти никто, но квалифицированное
// решение считается от всего дома, а не от явки.
func TestEvaluateQualifiedFromHouse(t *testing.T) {
	votes := []Vote{
		vote(1, ChoiceFor, 2900, StatusConfirmed),
		vote(2, ChoiceAgainst, 100, StatusConfirmed),
		vote(3, ChoiceAbstain, 1200, StatusConfirmed),
	}

	res := Evaluate(houseArea, votes, RuleHard())

	if !res.Quorum {
		t.Error("4200 из 6000 — кворум есть")
	}
	if res.Accepted {
		t.Error("2900 из 6000 — это 48%, решение не принято")
	}
	if res.Gap != 1100 {
		t.Errorf("не хватает %v, хотели 1100", res.Gap)
	}
}

func TestEvaluateSimpleAccepted(t *testing.T) {
	votes := []Vote{
		vote(1, ChoiceFor, 2500, StatusConfirmed),
		vote(2, ChoiceAgainst, 1000, StatusConfirmed),
	}

	res := Evaluate(houseArea, votes, RuleSoft())

	if !res.Quorum {
		t.Error("3500 из 6000 — кворум есть")
	}
	if !res.Accepted {
		t.Error("2500 из 3500 — больше половины явки, решение принято")
	}
	if res.Gap != 0 {
		t.Errorf("Gap = %v, хотели 0", res.Gap)
	}
}
