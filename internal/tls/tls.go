package tls

import (
	"bytes"
	"crypto"
	"crypto/rand"
	// "crypto/rsa"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math"
	"math/big"
	"net"
	"os"
	"time"

	"github.com/isobit/ndog/internal"
)

type listenOptions struct {
	TLSCert string
	TLSKey  string
}

func extractListenOptions(opts ndog.Options) (listenOptions, error) {
	o := listenOptions{}

	if val, ok := opts.Pop("cert"); ok {
		o.TLSCert = val
	}
	if val, ok := opts.Pop("key"); ok {
		o.TLSKey = val
	}

	return o, nil
}

type CA struct {
	cert *x509.Certificate
	// key *rsa.PrivateKey
	key crypto.Signer
}

// func LoadCAFromFiles(certFilename string, keyFilename string) (*CA, error) {
// 	certBytes, err := os.ReadFile(certFilename)
// 	if err != nil {
// 		return nil, fmt.Errorf("error reading cert %s: %w", certFilename, err)
// 	}

// 	keyBytes, err := os.ReadFile(certFilename)
// 	if err != nil {
// 		return nil, fmt.Errorf("error reading key %s: %w", keyFilename, err)
// 	}


// }

func GenerateCA() (*CA, error) {
	serialNumber, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	cert := &x509.Certificate{
		// SerialNumber: big.NewInt(time.Now().Unix()),
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization:  []string{""},
			Country:       []string{""},
			Province:      []string{""},
			Locality:      []string{""},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},

		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),

		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},

		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	// key, err := rsa.GenerateKey(rand.Reader, 2048)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	return &CA{
		cert: cert,
		key: key,
	}, nil
}

func (ca *CA) certBytes() ([]byte, error) {
	return x509.CreateCertificate(
		rand.Reader,
		ca.cert,
		ca.cert,
		ca.key.Public(),
		ca.key,
	)
}

func (ca *CA) GenerateAndSignTLSCert() (tls.Certificate, error) {
	serialNumber, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	cert := &x509.Certificate{
		SerialNumber: serialNumber,
		// Subject: pkix.Name{
		// 	Organization:  []string{""},
		// 	Country:       []string{""},
		// 	Province:      []string{""},
		// 	Locality:      []string{""},
		// 	StreetAddress: []string{""},
		// 	PostalCode:    []string{""},
		// },

		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),

		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},

		// SubjectKeyId: []byte{1, 2, 3, 4, 6},

		IPAddresses: []net.IP{
			net.IPv4(127, 0, 0, 1),
			net.IPv6loopback,
		},
	}
	// key, err := rsa.GenerateKey(rand.Reader, 2048)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		return tls.Certificate{}, err
	}

	certPEM := new(bytes.Buffer)
	pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	certPrivKeyPEM := new(bytes.Buffer)
	// pem.Encode(certPrivKeyPEM, &pem.Block{
	// 	Type:  "RSA PRIVATE KEY",
	// 	Bytes: x509.MarshalPKCS1PrivateKey(key),
	// })
	keyDERBytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return tls.Certificate{}, err
	}
	pem.Encode(certPrivKeyPEM, &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: keyDERBytes,
	})

	return tls.X509KeyPair(certPEM.Bytes(), certPrivKeyPEM.Bytes())
}

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
