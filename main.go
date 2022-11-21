package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/isobit/cli"
)

var in io.Reader = os.Stdin
var log io.Writer = os.Stderr
var out io.Writer = os.Stdout

var verbose bool = false
var logColor bool = false

func logf(format string, v ...interface{}) (int, error) {
	if logColor {
		format = "\u001b[30;1m" + format + "\u001b[0m"
	}
	if len(format) > 0 && format[len(format)-1] != '\n' {
		format = format + "\n"
	}
	return fmt.Fprintf(log, format, v...)
}

func main() {
	if stderrStat, err := os.Stderr.Stat(); err == nil {
		if stderrStat.Mode()&os.ModeCharDevice != 0 {
			logColor = true
		}
	}

	cli.New("ndog", &Ndog{}).
		Parse().
		RunFatal()
}

type Ndog struct {
	Verbose    bool     `cli:"short=v"`
	ListenURL  *url.URL `cli:"name=listen,short=l,placeholder=URL"`
	ConnectURL *url.URL `cli:"name=connect,short=c,placeholder=URL"`
}

func (cmd Ndog) Run() error {
	verbose = cmd.Verbose
	if cmd.ListenURL != nil && cmd.ConnectURL != nil {
		if cmd.ListenURL.Scheme == "tcp" && cmd.ConnectURL.Scheme == "tcp" {
			return proxyTCP(cmd.ListenURL.Host, cmd.ConnectURL.Host)
		}
		return fmt.Errorf("proxy is not supported yet")
	} else if cmd.ListenURL != nil {
		switch cmd.ListenURL.Scheme {
		case "tcp":
			return listenTCP(cmd.ListenURL.Host)
		case "udp":
			return listenUDP(cmd.ListenURL.Host)
		case "http":
			return listenHTTP(cmd.ListenURL.Host)
		default:
			return fmt.Errorf("unknown listen scheme: %s", cmd.ListenURL.Scheme)
		}
	} else if cmd.ConnectURL != nil {
		switch cmd.ConnectURL.Scheme {
		case "tcp":
			return dialTCP(cmd.ConnectURL.Host)
		case "udp":
			return dialUDP(cmd.ConnectURL.Host)
		default:
			return fmt.Errorf("unknown connect scheme: %s", cmd.ConnectURL.Scheme)
		}
	}
	return nil
}

func dialTCP(addrStr string) error {
	addr, err := net.ResolveTCPAddr("tcp", addrStr)
	if err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}

	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	logf("connected: %s", addr)

	go func() {
		io.Copy(conn, in)
	}()

	_, err = io.Copy(out, conn)
	return err
}

func dialUDP(addrStr string) error {
	addr, err := net.ResolveUDPAddr("udp", addrStr)
	if err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	logf("connected: %s", addr)

	go func() {
		io.Copy(conn, in)
	}()

	_, err = io.Copy(out, conn)
	return err
}

func listenTCP(addrStr string) error {
	addr, err := net.ResolveTCPAddr("tcp", addrStr)
	if err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}

	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return err
	}
	defer listener.Close()
	logf("listening: %s", listener.Addr())

	in := NewFanoutLineReader(in)
	go in.ScanLoop()

	handleConn := func(conn *net.TCPConn) {
		defer conn.Close()

		in, cancel := in.Tee()
		defer cancel()
		go io.Copy(conn, in)

		// io.Copy(out, conn)
		remoteAddr := conn.RemoteAddr()
		buf := make([]byte, 1024)
		for {
			nr, err := conn.Read(buf)
			if err != nil {
				return
			}
			if verbose {
				logf("read: %d bytes from %s", nr, remoteAddr)
			}
			_, err = out.Write(buf[:nr])
			if err != nil {
				return
			}
		}
	}

	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			if conn != nil {
				logf("accept error: %s: %s", conn.RemoteAddr(), err)
			} else {
				logf("accept error: %s", err)
			}
			continue
		}
		logf("accepted: %s", conn.RemoteAddr())
		go func() {
			handleConn(conn)
			logf("closed: %s", conn.RemoteAddr())
		}()
	}
}

func listenUDP(addrStr string) error {
	addr, err := net.ResolveUDPAddr("udp", addrStr)
	if err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	logf("listening: %s", conn.LocalAddr())

	// _, err = io.Copy(out, conn)
	// return err

	buf := make([]byte, 1024)
	for {
		nr, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			if err == io.EOF || err == net.ErrClosed {
				return nil
			}
			return err
		}
		if verbose {
			logf("read: %d bytes from %s", nr, remoteAddr)
		}

		_, err = out.Write(buf[:nr])
		if err != nil {
			return err
		}
	}
}

func listenHTTP(addrStr string) error {
	s := &http.Server{
		Addr: addrStr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logf("request: %s: %s %s", r.RemoteAddr, r.Method, r.URL)
			if verbose {
				for key, values := range r.Header {
					logf("request header: %s: %s", key, strings.Join(values, ", "))
				}
			}
			io.Copy(out, r.Body)
			fmt.Fprintln(out)
		}),
	}
	logf("listening: %s", addrStr)
	return s.ListenAndServe()
}

func proxyTCP(listenAddr string, connectAddr string) error {
	listenTCPAddr, err := net.ResolveTCPAddr("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}

	listener, err := net.ListenTCP("tcp", listenTCPAddr)
	if err != nil {
		return err
	}
	defer listener.Close()
	logf("listening: %s", listener.Addr())

	handleConn := func(srcConn *net.TCPConn) error {
		connectTCPAddr, err := net.ResolveTCPAddr("tcp", connectAddr)
		if err != nil {
			return fmt.Errorf("invalid address: %w", err)
		}
		destConn, err := net.DialTCP("tcp", nil, connectTCPAddr)
		if err != nil {
			return err
		}
		defer destConn.Close()
		logf("connected: %s", connectAddr)

		go func() {
			// io.Copy(srcConn, io.TeeReader(destConn, PrefixWriter(out, []byte("< "))))
			lw, lwClose := TeeLinesReader(destConn, func(p []byte) {
				fmt.Fprintf(out, "< ")
				out.Write(p)
				fmt.Fprintln(out)
			})
			defer lwClose()
			io.Copy(srcConn, lw)
		}()

		// _, err = io.Copy(destConn, io.TeeReader(srcConn, PrefixWriter(out, []byte("> "))))
		lw, lwClose := TeeLinesReader(srcConn, func(p []byte) {
			fmt.Fprintf(out, "> ")
			out.Write(p)
			fmt.Fprintln(out)
		})
		defer lwClose()
		_, err = io.Copy(destConn, lw)
		if err != nil {
			return err
		}
		return nil
	}

	for {
		srcConn, err := listener.AcceptTCP()
		if err != nil {
			if srcConn != nil {
				logf("accept error: %s: %s", srcConn.RemoteAddr(), err)
			} else {
				logf("accept error: %s", err)
			}
			continue
		}
		logf("accepted: %s", srcConn.RemoteAddr())
		go func() {
			handleConn(srcConn)
			logf("closed: %s", srcConn.RemoteAddr())
		}()
	}
}
