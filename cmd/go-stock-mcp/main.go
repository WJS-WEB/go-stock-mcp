package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"go-stock/backend/agent/tools"
	"go-stock/backend/logger"
)

const serverVersion = "0.1.0"

type config struct {
	dbPath string
}

type toolSpec struct {
	sourceName string
	mcpName    string
}

// Keep the first public MCP surface deliberately small and read-only. The
// source tools are existing go-stock Eino tools; the MCP names are stable,
// lower-snake-case names intended for clients such as DeepSeek Harness.
var publicToolSpecs = []toolSpec{
	{sourceName: "QueryStockCodeInfo", mcpName: "search_stock"},
	{sourceName: "QueryStockNewsTool", mcpName: "search_stock_news"},
	{sourceName: "QueryBKDictInfo", mcpName: "list_industries"},
	{sourceName: "GetStockInfo", mcpName: "get_stock_info"},
	{sourceName: "GetStockKLine", mcpName: "get_stock_kline"},
	{sourceName: "GetStockMinuteData", mcpName: "get_stock_minute_data"},
	{sourceName: "GetStockFinancialInfo", mcpName: "get_stock_financial_info"},
	{sourceName: "GetStockMoneyData", mcpName: "get_stock_money_data"},
	{sourceName: "GetMarketData", mcpName: "get_market_data"},
	{sourceName: "GetIndustryMoneyRank", mcpName: "get_industry_money_rank"},
	{sourceName: "GetFundInfo", mcpName: "get_fund_info"},
	{sourceName: "GetInvestCalendar", mcpName: "get_invest_calendar"},
	{sourceName: "SearchReport", mcpName: "search_reports"},
	{sourceName: "IsTradingDay", mcpName: "is_trading_day"},
}

func main() {
	cfg := parseConfig()
	configureRuntimeLogging()

	if err := openReadOnlyDatabase(cfg.dbPath); err != nil {
		fmt.Fprintf(os.Stderr, "go-stock-mcp: database initialization failed: %v\n", err)
		os.Exit(1)
	}

	bridge, err := newToolBridge(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "go-stock-mcp: tool initialization failed: %v\n", err)
		os.Exit(1)
	}

	mcpServer := server.NewMCPServer(
		"go-stock-mcp",
		serverVersion,
		server.WithToolCapabilities(false),
	)
	for _, registered := range bridge {
		mcpServer.AddTool(registered.definition, registered.handler)
	}

	// MCP stdio owns stdout. Do not write logs or diagnostics there.
	if err := server.ServeStdio(mcpServer); err != nil {
		fmt.Fprintf(os.Stderr, "go-stock-mcp: server stopped: %v\n", err)
	}
}

func parseConfig() config {
	defaultDB := strings.TrimSpace(os.Getenv("GO_STOCK_DB_PATH"))
	if defaultDB == "" {
		defaultDB = "data/stock.db"
	}

	flag.CommandLine.SetOutput(os.Stderr)
	dbPath := flag.String("db", defaultDB, "path to the go-stock SQLite database")
	version := flag.Bool("version", false, "print the MCP server version")
	flag.Parse()
	if *version {
		fmt.Fprintln(os.Stderr, serverVersion)
		os.Exit(0)
	}
	return config{dbPath: *dbPath}
}

// logger is initialized by package init with os.Stdout. Reinitialize it after
// switching the console sink to stderr so Eino tools cannot corrupt MCP JSON-RPC.
func configureRuntimeLogging() {
	stdout := os.Stdout
	os.Stdout = os.Stderr
	logger.InitLogger()
	os.Stdout = stdout
}

type registeredTool struct {
	definition mcp.Tool
	handler    server.ToolHandlerFunc
}

func newToolBridge(ctx context.Context) ([]registeredTool, error) {
	available := make(map[string]tool.InvokableTool)
	for _, candidate := range allEinoTools() {
		info, err := candidate.Info(ctx)
		if err != nil {
			return nil, fmt.Errorf("read tool info: %w", err)
		}
		invokable, ok := candidate.(tool.InvokableTool)
		if ok {
			available[info.Name] = invokable
		}
	}

	registered := make([]registeredTool, 0, len(publicToolSpecs))
	for _, spec := range publicToolSpecs {
		source, ok := available[spec.sourceName]
		if !ok {
			return nil, fmt.Errorf("public tool %q is not available in go-stock", spec.sourceName)
		}

		info, err := source.Info(ctx)
		if err != nil {
			return nil, fmt.Errorf("read %s info: %w", spec.sourceName, err)
		}
		rawSchema, err := toolInputSchema(info)
		if err != nil {
			return nil, fmt.Errorf("convert %s schema: %w", spec.sourceName, err)
		}

		definition := mcp.NewToolWithRawSchema(
			spec.mcpName,
			info.Desc+" 该工具为只读能力，不修改 go-stock 本地数据库。",
			rawSchema,
		)
		definition.Annotations.ReadOnlyHint = mcp.ToBoolPtr(true)
		definition.Annotations.DestructiveHint = mcp.ToBoolPtr(false)
		definition.Annotations.IdempotentHint = mcp.ToBoolPtr(true)
		definition.Annotations.OpenWorldHint = mcp.ToBoolPtr(true)

		registered = append(registered, registeredTool{
			definition: definition,
			handler:    invokeEinoTool(source),
		})
	}

	return registered, nil
}

func allEinoTools() []tool.BaseTool {
	all := []tool.BaseTool{
		tools.GetQueryStockCodeInfoTool(),
		tools.GetQueryStockNewsTool(),
		tools.GetQueryBKDictTool(),
	}
	all = append(all, tools.GetAllDataTools()...)
	all = append(all, tools.GetHolidayTools()...)
	return all
}

func toolInputSchema(info *schema.ToolInfo) ([]byte, error) {
	if info.ParamsOneOf == nil {
		return []byte(`{"type":"object","properties":{}}`), nil
	}
	schemaValue, err := info.ParamsOneOf.ToJSONSchema()
	if err != nil {
		return nil, err
	}
	if schemaValue == nil {
		return []byte(`{"type":"object","properties":{}}`), nil
	}
	return json.Marshal(schemaValue)
}

func invokeEinoTool(source tool.InvokableTool) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		rawArguments := request.GetRawArguments()
		var args []byte
		if rawArguments == nil {
			args = []byte(`{}`)
		} else {
			encoded, err := json.Marshal(rawArguments)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid tool arguments: %v", err)), nil
			}
			args = encoded
		}

		result, err := source.InvokableRun(ctx, string(args))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(result), nil
	}
}
