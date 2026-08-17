# go-stock MCP Server + DeepSeek Harness Bundle

This repository contains a read-only MCP facade for the existing go-stock data
layer. It is designed for local stdio clients such as DeepSeek Harness,
MCP Inspector, and other MCP-compatible applications.

## Exposed tools

The first public surface contains fourteen read-only tools:

```text
search_stock
search_stock_news
list_industries
get_stock_info
get_stock_kline
get_stock_minute_data
get_stock_financial_info
get_stock_money_data
get_market_data
get_industry_money_rank
get_fund_info
get_invest_calendar
search_reports
is_trading_day
```

The MCP process opens the SQLite database with `mode=ro`. It does not run the
Wails application, auto-migrate tables, or expose stock-group, trading-record,
notification, prompt, or MCP-configuration write operations.

## Build the MCP executable

From the repository root:

```powershell
go build -o build/go-stock-mcp.exe ./cmd/go-stock-mcp
```

The executable accepts either an environment variable or a flag:

```powershell
$env:GO_STOCK_DB_PATH = 'D:\path\to\stock.db'
build\go-stock-mcp.exe
```

The server uses stdout exclusively for MCP JSON-RPC. Diagnostics and tool logs
are written to stderr and the existing log files.

## DeepSeek Harness installation

The root `package.json` declares a Harness Bundle. After installing the
`go-stock-mcp` executable and putting it on `PATH`, install the Bundle into a
profile:

```powershell
dsh plugin --profile demo add github:WJS-WEB/go-stock-mcp
dsh --profile demo
```

For a local checkout, use:

```powershell
dsh plugin --profile demo add .
```

Before starting Harness, set the database path if the profile's working
directory is not the go-stock repository:

```powershell
$env:GO_STOCK_DB_PATH = 'D:\path\to\go-stock\data\stock.db'
$env:GO_STOCK_ROOT = 'D:\path\to\go-stock'
$env:GO_STOCK_MCP_COMMAND = 'D:\path\to\go-stock\build\go-stock-mcp.exe'
```

The Harness client namespaces tools as `mcp__go-stock__<tool_name>`.

## Distribution notes

The Bundle and executable are separate distribution artifacts. A GitHub
installation of the Bundle does not automatically install a platform-specific
Go executable. Releases should publish Windows, Linux, and macOS binaries with
checksums, while the Bundle keeps the stable MCP configuration.

Do not commit `data/stock.db`, API keys, HTTP headers, or other credentials.
