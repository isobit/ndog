package tls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/isobit/ndog/internal/log"
)

type Config struct {
	TLSSkipVerify bool   `cli:"name=tls-skip-verify,env=NDOG_TLS_SKIP_VERIFY"`
	TLSServerName string `cli:"name=tls-server-name,env=NDOG_TLS_SERVER_NAME"`

	TLSCert string `cli:"name=tls-cert,env=NDOG_TLS_CERT"`
	TLSKey  string `cli:"name=tls-key,env=NDOG_TLS_KEY"`

	TLSCACert     string   `cli:"name=tls-ca-cert,env=NDOG_TLS_CA_CERT"`
	TLSCAKey      string   `cli:"name=tls-ca-key,env=NDOG_TLS_CA_KEY"`
	TLSExtraHosts []string `cli:"name=tls-extra-hosts,env=NDOG_TLS_CA_EXTRA_HOSTS,append"`
}

func (cfg Config) Config(server bool, hosts []string) (*tls.Config, error) {
	c := &tls.Config{
		InsecureSkipVerify: cfg.TLSSkipVerify,
		ServerName:         cfg.TLSServerName,
	}

	if cfg.TLSCACert != "" {
		log.Logf(1, "loading TLS root CA cert in %s", cfg.TLSCACert)

		certPool, err := x509.SystemCertPool()
		if err != nil {
			return nil, fmt.Errorf("error loading system cert pool: %w", err)
		}

		certBytes, err := os.ReadFile(cfg.TLSCACert)
		if err != nil {
			return nil, fmt.Errorf("error reading cert %s: %w", cfg.TLSCACert, err)
		}
		if ok := certPool.AppendCertsFromPEM(certBytes); !ok {
			return nil, fmt.Errorf("no certs were appended from %s", cfg.TLSCACert)
		}

		c.RootCAs = certPool
	}

	if cfg.TLSCert != "" && cfg.TLSKey != "" {
		log.Logf(1, "loading TLS cert in %s and key in %s", cfg.TLSCACert, cfg.TLSCAKey)
		cert, err := tls.LoadX509KeyPair(cfg.TLSCert, cfg.TLSKey)
		if err != nil {
			return nil, fmt.Errorf("error loading cert: %w", err)
		}

		c.Certificates = []tls.Certificate{cert}
	}

	if server && c.Certificates == nil {
		certHosts := make([]string, len(hosts))
		copy(certHosts, hosts)
		if cfg.TLSExtraHosts != nil {
			certHosts = append(certHosts, cfg.TLSExtraHosts...)
		}

		ca, err := cfg.getCA()
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

func (cfg Config) getCA() (*CA, error) {
	if cfg.TLSCACert != "" && cfg.TLSCAKey != "" {
		log.Logf(1, "loading TLS CA from cert in %s and key in %s", cfg.TLSCACert, cfg.TLSCAKey)
		return LoadCAFromFiles(cfg.TLSCACert, cfg.TLSCAKey)
	}
	log.Logf(1, "generating self-signed TLS CA cert")
	return GenerateCA()
}
