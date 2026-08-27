/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Excel and CSV, from the same Result the screen renders.
 */

package reporting

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"github.com/xuri/excelize/v2"
)

// Format is an export target.
type Format string

const (
	FormatXLSX Format = "xlsx"
	FormatCSV  Format = "csv"
)

// ParseFormat validates a requested format.
func ParseFormat(raw string) (Format, error) {
	switch Format(strings.ToLower(strings.TrimSpace(raw))) {
	case FormatXLSX, "":
		return FormatXLSX, nil
	case FormatCSV:
		return FormatCSV, nil
	default:
		return "", fmt.Errorf("unsupported export format %q", raw)
	}
}

// ContentType is what the response says the bytes are.
func (f Format) ContentType() string {
	if f == FormatCSV {
		return "text/csv; charset=utf-8"
	}
	return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
}

// Export renders a result. `title` is the report's name in the caller's locale
// and becomes the sheet name and the first row.
func Export(format Format, title string, result Result, locale string) ([]byte, error) {
	if format == FormatCSV {
		return exportCSV(title, result, locale)
	}
	return exportXLSX(title, result, locale)
}

// exportCSV writes the columns and the rows, with a UTF-8 byte-order mark.
//
// The BOM is there for Excel. Without it, Excel on Windows reads a UTF-8 CSV as
// the system code page, and every Cyrillic heading in this platform's reports
// arrives as mojibake — which is the entire content of a Mongolian report.
func exportCSV(_ string, result Result, locale string) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("\xef\xbb\xbf")

	writer := csv.NewWriter(&buf)

	header := make([]string, 0, len(result.Columns))
	for _, column := range result.Columns {
		header = append(header, LocalizedTitle(column.Titles, locale, column.Key))
	}
	if err := writer.Write(header); err != nil {
		return nil, err
	}

	for _, row := range result.Rows {
		record := make([]string, 0, len(result.Columns))
		for _, column := range result.Columns {
			record = append(record, formatCell(row[column.Key], column.Kind))
		}
		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}

	if len(result.Totals) > 0 {
		record := make([]string, 0, len(result.Columns))
		for index, column := range result.Columns {
			switch {
			case index == 0:
				record = append(record, totalsLabel(locale))
			case column.Total:
				record = append(record, formatCell(result.Totals[column.Key], column.Kind))
			default:
				record = append(record, "")
			}
		}
		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// exportXLSX writes a formatted sheet: a title row, a bold header, number and
// date formats per column, and a totals row.
//
// Numbers go in as numbers rather than as strings. A spreadsheet whose amounts
// cannot be summed is a screenshot with extra steps, and it is the first thing
// anybody notices about an export.
func exportXLSX(title string, result Result, locale string) ([]byte, error) {
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()

	const sheet = "Sheet1"

	titleStyle, err := file.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 14},
	})
	if err != nil {
		return nil, err
	}
	headerStyle, err := file.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#EEF2F7"}},
		Border: []excelize.Border{
			{Type: "bottom", Color: "#B9C3D0", Style: 1},
		},
	})
	if err != nil {
		return nil, err
	}
	totalStyle, err := file.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Border: []excelize.Border{
			{Type: "top", Color: "#B9C3D0", Style: 1},
		},
	})
	if err != nil {
		return nil, err
	}

	columnStyles := make([]int, len(result.Columns))
	for index, column := range result.Columns {
		style, err := file.NewStyle(&excelize.Style{NumFmt: numberFormat(column.Kind)})
		if err != nil {
			return nil, err
		}
		columnStyles[index] = style
	}

	// Row 1: the report's name, as the reader asked for it. Row 2 is left
	// empty; row 3 is the header.
	if err := file.SetCellStr(sheet, "A1", title); err != nil {
		return nil, err
	}
	if err := file.SetCellStyle(sheet, "A1", "A1", titleStyle); err != nil {
		return nil, err
	}

	const headerRow = 3
	for index, column := range result.Columns {
		cell, err := excelize.CoordinatesToCellName(index+1, headerRow)
		if err != nil {
			return nil, err
		}
		if err := file.SetCellStr(sheet, cell, LocalizedTitle(column.Titles, locale, column.Key)); err != nil {
			return nil, err
		}
		if err := file.SetCellStyle(sheet, cell, cell, headerStyle); err != nil {
			return nil, err
		}
	}

	for rowIndex, row := range result.Rows {
		for columnIndex, column := range result.Columns {
			cell, err := excelize.CoordinatesToCellName(columnIndex+1, headerRow+1+rowIndex)
			if err != nil {
				return nil, err
			}
			if err := writeCell(file, sheet, cell, row[column.Key], column.Kind); err != nil {
				return nil, err
			}
			if err := file.SetCellStyle(sheet, cell, cell, columnStyles[columnIndex]); err != nil {
				return nil, err
			}
		}
	}

	if len(result.Totals) > 0 {
		totalRow := headerRow + 1 + len(result.Rows)
		for columnIndex, column := range result.Columns {
			cell, err := excelize.CoordinatesToCellName(columnIndex+1, totalRow)
			if err != nil {
				return nil, err
			}
			switch {
			case columnIndex == 0:
				err = file.SetCellStr(sheet, cell, totalsLabel(locale))
			case column.Total:
				err = file.SetCellFloat(sheet, cell, result.Totals[column.Key], decimals(column.Kind), 64)
			}
			if err != nil {
				return nil, err
			}
			if err := file.SetCellStyle(sheet, cell, cell, totalStyle); err != nil {
				return nil, err
			}
		}
	}

	// Widths from the header, bounded. Auto-fitting to content would mean
	// walking every cell, and a column of long free text would push the rest
	// off the screen.
	for index, column := range result.Columns {
		name, err := excelize.ColumnNumberToName(index + 1)
		if err != nil {
			return nil, err
		}
		width := float64(len([]rune(LocalizedTitle(column.Titles, locale, column.Key)))) + 6
		if width < 12 {
			width = 12
		}
		if width > 44 {
			width = 44
		}
		if err := file.SetColWidth(sheet, name, name, width); err != nil {
			return nil, err
		}
	}

	// The header stays visible when a thousand-row report is scrolled.
	if err := file.SetPanes(sheet, &excelize.Panes{
		Freeze: true, Split: false, XSplit: 0, YSplit: headerRow, TopLeftCell: "A4", ActivePane: "bottomLeft",
	}); err != nil {
		return nil, err
	}

	buf, err := file.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeCell(file *excelize.File, sheet, cell string, value any, kind ColumnKind) error {
	if value == nil {
		return nil
	}
	switch kind {
	case ColumnNumber, ColumnMoney, ColumnPercent:
		return file.SetCellFloat(sheet, cell, asFloat(value), decimals(kind), 64)
	case ColumnDate, ColumnMonth:
		if when, ok := value.(time.Time); ok {
			return file.SetCellStr(sheet, cell, formatTime(when, kind))
		}
		return file.SetCellStr(sheet, cell, fmt.Sprint(value))
	default:
		return file.SetCellStr(sheet, cell, fmt.Sprint(value))
	}
}

// numberFormat maps a column kind to an Excel built-in format id.
//
//	4  = #,##0.00
//	3  = #,##0
//	10 = 0.00%
//
// Money uses the thousands-separated decimal rather than a currency format:
// this platform's tenants are billed in tugrik and a hard-coded ₮ in an export
// somebody opens in another currency would be wrong rather than merely absent.
func numberFormat(kind ColumnKind) int {
	switch kind {
	case ColumnMoney:
		return 4
	case ColumnNumber:
		return 3
	case ColumnPercent:
		return 10
	default:
		return 0
	}
}

func decimals(kind ColumnKind) int {
	switch kind {
	case ColumnMoney, ColumnPercent:
		return 2
	default:
		return 0
	}
}

func formatCell(value any, kind ColumnKind) string {
	if value == nil {
		return ""
	}
	switch kind {
	case ColumnMoney, ColumnPercent:
		return strconv.FormatFloat(asFloat(value), 'f', 2, 64)
	case ColumnNumber:
		return strconv.FormatFloat(asFloat(value), 'f', -1, 64)
	case ColumnDate, ColumnMonth:
		if when, ok := value.(time.Time); ok {
			return formatTime(when, kind)
		}
		return fmt.Sprint(value)
	default:
		return fmt.Sprint(value)
	}
}

func formatTime(when time.Time, kind ColumnKind) string {
	if kind == ColumnMonth {
		return when.Format("2006-01")
	}
	return when.In(nexus.Location()).Format("2006-01-02")
}

// totalsLabel is the only string this package renders for a person, so it is
// the only one that needs translating here. Everything else on the sheet comes
// from the report's own Titles.
func totalsLabel(locale string) string {
	switch locale {
	case "en":
		return "Total"
	case "ru":
		return "Итого"
	case "zh":
		return "合计"
	case "ar":
		return "الإجمالي"
	case "fr":
		return "Total"
	case "es":
		return "Total"
	default:
		return "Нийт"
	}
}

// Filename builds the download name: the report key, the date, the extension.
// The report's localised title is deliberately not used — a filename with
// Cyrillic in it survives a browser and a mail client unevenly, and the key is
// what somebody searching their downloads folder a month later will remember.
func Filename(key string, format Format) string {
	safe := strings.NewReplacer(".", "-", "/", "-", " ", "-").Replace(key)
	return fmt.Sprintf("%s-%s.%s", safe, nexus.Now().Format("2006-01-02"), format)
}
