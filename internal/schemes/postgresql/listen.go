package postgresql

import (
	"bufio"
	"fmt"
	"io"
	"net"

	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/isobit/ndog/internal"
)

func Listen(cfg ndog.ListenConfig) error {
	if err := cfg.Options.Done(); err != nil {
		return err
	}

	addr, err := net.ResolveTCPAddr("tcp", cfg.URL.Host)
	if err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}

	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return err
	}
	defer listener.Close()
	ndog.Logf(0, "listening: %s", listener.Addr())

	handleConn := func(conn *net.TCPConn) {
		defer conn.Close()

		remoteAddr := conn.RemoteAddr()
		ndog.Logf(1, "accepted: %s", remoteAddr)
		defer ndog.Logf(1, "closed: %s", remoteAddr)

		if err := handleConn(cfg, conn); err != nil {
			ndog.Logf(-1, "%s", err)
		}
	}

	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			if conn != nil {
				ndog.Logf(-1, "accept error: %s: %s", conn.RemoteAddr(), err)
			} else {
				ndog.Logf(-1, "accept error: %s", err)
			}
			continue
		}
		go handleConn(conn)
	}
}

func handleConn(cfg ndog.ListenConfig, conn net.Conn) error {
	remoteAddr := conn.RemoteAddr()

	backend := pgproto3.NewBackend(conn, conn)

	ndog.Logf(3, "receiving startup message")
	if err := handleStartup(conn, backend); err != nil {
		return fmt.Errorf("error in startup: %w", err)
	}

	for {
		msg, err := backend.Receive()
		if err != nil {
			return fmt.Errorf("receive error: %w", err)
		}
		switch msg := msg.(type) {

		case *pgproto3.Query:
			ndog.Logf(3, "got query message: %s", msg.String)

			stream := cfg.StreamManager.NewStream(remoteAddr.String())
			defer stream.Close()

			if _, err := io.WriteString(stream.Writer, msg.String); err != nil {
				return fmt.Errorf("error writing query to stream: %w", err)
			}
			stream.Writer.Close()

			backend.Send(&pgproto3.RowDescription{Fields: []pgproto3.FieldDescription{
				{
					Name:                 []byte("ndog"),
					TableOID:             0,
					TableAttributeNumber: 0,
					DataTypeOID:          pgtype.TextOID,
					DataTypeSize:         -1,
					TypeModifier:         -1,
					Format:               0,
				},
			}})

			scanner := bufio.NewScanner(stream.Reader)
			for scanner.Scan() {
				backend.Send(&pgproto3.DataRow{Values: [][]byte{scanner.Bytes()}})
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("error reading query response: %w", scanner.Err())
			}
			backend.Send(&pgproto3.CommandComplete{CommandTag: []byte{}})
			backend.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
			if err := backend.Flush(); err != nil {
				return fmt.Errorf("error sending response to query: %w", err)
			}

		case *pgproto3.Terminate:
			ndog.Logf(3, "got terminate message: %+v", msg)
			return nil

		default:
			return fmt.Errorf("got unknown message: %v", msg)

		}
	}
}

func handleStartup(conn net.Conn, backend *pgproto3.Backend) error {
	msg, err := backend.ReceiveStartupMessage()
	if err != nil {
		return fmt.Errorf("error receiving startup message: %w", err)
	}
	switch msg := msg.(type) {
	case *pgproto3.StartupMessage:
		ndog.Logf(3, "got StartupMessage: %+v", msg)
		backend.Send(&pgproto3.AuthenticationOk{})
		backend.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
		if err := backend.Flush(); err != nil {
			return fmt.Errorf("error sending response to startup message: %w", err)
		}
		return nil
	case *pgproto3.SSLRequest:
		ndog.Logf(3, "got SSLRequest: %+v", msg)
		if _, err := conn.Write([]byte{'N'}); err != nil {
			return fmt.Errorf("error sending deny SSL request: %w", err)
		}
		return handleStartup(conn, backend)
	default:
		return fmt.Errorf("invalid startup msg")
	}
}
