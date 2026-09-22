package registry

import (
	"bufio"
	"strings"
	"testing"
)

func TestDetectDelimiter(t *testing.T) {
	for _, sep := range []rune{';', ',', '\t'} {
		header := strings.SplitN(sample(sep, "30"), "\n", 2)[0]
		got, err := DetectDelimiter([]byte(header))
		if err != nil || got != sep { t.Fatalf("separator %q: got %q, %v", sep, got, err) }
	}
	for _, header := range []string{"", "name;area", strings.SplitN(sample('|', "30"), "\n", 2)[0]} {
		if _, err := DetectDelimiter([]byte(header)); err == nil { t.Fatalf("accepted %q", header) }
	}
}

func TestStreamingHeaderAndUTF8(t *testing.T) {
	input := strings.Replace(sample(';', "30"), "№ помещения", "№\nпомещения", 1)
	// Quote the header cell containing the newline.
	input = strings.Replace(input, "№\nпомещения", "\"№\nпомещения\"", 1)
	rows, err := ParseCSV(strings.NewReader(input))
	if err != nil || len(rows) != 1 || rows[0].SourceLine != 3 { t.Fatalf("multiline header: %+v, %v", rows, err) }
	input = strings.Replace(sample(';', "30"), "Иванов", string([]byte{0xff}), 1)
	if rows, err := ParseCSV(strings.NewReader(input)); err == nil || rows != nil { t.Fatal("accepted invalid UTF-8 in data") }
	if _, err := readHeader(bufio.NewReader(strings.NewReader(strings.Repeat("x", 65537)))); err == nil { t.Fatal("accepted oversized header") }
}
