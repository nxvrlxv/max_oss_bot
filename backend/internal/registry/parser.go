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

// Ownership describes one owner of one premises, not a registered MAX user.
// Areas are canonical decimal strings in square metres, suitable for SQL DECIMAL.
// FullName may contain a legal entity name. It is not a unique owner identifier.
type Ownership struct {
 // Share and dates retain the source notation; no fraction/date conversion is applied.
 Share string `json:"share,omitempty"`
 ObjectAddress string `json:"object_address"`
 PremisesType string `json:"premises_type,omitempty"`
 OwnershipType string `json:"ownership_type,omitempty"`
 RegistrationNumber string `json:"registration_number,omitempty"`
 RegistrationDate string `json:"registration_date,omitempty"`
 CadastralAssignmentDate string `json:"cadastral_assignment_date,omitempty"`
 ExtractNumber string `json:"extract_number,omitempty"`
 ExtractDate string `json:"extract_date,omitempty"`
 SourceLine int `json:"source_line"`
 // SourceFields preserves all original cells, including unknown columns and blanks.
 SourceFields map[string]string `json:"source_fields"`
 FullName string `json:"full_name"`
 HouseAddress string `json:"house_address"`
 FlatNumber string `json:"flat_number"`
 OwnedArea string `json:"owned_area"`
 FlatArea string `json:"flat_area"`
 CadastralNumber string `json:"cadastral_number,omitempty"`
}

// ParseCSV reads a UTF-8 registry (optional BOM), delimited by ';', ',' or tab.
// Each row must describe exactly one owner of one premises. A blank premises
// number continues the previous premises; conflicting premises fields are rejected.
// Only premises fields are inherited, never owner/right/extract fields. The entire import
// fails on an invalid row; a partial list must never be used for synchronization.
// Callers handling uploads should bound the reader/file size before calling.
func ParseCSV(src io.Reader) ([]Ownership, error) {
 buffered := bufio.NewReader(src)
 header, err := readHeader(buffered)
 if err != nil { return nil, err }
 sep, err := DetectDelimiter(header)
 if err != nil { return nil, err }
 // Replay the header so csv.Reader retains field counts and source line numbers.
 reader := csv.NewReader(io.MultiReader(bytes.NewReader(header), buffered))
 reader.Comma = sep
 names, err := reader.Read()
 if err != nil { return nil, fmt.Errorf("read CSV header: %w", err) }
 columns := map[string]int{}
 for i, name := range names { columns[clean(name)] = i }
 result := []Ownership{}
 premisesKeys := []string{"№ помещения", "Адрес объекта", "Кадастровый номер объекта", "Вид помещения", "Общая площадь, м²", "Дата присвоения кадастрового номера"}
 var previous map[string]string
 for {
  row, e := reader.Read()
  if e == io.EOF { break }
  if e != nil { return nil, fmt.Errorf("read CSV: %w", e) }
  line, _ := reader.FieldPos(0)
  for _, value := range row {
   if !utf8.ValidString(value) { return nil, fmt.Errorf("line %d: registry must be UTF-8 encoded", line) }
  }
  source := map[string]string{}
  values := map[string]string{}
  for key, i := range columns { source[key] = row[i]; values[key] = clean(row[i]) }
  if strings.ContainsAny(strings.TrimSpace(source["Правообладатель (правообладатели)"]), "\r\n") {
   return nil, fmt.Errorf("line %d: multiline owner cell is ambiguous; use one row per owner", line)
  }
  if values["№ помещения"] == "" {
   if previous == nil { return nil, fmt.Errorf("line %d: continuation without previous premises", line) }
   for _, key := range premisesKeys {
    value := values[key]
    if value != "" && value != previous[key] {
     return nil, fmt.Errorf("line %d: continuation conflicts with previous %s; repeat premises number for a new premises", line, key)
    }
    values[key] = previous[key]
   }
  }
  get := func(key string) string { return values[key] }
  for _, h := range requiredColumns { if get(h)=="" { return nil, fmt.Errorf("line %d: empty %s", line, h) } }
  owned, e := decimal(get("Долевая площадь, м²")); if e != nil { return nil, fmt.Errorf("line %d: owned area: %w", line, e) }
  total, e := decimal(get("Общая площадь, м²")); if e != nil { return nil, fmt.Errorf("line %d: flat area: %w", line, e) }
  if compareDecimals(owned, total)>0 { return nil, fmt.Errorf("line %d: owned area exceeds flat area", line) }
  address := get("Адрес объекта")
  if loc := premisesSuffix.FindStringIndex(address); loc != nil { address = strings.TrimSpace(address[:loc[0]]) }
  if address == "" { return nil, fmt.Errorf("line %d: empty house address", line) }
  result = append(result, Ownership{
   FullName:get("Правообладатель (правообладатели)"), HouseAddress:address, FlatNumber:get("№ помещения"), OwnedArea:owned, FlatArea:total, CadastralNumber:get("Кадастровый номер объекта"),
   Share:source["Доля собственника"], ObjectAddress:get("Адрес объекта"), PremisesType:get("Вид помещения"), OwnershipType:get("Вид собственности"),
   RegistrationNumber:get("Запись в ЕГРН, №"), RegistrationDate:get("Дата регистрации права"), CadastralAssignmentDate:get("Дата присвоения кадастрового номера"),
   ExtractNumber:get("Номер выписки"), ExtractDate:get("Дата выписки"), SourceLine:line, SourceFields:source,
  })
  previous = values
 }
 if len(result)==0 { return nil, fmt.Errorf("registry has no ownership rows") }
 return result, nil
}
