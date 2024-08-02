//go:build !linux

package netutil

import (
	"fmt"
	"syscall"
)

func ReusePortControl(network, address string, c syscall.RawConn) error {
	return fmt.Errorf("reuseport is currently only supported on linux")
}
