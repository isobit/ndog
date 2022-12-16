package postgresql

import (
	"bufio"
	"bytes"
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

func Connect(cfg ndog.Config) error {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, cfg.URL.String())
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	cc := conn.Config()
	name := fmt.Sprintf("%s:%d", cc.Host, cc.Port)
	ndog.Logf(0, "connected: %s", name)

	stream := cfg.NewStream(name)
	defer stream.Close()

	scanner := bufio.NewScanner(stream)
	scanner.Split(splitStatements)
	for scanner.Scan() {
		stmt := scanner.Text()
		ndog.Logf(1, "execute: %s", strconv.Quote(stmt))
		rows, err := conn.Query(ctx, stmt)
		if err != nil {
			return err
		}
		if cfg.JSON {
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
