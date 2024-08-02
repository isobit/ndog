package netutil

import (
	"context"
	"net"
	"syscall"
)

type Config struct {
	ReusePort bool `cli:"name=reuseport"`
}

func (cfg Config) ListenConfig() net.ListenConfig {
	return net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			if cfg.ReusePort {
				if err := ReusePortControl(network, address, c); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func (cfg Config) Listen(network, address string) (net.Listener, error) {
	listenCfg := cfg.ListenConfig()
	return listenCfg.Listen(context.Background(), network, address)
}

func (cfg Config) ListenPacket(network, address string) (net.PacketConn, error) {
	listenCfg := cfg.ListenConfig()
	return listenCfg.ListenPacket(context.Background(), network, address)
}
