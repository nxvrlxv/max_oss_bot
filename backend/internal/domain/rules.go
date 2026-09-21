package domain

// Choice — вариант ответа. Значения латиницей: они уходят в базу
// и в JSON, а подписи «за» / «против» / «воздержался» живут в интерфейсе.
type Choice string

const (
	ChoiceFor     Choice = "for"
	ChoiceAgainst Choice = "against"
	ChoiceAbstain Choice = "abstain"
)

// Status — состояние голоса. Пока инициатор не подтвердил привязку
// собственника, голос сохранён, но в подсчёт не идёт.
type Status string

const (
	StatusPending   Status = "pending"
	StatusConfirmed Status = "confirmed"
)

type Base string

const (
	BaseTotal         Base = "total"         // от всей площади дома
	BaseParticipating Base = "participating" // от площади принявших участие
)

type Step float64

const (
	HalfStep      Step = 0.5
	TwoThirdsStep Step = 2.0 / 3.0
	FullStep      Step = 1.0
)

type Op string

const (
	OpGreater      Op = ">"  // «более чем» — кворум, простое большинство
	OpGreaterEqual Op = ">=" // «не менее» — квалифицированное большинство
)

type Unit struct {
	UnitID   int
	Number   string
	Entrance int
	Area     float64
}

type Owner struct {
	OwnerID int
	UnitID  int
	Share   float64 // доля в праве: от 0 до 1
}

type Vote struct {
	OwnerID int
	UnitID  int
	Choice  Choice
	Status  Status
	Area    float64 // вес голоса в м, зафиксирован в момент голосования
}

type Tally struct {
	For     float64
	Against float64
	Abstain float64
	Total   float64 // явка: сумма учтённых голосов
}

// Rule — правило порога. Хранится в meetings.rule_json,
// поэтому поля с тегами.
type Rule struct {
	Base      Base `json:"base"`
	Threshold Step `json:"threshold"`
	Op        Op   `json:"op"`
}

// RuleQuorum — правомочность собрания: более 50% от всей площади дома.
func RuleQuorum() Rule {
	return Rule{
		Base:      BaseTotal,
		Threshold: HalfStep,
		Op:        OpGreater,
	}
}

// RuleSoft — обычное решение: большинство от принявших участие.
func RuleSoft() Rule {
	return Rule{
		Base:      BaseParticipating,
		Threshold: HalfStep,
		Op:        OpGreater,
	}
}

// RuleHard — капремонт, шлагбаум, аренда общего имущества, земля:
// не менее 2/3 от всей площади дома, а не от явки.
func RuleHard() Rule {
	return Rule{
		Base:      BaseTotal,
		Threshold: TwoThirdsStep,
		Op:        OpGreaterEqual,
	}
}

// RuleAll — уменьшение общего имущества: согласие всех собственников.
func RuleAll() Rule {
	return Rule{
		Base:      BaseTotal,
		Threshold: FullStep,
		Op:        OpGreaterEqual,
	}
}

// Valid проверяет правило, пришедшее из базы: rule_json может оказаться
// пустым или повреждённым, и тогда собрание молча покажет «порог не взят».
func (r Rule) Valid() bool {
	if r.Base != BaseTotal && r.Base != BaseParticipating {
		return false
	}
	if r.Op != OpGreater && r.Op != OpGreaterEqual {
		return false
	}
	return r.Threshold > 0 && r.Threshold <= FullStep
}

// Denominator возвращает число, от которого считается порог.
func (r Rule) Denominator(total, participating float64) float64 {
	if r.Base == BaseParticipating {
		return participating
	}
	return total
}

// Required — сколько метров нужно набрать при такой базе.
// При строгом неравенстве само это число порога ещё не даёт:
// его нужно превысить.
func (r Rule) Required(base float64) float64 {
	return base * float64(r.Threshold)
}

// Passed сравнивает долю набранных голосов с порогом.
// sum и base — в м², оба считаются снаружи.
func (r Rule) Passed(sum, base float64) bool {
	if base <= 0 {
		return false
	}

	share := sum / base

	if r.Op == OpGreaterEqual {
		return share >= float64(r.Threshold)
	}
	return share > float64(r.Threshold)
}

// Gap — сколько метров не хватает до порога.
func (r Rule) Gap(sum, base float64) float64 {
	gap := r.Required(base) - sum
	if gap < 0 {
		return 0
	}
	return gap
}
