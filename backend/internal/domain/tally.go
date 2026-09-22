package domain

type Choice string

const (
	ChoiceFor     Choice = "for"
	ChoiceAgainst Choice = "against"
	ChoiceAbstain Choice = "abstain"
)

type Vote struct {
	Choice Choice
	Area   int64
}

type Tally struct {
	For     int64
	Against int64
	Abstain int64
	Total   int64
}

func CountVotes(votes []Vote) (Tally, error) {
	var tally Tally
	for _, vote := range votes {
		switch vote.Choice {
		case ChoiceFor:
			tally.For += vote.Area
		case ChoiceAgainst:
			tally.Against += vote.Area
		case ChoiceAbstain:
			tally.Abstain += vote.Area
		}
		tally.Total += vote.Area
	}
	return tally, nil
}

func isQuorum(tally Tally, purpose int64) bool {
	if float64(tally.Total) > float64(purpose)/2.0 {
		return true
	} else {
		return false
	}
}

func countDifference(tally Tally, purpose int64, class string) {

}

func main() {

}
