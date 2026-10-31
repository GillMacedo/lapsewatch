package orders

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// Statuses.
const (
	Active  = "active"
	AtRisk  = "at risk"
	Lost    = "lost"
	New     = "new"
	OneTime = "one-time"
)

// Settings are the thresholds, in days unless named otherwise.
type Settings struct {
	AsOf        time.Time
	RiskFactor  float64
	LostFactor  float64
	MinRiskDays float64
	NewDays     float64
}

// Defaults flags a repeat customer once they've gone twice their usual gap.
func Defaults() Settings {
	return Settings{RiskFactor: 2, LostFactor: 4, MinRiskDays: 30, NewDays: 90}
}

// Customer is the result for one customer.
type Customer struct {
	Customer    string  `json:"customer"`
	Status      string  `json:"status"`
	Orders      int     `json:"orders"`
	Total       string  `json:"total"`
	LastOrder   string  `json:"last_order"`
	DaysSince   int     `json:"days_since_last_order"`
	UsualGap    float64 `json:"usual_days_between_orders,omitempty"`
	YearlyValue string  `json:"yearly_value,omitempty"`
	totalCents  int64
	yearlyCents int64
}

// Result summarizes the whole file.
type Result struct {
	AsOf        string         `json:"as_of"`
	Customers   int            `json:"customers"`
	RepeatRate  float64        `json:"repeat_rate_percent"`
	Revenue     string         `json:"revenue"`
	Top10Share  float64        `json:"top_10_percent_revenue_share"`
	Counts      map[string]int `json:"counts"`
	AtRiskValue string         `json:"at_risk_yearly_value"`
	List        []Customer     `json:"customers_by_priority"`
}

func formatCents(cents int64) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	return fmt.Sprintf("%s%d.%02d", sign, cents/100, cents%100)
}

func median(values []float64) float64 {
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[middle]
	}
	return (sorted[middle-1] + sorted[middle]) / 2
}


type Cohort struct {
	FirstOrderMonth string    `json:"first_order_month"`
	Customers       int       `json:"customers"`
	Retention       []float64 `json:"retention_percent"`
}

type CohortResult struct {
	AsOf    string   `json:"as_of"`
	Cohorts []Cohort `json:"cohorts"`
}

type cohortHistory struct {
	first time.Time
	months map[int]struct{}
}

type cohortGroup struct {
	customers int
	activeByMonth map[int]int
}

func monthNumber(day time.Time) int {
	return day.Year()*12 + int(day.Month()) - 1
}

// Cohorts groups customers by the month of their first positive order.
// Retention at age N is the share of the cohort with a positive order in the
// Nth calendar month after acquisition; age zero is 100% by definition.
func Cohorts(orders []Order, asOf time.Time) (CohortResult, error) {
	if asOf.IsZero() {
		for _, order := range orders {
			if order.Date.After(asOf) {
				asOf = order.Date
			}
		}
	}
	if asOf.IsZero() {
		return CohortResult{}, fmt.Errorf("the file has no readable orders")
	}
	cutoffMonth := monthNumber(asOf)
	people := map[string]*cohortHistory{}
	for _, order := range orders {
		if order.Date.After(asOf) || order.Cents <= 0 {
			continue
		}
		key := CustomerKey(order.Customer)
		person := people[key]
		if person == nil {
			person = &cohortHistory{first: order.Date, months: map[int]struct{}{}}
			people[key] = person
		}
		if order.Date.Before(person.first) {
			person.first = order.Date
		}
		person.months[monthNumber(order.Date)] = struct{}{}
	}
	if len(people) == 0 {
		return CohortResult{}, fmt.Errorf("the file has no readable orders")
	}
	groups := map[int]*cohortGroup{}
	for _, person := range people {
		firstMonth := monthNumber(person.first)
		group := groups[firstMonth]
		if group == nil {
			group = &cohortGroup{activeByMonth: map[int]int{}}
			groups[firstMonth] = group
		}
		group.customers++
		for month := range person.months {
			if month >= firstMonth && month <= cutoffMonth {
				group.activeByMonth[month-firstMonth]++
			}
		}
	}
	firstMonths := make([]int, 0, len(groups))
	for month := range groups {
		firstMonths = append(firstMonths, month)
	}
	sort.Ints(firstMonths)
	result := CohortResult{AsOf: asOf.Format("2006-01-02"), Cohorts: make([]Cohort, 0, len(firstMonths))}
	for _, firstMonth := range firstMonths {
		group := groups[firstMonth]
		retention := make([]float64, cutoffMonth-firstMonth+1)
		for age := range retention {
			retention[age] = math.Round(float64(group.activeByMonth[age])*1000/float64(group.customers)) / 10
		}
		result.Cohorts = append(result.Cohorts, Cohort{
			FirstOrderMonth: time.Date(firstMonth/12, time.Month(firstMonth%12+1), 1, 0, 0, 0, 0, time.UTC).Format("2006-01"),
			Customers: group.customers,
			Retention: retention,
		})
	}
	return result, nil
}

// Check sorts customers by how they're buying compared with their own pattern.
// A customer who usually orders every 30 days is at risk after 60 days, and one
// who orders every 90 days isn't, so one fixed cut-off wouldn't work.
func Check(orders []Order, settings Settings) (Result, error) {
	if settings.RiskFactor <= 1 || settings.LostFactor <= settings.RiskFactor || settings.MinRiskDays < 0 || settings.NewDays < 0 {
		return Result{}, fmt.Errorf("the lost factor must be above the risk factor, the risk factor above 1, and day settings can't be negative")
	}
	if len(orders) == 0 {
		return Result{}, fmt.Errorf("the file has no readable orders")
	}
	asOf := settings.AsOf
	if asOf.IsZero() {
		for _, order := range orders {
			if order.Date.After(asOf) {
				asOf = order.Date
			}
		}
	}

	type history struct {
		name  string
		days  map[time.Time]int64
		total int64
	}
	people := map[string]*history{}
	for _, order := range orders {
		if order.Date.After(asOf) {
			continue
		}
		key := CustomerKey(order.Customer)
		person := people[key]
		if person == nil {
			person = &history{name: order.Customer, days: map[time.Time]int64{}}
			people[key] = person
		}
		person.total += order.Cents
		// Exports often list one row per product. Several rows on a day are one
		// visit, and a refund isn't a visit at all.
		if order.Cents > 0 {
			person.days[order.Date] += order.Cents
		}
	}

	result := Result{AsOf: asOf.Format("2006-01-02"), Counts: map[string]int{Active: 0, AtRisk: 0, Lost: 0, New: 0, OneTime: 0}, List: []Customer{}}
	var revenue, riskValue int64
	repeat := 0
	var totals []int64
	for _, person := range people {
		if len(person.days) == 0 {
			continue
		}
		days := make([]time.Time, 0, len(person.days))
		for day := range person.days {
			days = append(days, day)
		}
		sort.Slice(days, func(i, j int) bool { return days[i].Before(days[j]) })
		last := days[len(days)-1]
		since := asOf.Sub(last).Hours() / 24
		customer := Customer{Customer: person.name, Orders: len(days), Total: formatCents(person.total), LastOrder: last.Format("2006-01-02"), DaysSince: int(since), totalCents: person.total}
		if len(days) == 1 {
			customer.Status = OneTime
			if since <= settings.NewDays {
				customer.Status = New
			}
		} else {
			repeat++
			gaps := make([]float64, 0, len(days)-1)
			for i := 1; i < len(days); i++ {
				gaps = append(gaps, days[i].Sub(days[i-1]).Hours()/24)
			}
			gap := median(gaps)
			customer.UsualGap = gap
			customer.yearlyCents = int64(math.Round(float64(person.total) / float64(len(days)) * 365 / gap))
			if customer.yearlyCents < 0 {
				customer.yearlyCents = 0
			}
			customer.YearlyValue = formatCents(customer.yearlyCents)
			switch {
			case since > math.Max(gap*settings.LostFactor, settings.MinRiskDays):
				customer.Status = Lost
			case since > math.Max(gap*settings.RiskFactor, settings.MinRiskDays):
				customer.Status = AtRisk
				riskValue += customer.yearlyCents
			default:
				customer.Status = Active
			}
		}
		result.Counts[customer.Status]++
		revenue += person.total
		totals = append(totals, person.total)
		result.List = append(result.List, customer)
	}
	result.Customers = len(result.List)
	if result.Customers > 0 {
		result.RepeatRate = math.Round(float64(repeat)/float64(result.Customers)*1000) / 10
	}
	result.Revenue = formatCents(revenue)
	result.AtRiskValue = formatCents(riskValue)
	sort.Slice(totals, func(i, j int) bool { return totals[i] > totals[j] })
	top := int(math.Ceil(float64(len(totals)) / 10))
	var topSum int64
	for _, total := range totals[:top] {
		topSum += total
	}
	if revenue > 0 {
		result.Top10Share = math.Round(float64(topSum)/float64(revenue)*1000) / 10
	}

	rank := map[string]int{AtRisk: 0, Lost: 1, OneTime: 2, New: 3, Active: 4}
	sort.Slice(result.List, func(i, j int) bool {
		a, b := result.List[i], result.List[j]
		if rank[a.Status] != rank[b.Status] {
			return rank[a.Status] < rank[b.Status]
		}
		if a.yearlyCents != b.yearlyCents {
			return a.yearlyCents > b.yearlyCents
		}
		if a.totalCents != b.totalCents {
			return a.totalCents > b.totalCents
		}
		return a.Customer < b.Customer
	})
	return result, nil
}
