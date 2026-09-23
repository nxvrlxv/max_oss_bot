package registry

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

var requiredColumns = []string{"№ помещения", "Адрес объекта", "Правообладатель (правообладатели)", "Долевая площадь, м²", "Общая площадь, м²"}

// DetectDelimiter returns ';', ',' or '\t' for a registry header.
// It uses CSV quoting rules and required column names, not character frequency.
// header must contain a complete first CSV record, optionally with a UTF-8 BOM.
func DetectDelimiter(header []byte) (rune, error) {
	header = bytes.TrimPrefix(header, []byte("\uFEFF"))
	if !utf8.Valid(header) {
		return 0, fmt.Errorf("файл должен быть в кодировке UTF-8 — пересохраните его как «CSV UTF-8»")
	}
	var detected rune
	for _, sep := range []rune{';', ',', '\t'} {
		r := csv.NewReader(bytes.NewReader(header))
		r.Comma = sep
		names, err := r.Read()
		if err != nil {
			continue
		}
		columns := map[string]bool{}
		valid := true
		for _, name := range names {
			key := clean(name)
			if columns[key] {
				valid = false
			}
			columns[key] = true
		}
		for _, name := range requiredColumns {
			if !columns[name] {
				valid = false
			}
		}
		if valid {
			if detected != 0 {
				return 0, fmt.Errorf("не удалось определить разделитель столбцов")
			}
			detected = sep
		}
	}
	if detected == 0 {
		return 0, fmt.Errorf("в первой строке файла нужны столбцы без повторов, через «;», «,» или табуляцию: %s", strings.Join(requiredColumns, ", "))
	}
	return detected, nil
}

// Buffer only the header, including quoted newlines. Bound malformed input too.
func readHeader(src *bufio.Reader) ([]byte, error) {
	const maxHeaderBytes = 64 * 1024
	var header []byte
	for {
		part, err := src.ReadSlice('\n')
		if len(header)+len(part) > maxHeaderBytes {
			return nil, fmt.Errorf("первая строка файла длиннее %d байт — это не похоже на реестр", maxHeaderBytes)
		}
		header = append(header, part...)
		if err == bufio.ErrBufferFull {
			continue
		}
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("не удалось прочитать заголовок: %w", err)
		}
		content := bytes.TrimSpace(bytes.TrimPrefix(header, []byte("\uFEFF")))
		if err == io.EOF || (bytes.Count(header, []byte{'"'})%2 == 0 && len(content) > 0) {
			return bytes.TrimPrefix(header, []byte("\uFEFF")), nil
		}
	}
}
