package domain

import "testing"

// Дом на 6000 м² — во всех тестах пакета.
const houseArea = 6000

func TestRulePassed(t *testing.T) {
	tests := []struct {
		name string
		rule Rule
		sum  float64
		base float64
		want bool
	}{
		{"кворум: ровно половина не даёт кворума", RuleQuorum(), 3000, houseArea, false},
		{"кворум: половина плюс метр", RuleQuorum(), 3001, houseArea, true},
		{"кворум: пустое собрание", RuleQuorum(), 0, houseArea, false},
		{"кворум: нулевая база", RuleQuorum(), 100, 0, false},

		{"простое: больше половины явки", RuleSoft(), 2101, 4200, true},
		{"простое: ровно половина явки", RuleSoft(), 2100, 4200, false},

		{"квалифицированное: ровно 2/3 принимается", RuleHard(), 4000, houseArea, true},
		{"квалифицированное: чуть меньше 2/3", RuleHard(), 3999, houseArea, false},

		{"единогласное: не все", RuleAll(), 5999, houseArea, false},
		{"единогласное: все", RuleAll(), houseArea, houseArea, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rule.Passed(tt.sum, tt.base); got != tt.want {
				t.Errorf("Passed(%v, %v) = %v, хотели %v", tt.sum, tt.base, got, tt.want)
			}
		})
	}
}

func TestRuleRequiredAndGap(t *testing.T) {
	quorum := RuleQuorum()

	if got := quorum.Required(houseArea); got != 3000 {
		t.Errorf("Required = %v, хотели 3000", got)
	}
	if got := quorum.Gap(2400, houseArea); got != 600 {
		t.Errorf("Gap = %v, хотели 600", got)
	}
	if got := quorum.Gap(5000, houseArea); got != 0 {
		t.Errorf("Gap при взятом пороге = %v, хотели 0", got)
	}

	if got := RuleHard().Required(houseArea); got != 4000 {
		t.Errorf("Required для 2/3 = %v, хотели 4000", got)
	}
}

func TestRuleValid(t *testing.T) {
	if !RuleQuorum().Valid() {
		t.Error("заготовка кворума должна быть валидной")
	}
	if (Rule{}).Valid() {
		t.Error("пустое правило не должно проходить проверку")
	}
	if (Rule{Base: BaseTotal, Threshold: 1.5, Op: OpGreater}).Valid() {
		t.Error("порог больше единицы недопустим")
	}
}
