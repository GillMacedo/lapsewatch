// Package server exposes the customer check as MCP tools.
package server

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/GillMacedo/lapsewatch/internal/orders"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// CheckInput is the input of customers_check_lapsed.
type CheckInput struct {
	Path       string            `json:"path" jsonschema:"Path to the orders CSV"`
	AsOf       string            `json:"as_of,omitempty" jsonschema:"Date to measure from, YYYY-MM-DD. Default is the newest order"`
	DateFormat string            `json:"date_format,omitempty" jsonschema:"dmy or mdy, needed only if the tool reports that dates are ambiguous"`
	Columns    map[string]string `json:"columns,omitempty" jsonschema:"Column names by field (date, customer, amount), when detection fails"`
	Limit      *int              `json:"limit,omitempty" jsonschema:"Most customers to return, highest priority first, default 100. 0 returns only the summary"`
}

// CheckOutput is the output of customers_check_lapsed.
type CheckOutput struct {
	orders.Result
	Truncated   bool  `json:"truncated"`
	SkippedRows []int `json:"skipped_rows"`
}

// InspectInput is the input of customers_inspect_csv.
type InspectInput struct {
	Path string `json:"path" jsonschema:"Path to the orders CSV"`
}

// InspectOutput holds no row contents.
type InspectOutput struct {
	Columns map[string]string `json:"columns"`
	Orders  int               `json:"orders"`
	Skipped int               `json:"skipped"`
}

func no() *bool { value := false; return &value }

// New returns the MCP server.
func New(version string) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "lapsewatch", Version: version}, nil)
	annotations := &mcp.ToolAnnotations{ReadOnlyHint: true, DestructiveHint: no(), IdempotentHint: true, OpenWorldHint: no()}

	mcp.AddTool(server, &mcp.Tool{
		Name:  "customers_check_lapsed",
		Title: "Find customers who stopped buying",
		Description: "Finds at-risk and lost customers in an orders CSV by comparing each customer with their own buying pattern, with yearly revenue at risk, repeat rate and revenue concentration. " +
			"Use for customer retention, churn analysis, or win-back lists. Read-only, and never uses the network. Results include customer names or emails from the file, so they reach the model. It can't contact anyone.",
		Annotations: annotations,
	}, check)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "customers_inspect_csv",
		Title:       "Inspect an orders CSV",
		Description: "Shows which columns lapsewatch detects in an orders CSV and how many rows it can read, without returning row contents. Use if the file is unfamiliar or a check failed.",
		Annotations: annotations,
	}, inspect)
	return server
}

func check(_ context.Context, _ *mcp.CallToolRequest, in CheckInput) (*mcp.CallToolResult, CheckOutput, error) {
	limit := 100
	if in.Limit != nil {
		limit = *in.Limit
	}
	if limit < 0 || limit > 500 {
		return nil, CheckOutput{}, errors.New("limit must be between 0 and 500")
	}
	settings := orders.Defaults()
	if in.AsOf != "" {
		asOf, err := time.Parse("2006-01-02", in.AsOf)
		if err != nil {
			return nil, CheckOutput{}, fmt.Errorf("as_of must look like 2026-12-31, not %q", in.AsOf)
		}
		settings.AsOf = asOf
	}
	file, err := orders.Load(in.Path, in.Columns, in.DateFormat)
	if err != nil {
		return nil, CheckOutput{}, err
	}
	result, err := orders.Check(file.Orders, settings)
	if err != nil {
		return nil, CheckOutput{}, err
	}
	out := CheckOutput{Result: result, SkippedRows: file.Skipped}
	if len(out.List) > limit {
		out.List, out.Truncated = out.List[:limit], true
	}
	return nil, out, nil
}

func inspect(_ context.Context, _ *mcp.CallToolRequest, in InspectInput) (*mcp.CallToolResult, InspectOutput, error) {
	file, err := orders.Load(in.Path, nil, "dmy")
	if err != nil {
		return nil, InspectOutput{}, err
	}
	return nil, InspectOutput{Columns: file.Columns, Orders: len(file.Orders), Skipped: len(file.Skipped)}, nil
}
