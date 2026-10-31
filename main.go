// Command lapsewatch finds customers who have stopped buying.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/GillMacedo/lapsewatch/internal/orders"
	"github.com/GillMacedo/lapsewatch/internal/server"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const version = "0.1.0"

const usage = `Usage:
  lapsewatch check <orders.csv>   Find customers who stopped buying
  lapsewatch cohorts <orders.csv> Show repeat-order retention by first-order month
  lapsewatch serve                Run as an MCP server over stdio
  lapsewatch version

Options for check:
  --as-of 2026-12-31              Date to measure from (default: the newest order)
  --date-format dmy|mdy           Needed only when dates are ambiguous
  --columns date=When,customer=Who,amount=Sum   Name columns it doesn't recognize
  --top <n>                       Most customers to list per status (default 10)
  --json                          Print JSON

Reads your file and never changes it. Never uses the network.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stdout, usage)
		return 2
	}
	switch args[0] {
	case "cohorts":
		if err := cohorts(args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "lapsewatch: %v\n", err)
			return 2
		}
		return 0
	case "serve":
		if err := server.New(version).Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			fmt.Fprintf(stderr, "lapsewatch: %v\n", err)
			return 2
		}
		return 0
	case "version":
		fmt.Fprintln(stdout, version)
		return 0
	case "check":
		if err := check(args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "lapsewatch: %v\n", err)
			return 2
		}
		return 0
	}
	fmt.Fprint(stdout, usage)
	return 2
}

func cohorts(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("cohorts", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	asOfFlag := flags.String("as-of", "", "")
	dateFormat := flags.String("date-format", "", "")
	columnsFlag := flags.String("columns", "", "")
	asJSON := flags.Bool("json", false, "")
	var paths []string
	for rest := args; ; {
		if err := flags.Parse(rest); err != nil {
			return err
		}
		if flags.NArg() == 0 {
			break
		}
		paths = append(paths, flags.Arg(0))
		rest = flags.Args()[1:]
	}
	if len(paths) != 1 {
		return fmt.Errorf("cohorts takes one CSV file\n%s", usage)
	}
	if *dateFormat != "" && *dateFormat != "dmy" && *dateFormat != "mdy" {
		return fmt.Errorf("--date-format must be dmy or mdy")
	}
	overrides := map[string]string{}
	if *columnsFlag != "" {
		for _, pair := range strings.Split(*columnsFlag, ",") {
			field, name, ok := strings.Cut(pair, "=")
			if !ok {
				return fmt.Errorf("--columns needs field=Column pairs, not %q", pair)
			}
			overrides[strings.TrimSpace(field)] = strings.TrimSpace(name)
		}
	}
	var asOf time.Time
	if *asOfFlag != "" {
		parsed, err := time.Parse("2006-01-02", *asOfFlag)
		if err != nil {
			return fmt.Errorf("--as-of must look like 2026-12-31, not %q", *asOfFlag)
		}
		asOf = parsed
	}
	file, err := orders.Load(paths[0], overrides, *dateFormat)
	if err != nil {
		return err
	}
	result, err := orders.Cohorts(file.Orders, asOf)
	if err != nil {
		return err
	}
	if *asJSON {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	}
	maxAge := 0
	for _, cohort := range result.Cohorts {
		if len(cohort.Retention)-1 > maxAge {
			maxAge = len(cohort.Retention) - 1
		}
	}
	fmt.Fprintf(stdout, "As of %s\n", result.AsOf)
	writer := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprint(writer, "First order month\tCustomers")
	for age := 0; age <= maxAge; age++ {
		fmt.Fprintf(writer, "\tM%d", age)
	}
	fmt.Fprintln(writer)
	for _, cohort := range result.Cohorts {
		fmt.Fprintf(writer, "%s\t%d", cohort.FirstOrderMonth, cohort.Customers)
		for age := 0; age <= maxAge; age++ {
			if age >= len(cohort.Retention) {
				fmt.Fprint(writer, "\t—")
			} else {
				fmt.Fprintf(writer, "\t%.1f%%", cohort.Retention[age])
			}
		}
		fmt.Fprintln(writer)
	}
	return writer.Flush()
}


func check(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	asOfFlag := flags.String("as-of", "", "")
	dateFormat := flags.String("date-format", "", "")
	columnsFlag := flags.String("columns", "", "")
	top := flags.Int("top", 10, "")
	asJSON := flags.Bool("json", false, "")

	// The standard flag package stops at the first path, so options can't
	// follow it without this loop.
	var paths []string
	for rest := args; ; {
		if err := flags.Parse(rest); err != nil {
			return err
		}
		if flags.NArg() == 0 {
			break
		}
		paths = append(paths, flags.Arg(0))
		rest = flags.Args()[1:]
	}
	if len(paths) != 1 {
		return fmt.Errorf("check takes one CSV file\n%s", usage)
	}
	if *dateFormat != "" && *dateFormat != "dmy" && *dateFormat != "mdy" {
		return fmt.Errorf("--date-format must be dmy or mdy")
	}
	if *top < 0 {
		return fmt.Errorf("--top can't be negative")
	}
	overrides := map[string]string{}
	if *columnsFlag != "" {
		for _, pair := range strings.Split(*columnsFlag, ",") {
			field, name, ok := strings.Cut(pair, "=")
			if !ok {
				return fmt.Errorf("--columns needs field=Column pairs, not %q", pair)
			}
			overrides[strings.TrimSpace(field)] = strings.TrimSpace(name)
		}
	}
	settings := orders.Defaults()
	if *asOfFlag != "" {
		asOf, err := time.Parse("2006-01-02", *asOfFlag)
		if err != nil {
			return fmt.Errorf("--as-of must look like 2026-12-31, not %q", *asOfFlag)
		}
		settings.AsOf = asOf
	}
	file, err := orders.Load(paths[0], overrides, *dateFormat)
	if err != nil {
		return err
	}
	result, err := orders.Check(file.Orders, settings)
	if err != nil {
		return err
	}
	if *asJSON {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(struct {
			orders.Result
			SkippedRows []int `json:"skipped_rows"`
		}{result, file.Skipped})
	}

	fmt.Fprintf(stdout, "As of %s: %d customers, %.1f%% bought more than once, %s revenue, top 10%% bring in %.1f%%\n", result.AsOf, result.Customers, result.RepeatRate, result.Revenue, result.Top10Share)
	section := func(title, status string) {
		printed := 0
		for _, c := range result.List {
			if c.Status != status {
				continue
			}
			if printed == 0 {
				fmt.Fprintf(stdout, "%s (%d)\n", title, result.Counts[status])
			}
			if printed < *top {
				if c.UsualGap > 0 {
					fmt.Fprintf(stdout, "  %s  %d days since last order, usually every %.0f  worth %s a year\n", c.Customer, c.DaysSince, c.UsualGap, c.YearlyValue)
				} else {
					fmt.Fprintf(stdout, "  %s  %d days since their only order  spent %s\n", c.Customer, c.DaysSince, c.Total)
				}
			}
			printed++
		}
		if printed > *top {
			fmt.Fprintf(stdout, "  and %d more\n", printed-*top)
		}
	}
	section("AT RISK: contact these first", orders.AtRisk)
	section("LOST", orders.Lost)
	section("ONE-TIME", orders.OneTime)
	fmt.Fprintf(stdout, "%d active, %d new.\n", result.Counts[orders.Active], result.Counts[orders.New])
	for _, row := range file.Skipped {
		fmt.Fprintf(stdout, "SKIPPED row %d: no valid date, customer or amount\n", row)
	}
	fmt.Fprintf(stdout, "%s a year at risk from %d customers.\n", result.AtRiskValue, result.Counts[orders.AtRisk])
	return nil
}
