package orders

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func date(text string) time.Time {
	parsed, err := time.Parse("2006-01-02", text)
	if err != nil {
		panic(err)
	}
	return parsed
}

// every makes count orders, gap days apart, ending lastAgo days before 2026-12-31.
func every(who string, gap, lastAgo, count int, cents int64) []Order {
	var out []Order
	for i := 0; i < count; i++ {
		out = append(out, Order{Date: date("2026-12-31").AddDate(0, 0, -(lastAgo + gap*i)), Customer: who, Cents: cents})
	}
	return out
}

func asOf() Settings {
	s := Defaults()
	s.AsOf = date("2026-12-31")
	return s
}

func statusOf(t *testing.T, orders []Order, s Settings) Customer {
	t.Helper()
	r, err := Check(orders, s)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.List) != 1 {
		t.Fatalf("got %+v", r.List)
	}
	return r.List[0]
}

func TestStatusesFollowEachCustomersOwnPattern(t *testing.T) {
	cases := []struct {
		name            string
		gap, lastAgo, n int
		want            string
	}{
		{"on schedule", 30, 10, 6, Active},
		{"exactly twice the gap is still active", 30, 60, 6, Active},
		{"past twice the gap", 30, 61, 6, AtRisk},
		{"exactly four times", 30, 120, 6, AtRisk},
		{"past four times", 30, 121, 6, Lost},
		{"quarterly buyer at 90 days is fine", 90, 90, 6, Active},
		{"weekly buyer needs 30 days for risk", 7, 20, 6, Active},
		{"weekly buyer after 31 days", 7, 31, 6, Lost},
	}
	for _, c := range cases {
		if got := statusOf(t, every("a@example.com", c.gap, c.lastAgo, c.n, 1000), asOf()); got.Status != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got.Status, c.want)
		}
	}
}

func TestSingleOrders(t *testing.T) {
	if got := statusOf(t, every("a", 1, 90, 1, 100), asOf()); got.Status != New {
		t.Errorf("got %+v", got)
	}
	if got := statusOf(t, every("a", 1, 91, 1, 100), asOf()); got.Status != OneTime {
		t.Errorf("got %+v", got)
	}
}

// Worked by hand: 6 orders of 10.00 every 30 days. Yearly value is
// 10.00 x 365 / 30 = 121.67.
func TestYearlyValueByHand(t *testing.T) {
	got := statusOf(t, every("a", 30, 70, 6, 1000), asOf())
	if got.YearlyValue != "121.67" || got.UsualGap != 30 || got.Total != "60.00" {
		t.Errorf("got %+v", got)
	}
	r, _ := Check(every("a", 30, 70, 6, 1000), asOf())
	if r.AtRiskValue != "121.67" {
		t.Errorf("got %+v", r)
	}
}

func TestRowsOnOneDayAreOneOrderAndRefundsArent(t *testing.T) {
	orders := append(every("a", 30, 5, 3, 1000), Order{Date: date("2026-12-26"), Customer: "a", Cents: 500}, Order{Date: date("2026-12-30"), Customer: "a", Cents: -300})
	got := statusOf(t, orders, asOf())
	if got.Orders != 3 || got.Total != "32.00" || got.LastOrder != "2026-12-26" {
		t.Errorf("got %+v", got)
	}
}

func TestCustomerMatchingIgnoresCaseAndSpaces(t *testing.T) {
	got := statusOf(t, []Order{{Date: date("2026-12-01"), Customer: "A@Example.com", Cents: 100}, {Date: date("2026-12-20"), Customer: " a@example.com", Cents: 100}}, asOf())
	if got.Orders != 2 {
		t.Errorf("got %+v", got)
	}
}


func TestCohortsWorkedByHand(t *testing.T) {
	orders := []Order{
		{Date: date("2026-01-02"), Customer: "a@example.com", Cents: 1000},
		{Date: date("2026-01-20"), Customer: " A@example.com ", Cents: 500},
		{Date: date("2026-02-03"), Customer: "a@example.com", Cents: 1000},
		{Date: date("2026-03-04"), Customer: "a@example.com", Cents: 1000},
		{Date: date("2026-01-05"), Customer: "b@example.com", Cents: 1000},
		{Date: date("2026-03-06"), Customer: "b@example.com", Cents: 1000},
		{Date: date("2026-02-10"), Customer: "c@example.com", Cents: 1000},
		{Date: date("2026-04-01"), Customer: "c@example.com", Cents: 1000},
		{Date: date("2026-03-12"), Customer: "d@example.com", Cents: 1000},
		{Date: date("2026-03-15"), Customer: "d@example.com", Cents: -200},
	}
	got, err := Cohorts(orders, date("2026-03-31"))
	if err != nil {
		t.Fatal(err)
	}
	want := []Cohort{
		{FirstOrderMonth: "2026-01", Customers: 2, Retention: []float64{100, 50, 100}},
		{FirstOrderMonth: "2026-02", Customers: 1, Retention: []float64{100, 0}},
		{FirstOrderMonth: "2026-03", Customers: 1, Retention: []float64{100}},
	}
	if fmt.Sprint(got.Cohorts) != fmt.Sprint(want) {
		t.Fatalf("got %+v, want %+v", got.Cohorts, want)
	}
	if got.AsOf != "2026-03-31" {
		t.Fatalf("AsOf = %s", got.AsOf)
	}
}

func TestCohortsRejectNoPositiveOrders(t *testing.T) {
	if _, err := Cohorts([]Order{{Date: date("2026-01-01"), Customer: "a", Cents: -1}}, date("2026-01-31")); err == nil {
		t.Fatal("expected an error for data with no positive orders")
	}
}

func TestSummary(t *testing.T) {
	var orders []Order
	orders = append(orders, every("big", 30, 5, 10, 10000)...)
	for i := 0; i < 9; i++ {
		orders = append(orders, every(fmt.Sprintf("small%d", i), 1, 200, 1, 1000)...)
	}
	r, err := Check(orders, asOf())
	if err != nil {
		t.Fatal(err)
	}
	// 10 customers, 1 repeat. Revenue is 1000 + 90 = 1090, and the top one is 91.7%.
	if r.Customers != 10 || r.RepeatRate != 10 || r.Revenue != "1090.00" || r.Top10Share != 91.7 || r.Counts[OneTime] != 9 {
		t.Errorf("got %+v", r)
	}
}

func TestPriorityPutsAtRiskFirstByValue(t *testing.T) {
	var orders []Order
	orders = append(orders, every("active", 30, 1, 5, 99900)...)
	orders = append(orders, every("risk-small", 30, 70, 5, 100)...)
	orders = append(orders, every("risk-big", 30, 70, 5, 9900)...)
	orders = append(orders, every("lost", 30, 500, 5, 5000)...)
	r, _ := Check(orders, asOf())
	var names []string
	for _, c := range r.List {
		names = append(names, c.Customer)
	}
	if !slices.Equal(names, []string{"risk-big", "risk-small", "lost", "active"}) {
		t.Errorf("got %v", names)
	}
}

func TestDefaultAsOfIsTheNewestOrderAndFutureOrdersAreIgnored(t *testing.T) {
	orders := every("a", 30, 0, 3, 100)
	r, _ := Check(orders, Defaults())
	if r.AsOf != "2026-12-31" || r.List[0].DaysSince != 0 {
		t.Errorf("got %+v", r)
	}
	s := Defaults()
	s.AsOf = date("2026-12-01")
	r, _ = Check(orders, s)
	if r.List[0].Orders != 2 {
		t.Errorf("got %+v", r.List[0])
	}
}

func TestBadInput(t *testing.T) {
	if _, err := Check(nil, Defaults()); err == nil {
		t.Error("no orders should fail")
	}
	bad := Defaults()
	bad.LostFactor = 1
	if _, err := Check(every("a", 1, 1, 2, 1), bad); err == nil {
		t.Error("bad factors should fail")
	}
}

func TestInvariantsOnRandomData(t *testing.T) {
	random := rand.New(rand.NewSource(11))
	for round := 0; round < 400; round++ {
		var orders []Order
		for i := 0; i < 1+random.Intn(100); i++ {
			orders = append(orders, Order{Date: date("2025-01-01").AddDate(0, 0, random.Intn(700)), Customer: fmt.Sprintf("c%d", random.Intn(12)), Cents: int64(random.Intn(5000) - 500)})
		}
		r, err := Check(orders, Defaults())
		if err != nil {
			t.Fatal(err)
		}
		sum := 0
		for _, count := range r.Counts {
			sum += count
		}
		if sum != r.Customers || len(r.List) != r.Customers {
			t.Fatalf("counts don't add up: %+v", r)
		}
		for _, c := range r.List {
			if c.DaysSince < 0 || c.Orders < 1 {
				t.Fatalf("bad customer %+v", c)
			}
		}
		if r.Top10Share < 0 && r.Revenue[0] != '-' {
			t.Fatalf("bad share %+v", r)
		}
		if fmt.Sprint(r) != fmt.Sprint(mustCheck(t, orders)) {
			t.Fatal("not deterministic")
		}
	}
}

func mustCheck(t *testing.T, orders []Order) Result {
	t.Helper()
	r, err := Check(orders, Defaults())
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func write(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "file.csv")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad(t *testing.T) {
	file, err := Load(write(t, "Report,,\nData;Cliente;Valor\n25/02/2026;Ana;1.234,56\nTotal;;\n"), nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Orders) != 1 || file.Orders[0].Cents != 123456 || file.DateFormat != "dmy" || !slices.Equal(file.Skipped, []int{4}) {
		t.Errorf("got %+v", file)
	}
	ambiguous := write(t, "Date,Email,Total\n03/02/2026,a,1\n04/02/2026,a,1\n")
	if _, err := Load(ambiguous, nil, ""); err == nil || !strings.Contains(err.Error(), "dmy or mdy") {
		t.Errorf("got %v", err)
	}
	unknown := write(t, "Foo,Bar\n1,2\n")
	if _, err := Load(unknown, nil, ""); err == nil || !strings.Contains(err.Error(), "First row: Foo, Bar") {
		t.Errorf("got %v", err)
	}
	if _, err := Load(unknown, map[string]string{"date": "Nope"}, ""); err == nil || !strings.Contains(err.Error(), "Columns found: Foo, Bar") {
		t.Errorf("got %v", err)
	}
	custom := write(t, "When,Who,Sum\n2026-03-02,a,5\n")
	if file, err := Load(custom, map[string]string{"date": "When", "customer": "Who", "amount": "Sum"}, ""); err != nil || len(file.Orders) != 1 {
		t.Errorf("%v %+v", err, file)
	}
	if _, err := Load("/no/such.csv", nil, ""); err == nil {
		t.Error("missing file should fail")
	}
	if _, err := Load(t.TempDir(), nil, ""); err == nil {
		t.Error("folder should fail")
	}
}
