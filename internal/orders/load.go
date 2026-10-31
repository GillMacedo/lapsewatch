// Package orders finds customers who have stopped buying.
package orders

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const maxFileBytes = 50_000_000

// Order is one row. Refunds have negative totals.
type Order struct {
	Row      int
	Date     time.Time
	Customer string
	Cents    int64
}

// File is a loaded orders export.
type File struct {
	Orders     []Order
	Skipped    []int
	Columns    map[string]string
	DateFormat string
}

var fieldNames = map[string][]string{
	"date":     {"date", "order date", "created at", "paid at", "sale date", "transaction date", "invoice date", "data", "data pedido", "fecha"},
	"customer": {"customer email", "email", "billing email", "buyer email", "customer", "customer name", "customer id", "client", "client name", "billing name", "name", "buyer", "account", "cliente", "e-mail", "kunde"},
	"amount":   {"total", "order total", "amount", "total price", "net sales", "value", "revenue", "gross sales", "valor", "valor total", "importe", "betrag"},
}

var accents = strings.NewReplacer("á", "a", "à", "a", "â", "a", "ã", "a", "ç", "c", "é", "e", "ê", "e", "í", "i", "ó", "o", "ô", "o", "õ", "o", "ú", "u", "ü", "u", "ñ", "n")

func normalHeader(cell string) string {
	cell = accents.Replace(strings.ToLower(strings.TrimSpace(cell)))
	return strings.Join(strings.Fields(strings.NewReplacer("_", " ", ".", " ", ":", " ").Replace(cell)), " ")
}

func detect(header []string, overrides map[string]string) (map[string]int, error) {
	found := map[string]int{}
	normal := make([]string, len(header))
	for i, cell := range header {
		normal[i] = normalHeader(cell)
	}
	for field, names := range fieldNames {
		for _, name := range names {
			if _, set := found[field]; set {
				break
			}
			for i, cell := range normal {
				if cell == name {
					found[field] = i
					break
				}
			}
		}
	}
	for field, name := range overrides {
		if _, known := fieldNames[field]; !known {
			return nil, fmt.Errorf("unknown field %q. Use date, customer or amount", field)
		}
		index := -1
		for i, cell := range normal {
			if cell == normalHeader(name) {
				index = i
			}
		}
		if index < 0 {
			return nil, fmt.Errorf("no column named %q. Columns found: %s", name, strings.Join(header, ", "))
		}
		found[field] = index
	}
	return found, nil
}

// CustomerKey makes one customer match across rows, ignoring case and spacing.
func CustomerKey(customer string) string {
	return strings.Join(strings.Fields(strings.ToLower(customer)), " ")
}

// Load reads an orders CSV.
func Load(path string, overrides map[string]string, dateFormat string) (*File, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("can't read %s: %w", path, errors.Unwrap(err))
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%s is a folder. Pass a CSV file", path)
	}
	if info.Size() > maxFileBytes {
		return nil, fmt.Errorf("%s is larger than 50 MB", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("can't read %s: %w", path, err)
	}
	text := decode(data)
	reader := csv.NewReader(strings.NewReader(text))
	reader.Comma = detectDelimiter(text)
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("can't parse %s as CSV: %w", path, err)
	}

	index := -1
	var columns map[string]int
	for i := 0; i < len(rows) && i < 20; i++ {
		found, err := detect(rows[i], overrides)
		if err != nil {
			continue
		}
		_, date := found["date"]
		_, customer := found["customer"]
		_, amount := found["amount"]
		if date && customer && amount {
			index, columns = i, found
			break
		}
	}
	if index < 0 {
		if len(overrides) > 0 && len(rows) > 0 {
			if _, err := detect(rows[0], overrides); err != nil {
				return nil, err
			}
		}
		first := "(empty file)"
		if len(rows) > 0 {
			first = strings.Join(rows[0], ", ")
		}
		return nil, fmt.Errorf("can't find date, customer and amount columns in %s. First row: %s. Name them with a column map", path, first)
	}

	cell := func(row []string, field string) string {
		if at := columns[field]; at < len(row) {
			return strings.TrimSpace(row[at])
		}
		return ""
	}
	body := rows[index+1:]
	var dates []string
	for _, row := range body {
		dates = append(dates, cell(row, "date"))
	}
	if dateFormat == "" {
		dateFormat = DetectDateFormat(dates)
	}
	if dateFormat == "" {
		return nil, fmt.Errorf("the dates in %s could be day-first or month-first. Pass a date format of dmy or mdy", path)
	}
	file := &File{Skipped: []int{}, DateFormat: dateFormat, Columns: map[string]string{}}
	for field, at := range columns {
		file.Columns[field] = rows[index][at]
	}
	for offset, row := range body {
		number := index + offset + 2
		if strings.TrimSpace(strings.Join(row, "")) == "" {
			continue
		}
		date, okDate := ParseDate(cell(row, "date"), dateFormat)
		cents, okAmount := ParseAmount(cell(row, "amount"))
		customer := cell(row, "customer")
		if !okDate || !okAmount || customer == "" {
			file.Skipped = append(file.Skipped, number)
			continue
		}
		file.Orders = append(file.Orders, Order{Row: number, Date: date, Customer: customer, Cents: cents})
	}
	return file, nil
}
