package postgresql

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/isobit/ndog/internal"
	"github.com/isobit/ndog/internal/ioutil"
	"github.com/isobit/ndog/internal/log"
)

var ListenScheme = &ndog.Scheme{
	Names:   []string{"postgres+listen"},
	Connect: listenConnect,

	Description: `
Connect runs LISTEN on each channel(s) in the URL fragment (comma-separated) on
the specified PostgreSQL server and outputs any received notifications.

Example: ndog -c 'postgres+listen://localhost#foo,bar'
	`,
	ConnectOptionHelp: ndog.OptionsHelp{}.
		Add("json", "", "use JSON representation for returned rows"),
}

func listenConnect(cfg ndog.ConnectConfig) error {
	opts, err := extractOptions(cfg.Options)
	if err != nil {
		return err
	}

	if cfg.URL.Fragment == "" {
		return fmt.Errorf("URL must include one or more comma-separated channel names as a fragment")
	}
	channelNames := strings.Split(cfg.URL.Fragment, ",")

	connUrl, _ := ndog.SplitURLSubscheme(cfg.URL)
	connUrl.Fragment = ""

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, connUrl.String())
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	cc := conn.Config()
	name := fmt.Sprintf("%s:%d", cc.Host, cc.Port)
	log.Logf(0, "connected: %s", name)

	stream := cfg.Stream

	for _, channelName := range channelNames {
		log.Logf(1, "exec: LISTEN %s", channelName)
		if _, err := conn.Exec(ctx, fmt.Sprintf("LISTEN %s", channelName)); err != nil {
			return err
		}
	}

	for {
		notification, err := conn.WaitForNotification(ctx)
		if err != nil {
			return err
		}
		log.Logf(1, "notification: %s", notification.Channel)
		if opts.JSON {
			if err := ioutil.WriteJSON(stream.Writer, notification); err != nil {
				return err
			}
		} else {
			io.WriteString(stream.Writer, notification.Payload)
			io.WriteString(stream.Writer, "\n")
		}
	}
}

var NotifyScheme = &ndog.Scheme{
	Names:   []string{"postgres+notify"},
	Connect: notifyConnect,

	Description: `
Connect runs NOTIFY on the channel in the URL fragment for each input line,
using the input as the payload.

Example: ndog -c 'postgres+notify://localhost#foo' -d hello
	`,
	ConnectOptionHelp: ndog.OptionsHelp{}.
		Add("json", "", "use JSON representation for returned rows"),
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

	connUrl, _ := ndog.SplitURLSubscheme(cfg.URL)
	connUrl.Fragment = ""

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, connUrl.String())
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	cc := conn.Config()
	name := fmt.Sprintf("%s:%d", cc.Host, cc.Port)
	log.Logf(0, "connected: %s", name)

	scanner := bufio.NewScanner(cfg.Stream.Reader)
	for scanner.Scan() {
		payload := scanner.Text()
		log.Logf(1, "notify: %s", channelName)
		if _, err := conn.Exec(ctx, "SELECT pg_notify($1, $2)", channelName, payload); err != nil {
			return err
		}
	}
	return nil
}
