package postgresql

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5"

	"github.com/isobit/ndog"
)

var ListenScheme = &ndog.Scheme{
	Names:   []string{"postgresql+listen", "postgres+listen", "pg+listen"},
	Connect: listenConnect,
}

func listenConnect(cfg ndog.ConnectConfig) error {
	opts, err := extractOptions(cfg.Options)
	if err != nil {
		return err
	}

	channelName := cfg.URL.Fragment
	if channelName == "" {
		return fmt.Errorf("URL must include the channel name as a fragment")
	}

	connUrl := *cfg.URL
	connUrl.Scheme = "postgresql"
	connUrl.Fragment = ""

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, connUrl.String())
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	cc := conn.Config()
	name := fmt.Sprintf("%s:%d", cc.Host, cc.Port)
	ndog.Logf(0, "connected: %s", name)

	stream := cfg.Stream

	if _, err := conn.Exec(ctx, fmt.Sprintf("LISTEN %s", channelName)); err != nil {
		return err
	}

	for {
		notification, err := conn.WaitForNotification(ctx)
		if err != nil {
			return err
		}
		ndog.Logf(1, "notification: %s", notification.Payload)
		if opts.JSON {
			data, err := json.Marshal(notification)
			if err != nil {
				return err
			}
			stream.Writer.Write(data)
			io.WriteString(stream.Writer, "\n")
		} else {
			io.WriteString(stream.Writer, notification.Payload)
			io.WriteString(stream.Writer, "\n")
		}
	}
}

var NotifyScheme = &ndog.Scheme{
	Names:   []string{"postgresql+notify", "postgres+notify", "pg+notify"},
	Connect: notifyConnect,
}

func notifyConnect(cfg ndog.ConnectConfig) error {
	// JSON not supported yet
	if err := cfg.Options.Done(); err != nil {
		return err
	}

	channelName := cfg.URL.Fragment
	if channelName == "" {
		return fmt.Errorf("URL must include the channel name as a fragment")
	}

	connUrl := *cfg.URL
	connUrl.Scheme = "postgresql"
	connUrl.Fragment = ""

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, connUrl.String())
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	cc := conn.Config()
	name := fmt.Sprintf("%s:%d", cc.Host, cc.Port)
	ndog.Logf(0, "connected: %s", name)

	scanner := bufio.NewScanner(cfg.Stream.Reader)
	for scanner.Scan() {
		payload := scanner.Text()
		ndog.Logf(1, "notify: %s", payload)
		if _, err := conn.Exec(ctx, "SELECT pg_notify($1, $2)", channelName, payload); err != nil {
			return err
		}
	}
	return nil
}
