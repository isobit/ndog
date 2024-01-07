package tls

import (
	"crypto/x509"
	"fmt"
	"os"
)

func CertPoolFromCACert(filename string) (*x509.CertPool, error) {
	certPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("error loading system cert pool: %w", err)
	}

	cert, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("error reading CA cert %s: %w", filename, err)
	}
	if ok := certPool.AppendCertsFromPEM(cert); !ok {
		return nil,fmt.Errorf("unable to parse CA cert %s", filename)
	}

	return certPool, nil
}
