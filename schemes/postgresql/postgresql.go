package postgresql

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"

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
	if i := bytes.Index(data, []byte{';', '\n'}); i >= 0 {
		return i + 2, data[:i+2], nil
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

	name := conn.Config().ConnString()
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
		for rows.Next() {
			if err := rows.Err(); err != nil {
				return err
			}

			fields := []string{}
			for _, fd := range rows.FieldDescriptions() {
				fields = append(fields, fd.Name)
			}
			ndog.Logf(1, "row fields: %s", strings.Join(fields, ","))

			values, err := rows.Values()
			if err != nil {
				return err
			}
			for i, v := range values {
				if i > 0 {
					fmt.Fprintf(stream, ",")
				}
				fmt.Fprintf(stream, "%v", v)
			}
			fmt.Fprintf(stream, "\n")
		}
		rows.Close()
	}
	return nil
}
