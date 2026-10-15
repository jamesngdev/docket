package devdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// postgres runs every query as a single prepared statement inside a READ ONLY
// transaction that is rolled back afterwards, so writes, DDL and multi-statement
// tricks fail inside the database itself.
type postgres struct{ conn *pgx.Conn }

func openPostgres(ctx context.Context, raw string) (Engine, error) {
	conn, err := pgx.Connect(ctx, raw)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	return &postgres{conn: conn}, nil
}

func (p *postgres) Close() { _ = p.conn.Close(context.Background()) }

func (p *postgres) Tools() []Tool {
	return []Tool{
		{Name: "db_schema", Description: "PostgreSQL dev database structure. Without `table`: every table with its columns. " +
			"With `table` (e.g. users, \"User\" or public.orders): columns with types/defaults, constraints (PK, FK, unique) and indexes.",
			InputSchema: schema(map[string]any{"table": str("optional table name, schema-qualified or not")})},
		{Name: "sql_query", Description: fmt.Sprintf("Run ONE read-only SQL query (SELECT / WITH … SELECT / VALUES / TABLE) on the "+
			"PostgreSQL dev database. Writes are rejected. Returns one JSON object per row, at most %d rows — use WHERE, "+
			"LIMIT, COUNT/GROUP BY to stay small. Quote mixed-case identifiers (\"User\").", maxRows),
			InputSchema: schema(map[string]any{"sql": str("a single SELECT query, no trailing statements")}, "sql")},
	}
}

func (p *postgres) Call(ctx context.Context, tool string, raw json.RawMessage) (string, error) {
	var a struct{ SQL, Table string }
	if err := json.Unmarshal(raw, &a); err != nil {
		return "", fmt.Errorf("bad arguments: %w", err)
	}
	switch tool {
	case "sql_query":
		sql := strings.TrimRight(strings.TrimSpace(a.SQL), "; \n\t")
		if sql == "" {
			return "", errors.New("sql is required")
		}
		rows, more, err := p.rows(ctx, sql, maxRows)
		return render(rows, more), err
	case "db_schema":
		if strings.TrimSpace(a.Table) == "" {
			return p.tables(ctx)
		}
		return p.table(ctx, a.Table)
	}
	return "", fmt.Errorf("unknown tool %q", tool)
}

// rows wraps query so Postgres renders each row as JSON (exact type fidelity
// for uuid, numeric, jsonb, timestamps…) and caps the row count.
func (p *postgres) rows(ctx context.Context, query string, limit int, args ...any) ([]string, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	tx, err := p.conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, fmt.Sprintf("SET LOCAL statement_timeout = %d", queryTimeout.Milliseconds())); err != nil {
		return nil, false, err
	}
	wrapped := fmt.Sprintf("SELECT row_to_json(q)::text FROM (\n%s\n) q LIMIT %d", query, limit+1)
	rs, err := tx.Query(ctx, wrapped, args...)
	if err != nil {
		return nil, false, err
	}
	out, err := pgx.CollectRows(rs, pgx.RowTo[string])
	if err != nil {
		return nil, false, err
	}
	if len(out) > limit {
		return out[:limit], true, nil
	}
	return out, false, nil
}

func (p *postgres) tables(ctx context.Context) (string, error) {
	rows, more, err := p.rows(ctx, `
		SELECT c.table_schema || '.' || c.table_name AS table,
		       string_agg(c.column_name || ' ' || c.udt_name, ', ' ORDER BY c.ordinal_position) AS columns
		FROM information_schema.columns c
		WHERE c.table_schema NOT IN ('pg_catalog', 'information_schema') AND c.table_schema NOT LIKE 'pg_toast%'
		GROUP BY c.table_schema, c.table_name ORDER BY 1`, 500)
	return render(rows, more), err
}

// table describes one table; name may be schema-qualified and/or quoted.
func (p *postgres) table(ctx context.Context, name string) (string, error) {
	sch, tbl := "", strings.Trim(strings.TrimSpace(name), `"`)
	if i := strings.LastIndex(name, "."); i >= 0 {
		sch, tbl = strings.Trim(name[:i], `" `), strings.Trim(name[i+1:], `" `)
	}
	var oid uint32
	err := p.conn.QueryRow(ctx, `SELECT c.oid FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind IN ('r','p','v','m','f') AND (c.relname = $1 OR lower(c.relname) = lower($1)) AND ($2 = '' OR n.nspname = $2)
		ORDER BY c.relname = $1 DESC, n.nspname = 'public' DESC LIMIT 1`, tbl, sch).Scan(&oid)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("table %q not found (call db_schema without arguments to list tables)", name)
	}
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, sec := range []struct{ title, sql string }{
		{"columns", `SELECT a.attname AS column, format_type(a.atttypid, a.atttypmod) AS type, a.attnotnull AS not_null,
			pg_get_expr(d.adbin, d.adrelid) AS default FROM pg_attribute a
			LEFT JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
			WHERE a.attrelid = $1 AND a.attnum > 0 AND NOT a.attisdropped ORDER BY a.attnum`},
		{"constraints", `SELECT conname AS name, pg_get_constraintdef(oid) AS definition FROM pg_constraint WHERE conrelid = $1 ORDER BY contype, conname`},
		{"indexes", `SELECT pg_get_indexdef(indexrelid) AS definition FROM pg_index WHERE indrelid = $1`},
	} {
		rows, more, err := p.rows(ctx, sec.sql, 500, oid)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "## %s\n%s\n\n", sec.title, render(rows, more))
	}
	return b.String(), nil
}
