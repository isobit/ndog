package tls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/isobit/ndog/internal/log"
)

type Options struct {
	TLSSkipVerify bool `cli:"name=tls-skip-verify,env=NDOG_TLS_SKIP_VERIFY"`

	TLSCert string `cli:"name=tls-cert,env=NDOG_TLS_CERT"`
	TLSKey  string `cli:"name=tls-key,env=NDOG_TLS_KEY"`

	TLSCACert     string   `cli:"name=tls-ca-cert,env=NDOG_TLS_CA_CERT"`
	TLSCAKey      string   `cli:"name=tls-ca-key,env=NDOG_TLS_CA_KEY"`
	TLSExtraHosts []string `cli:"name=tls-extra-hosts,env=NDOG_TLS_CA_EXTRA_HOSTS,append"`
}

func (opts Options) Config(server bool, hosts []string) (*tls.Config, error) {
	c := &tls.Config{
		InsecureSkipVerify: opts.TLSSkipVerify,
	}

	if opts.TLSCACert != "" {
		log.Logf(1, "loading TLS root CA cert in %s", opts.TLSCACert)

		certPool, err := x509.SystemCertPool()
		if err != nil {
			return nil, fmt.Errorf("error loading system cert pool: %w", err)
		}

		certBytes, err := os.ReadFile(opts.TLSCACert)
		if err != nil {
			return nil, fmt.Errorf("error reading cert %s: %w", opts.TLSCACert, err)
		}
		certPool.AppendCertsFromPEM(certBytes)

		c.RootCAs = certPool
	}

	if opts.TLSCert != "" && opts.TLSKey != "" {
		log.Logf(1, "loading TLS cert in %s and key in %s", opts.TLSCACert, opts.TLSCAKey)
		cert, err := tls.LoadX509KeyPair(opts.TLSCert, opts.TLSKey)
		if err != nil {
			return nil, fmt.Errorf("error loading cert: %w", err)
		}

		c.Certificates = []tls.Certificate{cert}
	}

	if server && c.Certificates == nil {
		certHosts := make([]string, len(hosts))
		copy(certHosts, hosts)
		if opts.TLSExtraHosts != nil {
			certHosts = append(certHosts, opts.TLSExtraHosts...)
		}

		ca, err := opts.getCA()
		if err != nil {
			return nil, fmt.Errorf("error loading CA to generate cert: %w", err)
		}

		cert, err := ca.GenerateAndSignTLSCert(certHosts)
		if err != nil {
			return nil, fmt.Errorf("error generating cert: %w", err)
		}

		c.Certificates = []tls.Certificate{cert}
	}

	return c, nil
}

func (opts Options) getCA() (*CA, error) {
	if opts.TLSCACert != "" && opts.TLSCAKey != "" {
		log.Logf(1, "loading TLS CA from cert in %s and key in %s", opts.TLSCACert, opts.TLSCAKey)
		return LoadCAFromFiles(opts.TLSCACert, opts.TLSCAKey)
	}
	log.Logf(1, "generating self-signed TLS CA cert")
	return GenerateCA()
}
