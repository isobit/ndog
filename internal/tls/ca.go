package tls

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
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

	"github.com/isobit/ndog/internal/log"
)

type CA struct {
	cert *x509.Certificate
	key  crypto.Signer
}

func LoadCAFromFiles(certFilename string, keyFilename string) (*CA, error) {
	certBytes, err := os.ReadFile(certFilename)
	if err != nil {
		return nil, fmt.Errorf("error reading cert %s: %w", certFilename, err)
	}

	keyBytes, err := os.ReadFile(keyFilename)
	if err != nil {
		return nil, fmt.Errorf("error reading key %s: %w", keyFilename, err)
	}

	certBlock, _ := pem.Decode(certBytes)
	keyBlock, _ := pem.Decode(keyBytes)

	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("error parsing cert %s: %w", certFilename, err)
	}

	key, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if err != nil {
		fmt.Println(keyBlock.Type)
		return nil, fmt.Errorf("error parsing key %s: %w", keyFilename, err)
	}

	return &CA{
		cert: cert,
		key:  key.(crypto.Signer),
	}, nil
}

func GenerateCA() (*CA, error) {
	serialNumber, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	cert := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization:  []string{""},
			Country:       []string{""},
			Province:      []string{""},
			Locality:      []string{""},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},

		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(10, 0, 0),

		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},

		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	return &CA{
		cert: cert,
		key:  key,
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

func hostsToIPAndDNS(hosts []string) ([]net.IP, []string) {
	ips := []net.IP{}
	dnsNames := []string{}
	for _, host := range hosts {
		host := host
		ip := net.ParseIP(host)
		if ip != nil {
			ips = append(ips, ip)
		} else {
			dnsNames = append(dnsNames, host)

			for _, network := range []string{"ip4", "ip6"} {
				addr, err := net.ResolveIPAddr(network, host)
				if err == nil {
					ips = append(ips, addr.IP)
				} else {
					log.Logf(10, "failed to resolve %s: %s", host, err)
				}
			}
		}
	}
	return ips, dnsNames
}

func (ca *CA) GenerateAndSignTLSCert(hosts []string) (tls.Certificate, error) {
	ips, dnsNames := hostsToIPAndDNS(hosts)

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

		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(10, 0, 0),

		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},

		// SubjectKeyId: []byte{1, 2, 3, 4, 6},

		IPAddresses: ips,
		DNSNames:    dnsNames,

		// DNSNames: []string{
		// 	"localhost",
		// },
		// IPAddresses: []net.IP{
		// 	net.IPv4(127, 0, 0, 1),
		// 	net.IPv6loopback,
		// },
	}

	log.Logf(10, "TLS cert template: %+v", cert)

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
