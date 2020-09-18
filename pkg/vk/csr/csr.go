/*
 * Copyright 2020 The Multicluster-Scheduler Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package csr

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"net"
	"os"
	"time"

	"github.com/pkg/errors"
	"k8s.io/api/certificates/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	"admiralty.io/multicluster-scheduler/third_party/github.com/jetstack/cert-manager/pkg/util/pki"
)

func GetCertificateFromKubernetesAPIServer(ctx context.Context, k kubernetes.Interface) (certPEM, keyPEM []byte, err error) {
	reader := rand.Reader
	bitSize := 2048
	key, err := rsa.GenerateKey(reader, bitSize)
	utilruntime.Must(err)

	var ku x509.KeyUsage
	ku |= x509.KeyUsageKeyEncipherment
	ku |= x509.KeyUsageDigitalSignature

	usage, err := pki.BuildASN1KeyUsageRequest(ku)
	utilruntime.Must(errors.Wrap(err, "failed to asn1 encode usages"))

	ekus := []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	var asn1ExtendedUsages []asn1.ObjectIdentifier
	for _, eku := range ekus {
		if oid, ok := pki.OIDFromExtKeyUsage(eku); ok {
			asn1ExtendedUsages = append(asn1ExtendedUsages, oid)
		}
	}
	extendedUsage := pkix.Extension{
		Id: pki.OIDExtensionExtendedKeyUsage,
	}
	extendedUsage.Value, err = asn1.Marshal(asn1ExtendedUsages)
	utilruntime.Must(errors.Wrap(err, "failed to asn1 encode extended usages"))

	csrTemplate := &x509.CertificateRequest{
		Version:            3,
		SignatureAlgorithm: x509.SHA256WithRSA,
		PublicKeyAlgorithm: x509.RSA,
		Subject: pkix.Name{
			CommonName:   "system:node:admiralty",
			Organization: []string{"system:nodes"},
		},
		ExtraExtensions: []pkix.Extension{usage, extendedUsage},
		IPAddresses:     []net.IP{net.ParseIP(os.Getenv("VKUBELET_POD_IP"))},
	}

	csrDER, err := x509.CreateCertificateRequest(reader, csrTemplate, key)
	utilruntime.Must(err)

	csrPEM := pem.EncodeToMemory(&pem.Block{Bytes: csrDER, Type: "CERTIFICATE REQUEST"})

	csrK8s := &v1beta1.CertificateSigningRequest{}
	csrK8s.GenerateName = "admiralty-"
	csrK8s.Spec.Usages = []v1beta1.KeyUsage{v1beta1.UsageKeyEncipherment, v1beta1.UsageDigitalSignature, v1beta1.UsageServerAuth}
	signerName := v1beta1.KubeletServingSignerName
	csrK8s.Spec.SignerName = &signerName
	csrK8s.Spec.Request = csrPEM
	csrK8s, err = k.CertificatesV1beta1().CertificateSigningRequests().Create(ctx, csrK8s, metav1.CreateOptions{})
	if err != nil {
		return nil, nil, err
	}

	csrK8s.Status.Conditions = append(csrK8s.Status.Conditions, v1beta1.CertificateSigningRequestCondition{
		Type:    v1beta1.CertificateApproved,
		Reason:  "self-approved",
		Message: "self-approved",
	})
	csrK8s, err = k.CertificatesV1beta1().CertificateSigningRequests().UpdateApproval(ctx, csrK8s, metav1.UpdateOptions{})
	if err != nil {
		return nil, nil, err
	}

	csrName := csrK8s.Name
	err = wait.PollImmediateUntil(time.Second, func() (done bool, err error) {
		csrK8s, err = k.CertificatesV1beta1().CertificateSigningRequests().Get(ctx, csrName, metav1.GetOptions{})
		if err != nil {
			// TODO log if retriable; fail otherwise
			return false, nil
		}
		if len(csrK8s.Status.Certificate) > 0 {
			certPEM = csrK8s.Status.Certificate
			return true, nil
		}
		return false, nil
	}, ctx.Done())
	if err != nil {
		return nil, nil, err
	}

	keyPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	return certPEM, keyPEM, nil
}
