---
name: codegraph-usage
description: Read and understand the codebase through the codegraph MCP server instead of grep/read loops. Use before reading or changing any code.
---

# Reading code with codegraph

The repo is indexed by codegraph (`.codegraph/`). The `codegraph` MCP server is your primary code-reading tool.

1. Start every investigation with `codegraph_explore` (a natural-language question or symbol names). It returns
   the verbatim source of the relevant symbols plus call paths in one call. Treat that output as already read.
2. Open files directly only when codegraph has no index for them (new files, config, migrations).
3. Shell fallback when MCP is unavailable: `codegraph explore "<question or symbols>"`.
4. Before editing a symbol, check its blast radius: `codegraph impact <symbol>` / `codegraph callers <symbol>`.

Never dump whole directories to discover structure.
