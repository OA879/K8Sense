package checks

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/OA879/K8Sense/backend/pkg/clusterdoctor"
)

const certExpiryWarnWindow = 30 * 24 * time.Hour

func init() {
	// CERT-001 warns 30 days out; CERT-002 flags already-expired. Both read
	// only the public certificate (tls.crt) — never the private key (tls.key).
	clusterdoctor.RegisterCheck("check_tls_secret_expiring", checkTLSSecret(false))
	clusterdoctor.RegisterCheck("check_tls_secret_expired", checkTLSSecret(true))
}

// checkTLSSecret returns a CheckFunc over every kubernetes.io/tls Secret. When
// expiredOnly is true it flags certs already past NotAfter (CERT-002); when
// false it flags certs expiring within the warn window but not yet expired
// (CERT-001), so a given secret is reported by exactly one of the two rules.
func checkTLSSecret(expiredOnly bool) clusterdoctor.CheckFunc {
	return func(ctx context.Context, clientset kubernetes.Interface) ([]clusterdoctor.RawFinding, error) {
		secrets, err := clientset.CoreV1().Secrets(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		now := time.Now()

		var findings []clusterdoctor.RawFinding

		for _, secret := range secrets.Items {
			if secret.Type != corev1.SecretTypeTLS {
				continue
			}

			cert := parseLeafCert(secret.Data["tls.crt"])
			if cert == nil {
				continue
			}

			expired := now.After(cert.NotAfter)
			expiringSoon := !expired && cert.NotAfter.Sub(now) < certExpiryWarnWindow

			if (expiredOnly && expired) || (!expiredOnly && expiringSoon) {
				findings = append(findings, clusterdoctor.RawFinding{
					Namespace:    secret.Namespace,
					ResourceKind: "Secret",
					ResourceName: secret.Name,
					RawObject: fmt.Sprintf(
						`{"notAfter": %q, "commonName": %q}`,
						cert.NotAfter.Format(time.RFC3339), cert.Subject.CommonName,
					),
				})
			}
		}

		return findings, nil
	}
}

// parseLeafCert decodes the first PEM CERTIFICATE block from tls.crt (the leaf
// certificate) and returns it, or nil if the data is missing/unparseable.
// Only the public certificate is ever touched here.
func parseLeafCert(pemBytes []byte) *x509.Certificate {
	if len(pemBytes) == 0 {
		return nil
	}

	for {
		block, rest := pem.Decode(pemBytes)
		if block == nil {
			return nil
		}

		if block.Type == "CERTIFICATE" {
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil
			}

			return cert
		}

		pemBytes = rest
	}
}
