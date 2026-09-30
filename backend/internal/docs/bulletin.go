package docs

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/signintech/gopdf"

	"oss-max/internal/domain"
	"oss-max/internal/storage"
	"oss-max/templates"
)

// Bulletins рисует бюллетени одним PDF: собственнику — его собственный,
// инициатору — комплект для обхода. На каждого собственника три листа:
// сведения о праве, решение по вопросу и памятка по заполнению.
func Bulletins(w io.Writer, meeting storage.Meeting, bulletins []storage.Bulletin) error {
	d, err := newDocument()
	if err != nil {
		return err
	}
	for _, b := range bulletins {
		d.bulletin(meeting, b)
	}
	if d.err != nil {
		return fmt.Errorf("бюллетень: %w", d.err)
	}
	return d.pdf.Write(w)
}

const (
	font    = "serif"
	margin  = 15.0 // поля листа, мм
	padding = 1.2  // отступ текста от линий таблицы, мм
	ptToMM  = 25.4 / 72
)

// Размеры A4 в миллиметрах. Первый лист альбомный: в таблицу прав
// входят девять колонок. Остальные — книжные.
type size struct{ W, H float64 }

var (
	portrait  = size{210, 297}
	landscape = size{297, 210}
)

// document — PDF с текущей позицией по вертикали. Первая ошибка
// запоминается, дальнейшие вызовы её не затирают: вёрстка читается
// подряд, без проверки после каждой строки.
type document struct {
	pdf    *gopdf.GoPdf
	page   size
	y      float64
	footer string // чей бюллетень — на каждом листе: комплект печатают пачкой
	sheet  int
	err    error
}

func newDocument() (*document, error) {
	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4, Unit: gopdf.UnitMM})

	for _, f := range []struct {
		file  string
		style int
	}{
		{"LiberationSerif-Regular.ttf", gopdf.Regular},
		{"LiberationSerif-Bold.ttf", gopdf.Bold},
		{"LiberationSerif-Italic.ttf", gopdf.Italic},
	} {
		data, err := templates.Fonts.ReadFile("fonts/" + f.file)
		if err != nil {
			return nil, err
		}
		if err := pdf.AddTTFFontDataWithOption(font, data, gopdf.TtfOption{Style: f.style}); err != nil {
			return nil, fmt.Errorf("шрифт %s: %w", f.file, err)
		}
	}
	return &document{pdf: pdf}, nil
}

func (d *document) fail(err error) {
	if d.err == nil {
		d.err = err
	}
}

func (d *document) bulletin(meeting storage.Meeting, b storage.Bulletin) {
	flats := make([]string, 0, len(b.Rights))
	for _, r := range b.Rights {
		flats = append(flats, r.FlatNumber)
	}
	name := b.Name
	if name == "" {
		name = "собственник не указан"
	}
	d.footer = fmt.Sprintf("Решение собственника: %s · пом. %s", name, strings.Join(flats, ", "))
	d.sheet = 0

	d.rightsSheet(meeting, b)
	d.decisionSheet(meeting, b)
	d.memoSheet(b)
}

// rightsSheet — кто собственник и на что у него право: по выписке ЕГРН.
func (d *document) rightsSheet(meeting storage.Meeting, b storage.Bulletin) {
	d.newPage(landscape)
	width := d.page.W - 2*margin

	for _, line := range []string{
		"Решение собственника",
		"помещения в многоквартирном доме, расположенном по адресу:",
		meeting.Address,
	} {
		d.cell(margin, d.y, width, line, gopdf.Regular, 12, gopdf.Center)
		d.y += lineHeight(12)
	}
	d.y += 6

	if b.Kind == "org" {
		d.field(margin, width, b.Name, "(полное наименование организации)")
		d.y += 13
		d.field(margin, width, "", "(ИНН, ОГРН юридического лица)")
	} else {
		d.field(margin, width-62, b.Name, "(фамилия, имя, отчество)")
		d.field(margin+width-55, 55, "", "(дата рождения)")
		d.y += 13
		d.field(margin, width, "", "(страховой номер индивидуального лицевого счёта — СНИЛС)")
	}
	d.y += 15

	d.paragraph("являющийся(-щаяся) собственником помещения(-ний) или доли в праве на помещение(-ния), "+
		"указанных ниже, что подтверждается записью в ЕГРН:", 10)
	d.y += 2

	columns := []column{
		{"№ п/п", 9, gopdf.Center},
		{"Номер помещения", 20, gopdf.Center},
		{"Номер регистрации права (при отсутствии — название и номер правоустанавливающего документа)", 54, gopdf.Left},
		{"Дата регистрации права", 23, gopdf.Center},
		{"Площадь помещения, кв. м", 24, gopdf.Center},
		{"Кадастровый номер", 36, gopdf.Left},
		{"Вид собственности (единоличная, долевая, совместная)", 40, gopdf.Left},
		{"Размер доли", 17, gopdf.Center},
		{"Тип помещения (квартира, комната, часть коммунальной квартиры, нежилое помещение)", 44, gopdf.Left},
	}
	rows := make([][]string, 0, len(b.Rights))
	for i, r := range b.Rights {
		rows = append(rows, []string{
			strconv.Itoa(i + 1), r.FlatNumber, r.RegistrationNumber, r.RegistrationDate,
			decimal(r.FlatArea), r.CadastralNumber, r.OwnershipType, shareLabel(r), r.PremisesType,
		})
	}
	d.table(columns, rows, nil)
}

// decisionSheet — вопрос и ответ. Ответ из приложения уже отмечен:
// собственнику остаётся проверить его и подписать.
func (d *document) decisionSheet(meeting storage.Meeting, b storage.Bulletin) {
	d.newPage(portrait)
	width := d.page.W - 2*margin

	if b.Kind == "org" {
		d.paragraph("в лице руководителя", 10)
		d.y += 1
		d.field(margin, width, "", "(должность, фамилия, имя, отчество)")
		d.y += 12
		d.paragraph("действующего на основании устава организации.", 10)
		d.y += 5
	}

	d.paragraph("Решение собственника (представителя собственника) помещения по вопросу повестки дня:", 10)
	d.y += 2

	marks := map[domain.Choice]int{domain.ChoiceFor: 2, domain.ChoiceAgainst: 3, domain.ChoiceAbstain: 4}
	row := []string{"1", strings.TrimSpace(meeting.Question), "", "", ""}
	if i, ok := marks[b.Choice]; ok {
		row[i] = "X"
	}
	d.table([]column{
		{"№", 8, gopdf.Center},
		{"Вопрос повестки дня", 110, gopdf.Left},
		{"ЗА", 16, gopdf.Center},
		{"ПРОТИВ", 20, gopdf.Center},
		{"ВОЗДЕРЖАЛСЯ", 26, gopdf.Center},
	}, [][]string{row}, func(col int) bool { return col >= 2 })
	d.y += 8

	for _, text := range []string{
		"В соответствии с ч. 6 ст. 48 Жилищного кодекса Российской Федерации при голосовании, " +
			"осуществляемом посредством оформленных в письменной форме решений собственников " +
			"по вопросам, поставленным на голосование, засчитываются голоса по вопросам, по которым " +
			"участвующим в голосовании собственником оставлен только один из возможных вариантов голосования.",
		"С сообщением о проведении общего собрания ознакомлен(а) не позднее чем за 10 дней до даты его проведения.",
		"Дата голосования: «____» ____________________ 20___ г.",
	} {
		d.paragraph(text, 10)
		d.y += 4
	}
	d.labelLine("Время передачи решения инициатору собрания:", 30)
	d.y += 4
	d.labelLine("Адрес передачи заполненного бланка решения:", 0)
	d.y += 12

	signer, name := "Собственник:", b.Name
	if b.Kind == "org" {
		signer, name = "Руководитель:", ""
	}
	d.signature(signer, name)
	d.y += 16

	d.paragraph("Согласен(на) на обработку моих персональных данных инициатором общего собрания "+
		"в целях обеспечения моего участия в общем собрании собственников помещений в многоквартирном доме.",
		10)
	d.y += 8
	d.signature("", name)
}

// memoSheet — памятка: что делает решение недействительным.
func (d *document) memoSheet(b storage.Bulletin) {
	d.newPage(portrait)
	width := d.page.W - 2*margin

	d.paragraph("Приложение к решению собственника", 10)
	d.y += 6
	d.cell(margin, d.y, width, "УВАЖАЕМЫЙ СОБСТВЕННИК!", gopdf.Regular, 13, gopdf.Center)
	d.y += lineHeight(13) + 5

	paragraphs := []string{
		"Общее собрание собственников помещений в многоквартирном доме проводится в форме заочного голосования.",
		"По вопросу, поставленному на голосование, оставьте только один вариант ответа: «ЗА», «ПРОТИВ» " +
			"или «ВОЗДЕРЖАЛСЯ» — отметьте его знаком «X» или «V».",
	}
	if b.Choice != "" {
		paragraphs = append(paragraphs, "Отметка в решении перенесена из вашего ответа в мессенджере MAX. "+
			"Проверьте её и подпишите решение — голос засчитывается по подписанному бланку.")
	}
	paragraphs = append(paragraphs, "Решение не будет учтено при подсчёте голосов, если:")
	for _, text := range paragraphs {
		d.paragraph(text, 10)
		d.y += 3
	}

	for _, text := range []string{
		"нет сведений о собственнике помещения;",
		"не указаны номер помещения, номер и дата регистрации права или площадь помещения;",
		"по одному вопросу отмечено больше одного варианта ответа;",
		"нет подписи собственника или его представителя;",
		"вместо номера регистрации права в ЕГРН указан номер договора, акта или иного документа.",
	} {
		d.cell(margin+4, d.y, 4, "•", gopdf.Regular, 10, gopdf.Left)
		d.indented(text, 10, 10)
		d.y += 2
	}
}

// newPage начинает лист бюллетеня и сразу ставит колонтитул.
func (d *document) newPage(s size) {
	opt := gopdf.PageOption{PageSize: gopdf.PageSizeA4}
	if s == landscape {
		opt.PageSize = gopdf.PageSizeA4Landscape
	}
	d.pdf.AddPageWithOption(opt)
	d.page, d.y = s, margin
	d.sheet++

	d.pdf.SetTextColor(110, 110, 110)
	y := d.page.H - margin + 5
	width := d.page.W - 2*margin
	d.cell(margin, y, width-20, d.footer, gopdf.Regular, 7.5, gopdf.Left)
	d.cell(margin, y, width, fmt.Sprintf("лист %d", d.sheet), gopdf.Regular, 7.5, gopdf.Right)
	d.pdf.SetTextColor(0, 0, 0)
}

// cell пишет одну строку в прямоугольник шириной w; align — Left, Center или Right.
func (d *document) cell(x, y, w float64, text string, style int, size float64, align int) {
	if text == "" {
		return
	}
	d.setFont(style, size)
	d.pdf.SetXY(x, y)
	d.fail(d.pdf.CellWithOption(&gopdf.Rect{W: w, H: lineHeight(size)}, text, gopdf.CellOption{Align: align | gopdf.Top}))
}

func (d *document) setFont(style int, size float64) {
	d.fail(d.pdf.SetFontWithStyle(font, style, size))
}

// wrap разбивает текст на строки по ширине: по словам, а слово длиннее
// строки — посередине. Так переносятся длинные номера записей ЕГРН.
func (d *document) wrap(text string, w float64, style int, size float64) []string {
	d.setFont(style, size)
	var lines []string
	for _, part := range strings.Split(text, "\n") {
		if strings.TrimSpace(part) == "" {
			continue
		}
		split, err := d.pdf.SplitTextWithWordWrap(part, w)
		if err != nil {
			d.fail(err)
			return nil
		}
		for _, line := range split {
			lines = append(lines, strings.TrimSpace(line))
		}
	}
	return lines
}

// paragraph пишет абзац во всю ширину листа и сдвигает позицию вниз.
func (d *document) paragraph(text string, size float64) {
	d.indented(text, 0, size)
}

func (d *document) indented(text string, indent, size float64) {
	width := d.page.W - 2*margin - indent
	for _, line := range d.wrap(text, width, gopdf.Regular, size) {
		d.cell(margin+indent, d.y, width, line, gopdf.Regular, size, gopdf.Left)
		d.y += lineHeight(size)
	}
}

// field — линия для заполнения, как в бумажных бланках: над ней значение,
// если оно известно, под ней курсивом — что сюда вписать.
func (d *document) field(x, w float64, value, caption string) {
	line := d.y + 6
	d.cell(x, line-lineHeight(11), w, value, gopdf.Regular, 11, gopdf.Center)
	d.pdf.SetLineWidth(0.2)
	d.pdf.Line(x, line, x+w, line)
	d.cell(x, line+0.6, w, caption, gopdf.Italic, 7.5, gopdf.Center)
}

// labelLine — подпись и линия для ответа от руки; length 0 — до края листа.
func (d *document) labelLine(label string, length float64) {
	d.cell(margin, d.y, d.page.W-2*margin, label, gopdf.Regular, 10, gopdf.Left)
	d.setFont(gopdf.Regular, 10)
	labelWidth, err := d.pdf.MeasureTextWidth(label)
	d.fail(err)

	start := margin + labelWidth + 2
	end := d.page.W - margin
	if length > 0 {
		end = start + length
	}
	d.pdf.SetLineWidth(0.2)
	d.pdf.Line(start, d.y+lineHeight(10)-0.8, end, d.y+lineHeight(10)-0.8)
	d.y += lineHeight(10)
}

// signature — ФИО и подпись: ФИО вписано, если известно.
func (d *document) signature(label, name string) {
	x := margin
	if label != "" {
		d.cell(margin, d.y+6-lineHeight(10), 30, label, gopdf.Regular, 10, gopdf.Left)
		x += 26
	}
	d.field(x, 118-x+margin, name, "(фамилия, имя, отчество)")
	d.field(d.page.W-margin-50, 50, "", "(подпись)")
}

type column struct {
	title string
	width float64
	align int
}

// table рисует таблицу: высота строки — по самой длинной ячейке. Не влезает
// на лист — переходит на следующий такой же ориентации и повторяет шапку.
// mark отмечает колонки для отметки ответа: крупно и по центру ячейки.
func (d *document) table(columns []column, rows [][]string, mark func(col int) bool) {
	header := make([]string, len(columns))
	for i, c := range columns {
		header[i] = c.title
	}
	d.row(columns, header, 8, nil)
	for _, row := range rows {
		if d.y+d.rowHeight(columns, row, 9.5, mark) > d.page.H-margin-4 {
			d.newPage(d.page)
			d.row(columns, header, 8, nil)
		}
		d.row(columns, row, 9.5, mark)
	}
}

func (d *document) rowHeight(columns []column, cells []string, size float64, mark func(col int) bool) float64 {
	height := 7.0
	for i, c := range columns {
		if mark != nil && mark(i) {
			continue
		}
		lines := d.wrap(cells[i], c.width-2*padding, gopdf.Regular, size)
		if h := float64(len(lines))*lineHeight(size) + 2*padding; h > height {
			height = h
		}
	}
	return height
}

func (d *document) row(columns []column, cells []string, size float64, mark func(col int) bool) {
	height := d.rowHeight(columns, cells, size, mark)
	d.pdf.SetLineWidth(0.25)

	x := margin
	for i, c := range columns {
		d.pdf.RectFromUpperLeftWithStyle(x, d.y, c.width, height, "D")
		if mark != nil && mark(i) {
			d.cell(x, d.y+(height-lineHeight(14))/2, c.width, cells[i], gopdf.Bold, 14, gopdf.Center)
		} else {
			for j, line := range d.wrap(cells[i], c.width-2*padding, gopdf.Regular, size) {
				d.cell(x+padding, d.y+padding+float64(j)*lineHeight(size), c.width-2*padding, line, gopdf.Regular, size, c.align)
			}
		}
		x += c.width
	}
	d.y += height
}

func lineHeight(size float64) float64 {
	return size * ptToMM * 1.2
}

// shareLabel — доля, как в выписке; если её там не было — десятичной дробью.
func shareLabel(r storage.Right) string {
	if r.ShareText != "" {
		return r.ShareText
	}
	return decimal(r.Share)
}

// decimal — число с десятичной запятой и без лишних нулей: 87,9.
func decimal(v float64) string {
	return strings.Replace(strconv.FormatFloat(v, 'f', -1, 64), ".", ",", 1)
}
