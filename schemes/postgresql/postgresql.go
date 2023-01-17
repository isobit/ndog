package postgresql

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/jackc/pgx/v5"

	"github.com/isobit/ndog"
)

var Scheme = &ndog.Scheme{
	Names:   []string{"postgresql", "postgres"},
	Connect: Connect,
}

type Options struct {
	JSON bool
}

func extractOptions(opts ndog.Options) (Options, error) {
	o := Options{}
	if _, ok := opts.Pop("json"); ok {
		o.JSON = true
	}
	return o, opts.Done()
}

func Connect(cfg ndog.ConnectConfig) error {
	opts, err := extractOptions(cfg.Options)
	if err != nil {
		return err
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, cfg.URL.String())
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	cc := conn.Config()
	name := fmt.Sprintf("%s:%d", cc.Host, cc.Port)
	ndog.Logf(0, "connected: %s", name)

	stream := cfg.Stream

	scanner := bufio.NewScanner(stream.Reader)
	scanner.Split(splitStatements)
	for scanner.Scan() {
		stmt := scanner.Text()
		ndog.Logf(1, "execute: %s", strconv.Quote(stmt))
		rows, err := conn.Query(ctx, stmt)
		if err != nil {
			ndog.Logf(-1, "error executing query: %s", err)
			continue
		}
		if opts.JSON {
			if err := rowsToJSON(stream.Writer, rows); err != nil {
				ndog.Logf(-1, "error converting rows to JSON: %s", err)
			}
		} else {
			if err := rowsToCSV(stream.Writer, rows); err != nil {
				ndog.Logf(-1, "error converting rows to CSV: %s", err)
			}
		}
	}
	return nil
}

func rowsToJSON(w io.Writer, rows pgx.Rows) error {
	defer rows.Close()
	jsonData := []map[string]any{}
	for rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}

		fields := []string{}
		for _, fd := range rows.FieldDescriptions() {
			fields = append(fields, fd.Name)
		}

		values, err := rows.Values()
		if err != nil {
			return err
		}

		rowJsonData := map[string]any{}
		for i, v := range values {
			rowJsonData[fields[i]] = v
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

func rowsToCSV(w io.Writer, rows pgx.Rows) error {
	defer rows.Close()
	for rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}

		values, err := rows.Values()
		if err != nil {
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

func splitStatements(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF {
		if len(data) == 0 {
			return 0, nil, nil
		}
		return len(data), data, nil
	}
	inEscape := false
	inString := false
	for i, b := range data {
		if inEscape {
			inEscape = false
			continue
		}
		switch {
		case b == '\\':
			inEscape = true
		case b == '\'':
			inString = !inString
		case b == ';' && !inString:
			return i + 1, data[:i], nil
		}
	}
	// request more data
	return 0, nil, nil
}
