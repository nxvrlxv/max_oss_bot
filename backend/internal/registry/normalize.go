package registry

import (
	"fmt"
	"math/big"
	"regexp"
	"strings"
)

var premisesSuffix = regexp.MustCompile(`(?i),\s*(?:кв\.?|квартира|пом\.?|помещение)\s*[^,]+$`)
var decimalPattern = regexp.MustCompile(`^[0-9]+(?:[.,][0-9]+)?$`)

func clean(s string) string { return strings.Join(strings.Fields(s), " ") }

func decimal(s string) (string, error) {
	if !decimalPattern.MatchString(s) {
		return "", fmt.Errorf("«%s» — не положительное число", s)
	}
	s = strings.ReplaceAll(s, ",", ".")
	parts := strings.SplitN(s, ".", 2)
	integer := strings.TrimLeft(parts[0], "0")
	if integer == "" {
		integer = "0"
	}
	s = integer
	if len(parts) == 2 {
		fraction := strings.TrimRight(parts[1], "0")
		if fraction != "" {
			s += "." + fraction
		}
	}
	if s == "0" {
		return "", fmt.Errorf("площадь должна быть больше нуля")
	}
	return s, nil
}

func compareDecimals(a, b string) int {
	x, _ := new(big.Rat).SetString(a)
	y, _ := new(big.Rat).SetString(b)
	return x.Cmp(y)
}
