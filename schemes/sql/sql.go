package sql

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/isobit/ndog"
)

var Scheme = &ndog.Scheme{
	Names:   []string{"postgresql", "postgres"},
	Connect: Connect,
}

func splitStatements(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF {
		if len(data) == 0 {
			return 0, nil, nil
		}
		return len(data), data, nil
	}
	if i := bytes.IndexByte(data, ';'); i >= 0 {
		return i + 1, data[:i+1], nil
	}
	return 0, nil, nil
}

type Options struct {
	JSON bool
}

func ExtractOptions(cfg ndog.Config) (Options, error) {
	opts := Options{}
	if _, ok := cfg.PopOption("json"); ok {
		opts.JSON = true
	}
	return opts, cfg.CheckRemainingOptions()
}

func Connect(cfg ndog.Config) error {
	opts, err := ExtractOptions(cfg)
	if err != nil {
		return err
	}

	ctx := context.Background()
	db, err := sql.Open("pgx", cfg.URL.String())
	if err != nil {
		return err
	}
	defer db.Close()

	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	name := cfg.URL.Host
	ndog.Logf(0, "connected: %s", name)

	stream := cfg.NewStream(name)
	defer stream.Close()

	scanner := bufio.NewScanner(stream)
	scanner.Split(splitStatements)
	for scanner.Scan() {
		stmt := scanner.Text()
		ndog.Logf(1, "execute: %s", strconv.Quote(stmt))
		rows, err := conn.QueryContext(ctx, stmt)
		if err != nil {
			return err
		}
		if opts.JSON {
			if err := rowsToJSON(stream, rows); err != nil {
				return err
			}
		} else {
			if err := rowsToCSV(stream, rows); err != nil {
				return err
			}
		}
	}
	return nil
}

func rowsToJSON(w io.Writer, rows *sql.Rows) error {
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return err
	}

	jsonData := []map[string]any{}
	for rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}

		values := make([]interface{}, len(columns))
		for i := 0; i < len(columns); i++ {
			// values[i] = sql.NullString
		}
		if err := rows.Scan(values...); err != nil {
			return err
		}

		rowJsonData := map[string]any{}
		for i, v := range values {
			rowJsonData[columns[i]] = v
		}
		jsonData = append(jsonData, rowJsonData)
	}

	data, err := json.Marshal(jsonData)
	if err != nil {
		return err
	}

	w.Write(data)
	fmt.Fprintf(w, "\n")
	return nil
}

func rowsToCSV(w io.Writer, rows *sql.Rows) error {
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return err
	}

	for rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}

		values := make([]interface{}, len(columns))
		if err := rows.Scan(values...); err != nil {
			return err
		}

		for i, v := range values {
			if i > 0 {
				fmt.Fprintf(w, ",")
			}
			fmt.Fprintf(w, "%v", v)
		}
		fmt.Fprintf(w, "\n")
	}
	return nil
}
