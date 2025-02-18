/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package nogh

import (
	"bytes"
	"sync"

	idemix2 "github.com/hyperledger-labs/fabric-smart-client/platform/fabric/core/generic/msp/idemix"
	"github.com/hyperledger-labs/fabric-smart-client/platform/view/view"
	"github.com/hyperledger-labs/fabric-token-sdk/token/core/identity"
	"github.com/hyperledger-labs/fabric-token-sdk/token/core/identity/msp/idemix"
	"github.com/hyperledger-labs/fabric-token-sdk/token/core/identity/msp/x509"
	"github.com/hyperledger-labs/fabric-token-sdk/token/core/zkatdlog/crypto"
	"github.com/hyperledger-labs/fabric-token-sdk/token/driver"
	"github.com/pkg/errors"
)

// VerifierDES deserializes verifiers
// A verifier checks the validity of a signature against the identity
// associated with the verifier
type VerifierDES interface {
	DeserializeVerifier(id view.Identity) (driver.Verifier, error)
}

// AuditDES deserializes raw bytes into a matcher, which allows an auditor to match an identity to an enrollment ID
type AuditDES interface {
	DeserializeAuditInfo(raw []byte) (driver.Matcher, error)
}

// deserializer deserializes verifiers associated with issuers, owners, and auditors
type deserializer struct {
	auditorDeserializer VerifierDES
	ownerDeserializer   VerifierDES
	issuerDeserializer  VerifierDES
	auditDeserializer   AuditDES
}

// NewDeserializer returns a deserializer
func NewDeserializer(pp *crypto.PublicParams) (*deserializer, error) {
	if pp == nil {
		return nil, errors.New("failed to get deserializer: nil public parameters")
	}
	idemixDes, err := idemix.NewDeserializer(pp.IdemixPK, pp.IdemixCurve)
	if err != nil {
		return nil, errors.Wrapf(err, "failed getting idemix deserializer for passed public params")
	}

	return &deserializer{
		auditorDeserializer: &x509.MSPIdentityDeserializer{},
		issuerDeserializer:  &x509.MSPIdentityDeserializer{},
		ownerDeserializer:   identity.NewRawOwnerIdentityDeserializer(idemixDes),
		auditDeserializer:   idemixDes,
	}, nil
}

// GetOwnerVerifier deserializes the verifier for the passed owner identity
func (d *deserializer) GetOwnerVerifier(id view.Identity) (driver.Verifier, error) {
	return d.ownerDeserializer.DeserializeVerifier(id)
}

// GetIssuerVerifier deserializes the verifier for the passed issuer identity
func (d *deserializer) GetIssuerVerifier(id view.Identity) (driver.Verifier, error) {
	return d.issuerDeserializer.DeserializeVerifier(id)
}

// GetAuditorVerifier deserializes the verifier for the passed auditor identity
func (d *deserializer) GetAuditorVerifier(id view.Identity) (driver.Verifier, error) {
	return d.auditorDeserializer.DeserializeVerifier(id)
}

// GetOwnerMatcher returns a matcher that allows auditors to match an identity to an enrollment ID
func (d *deserializer) GetOwnerMatcher(raw []byte) (driver.Matcher, error) {
	return d.auditDeserializer.DeserializeAuditInfo(raw)
}

// DeserializerProvider provides the deserializer matching zkatdlog public parameters
type DeserializerProvider struct {
	oldHash []byte
	des     *deserializer
	mux     sync.Mutex
}

// NewDeserializerProvider returns a DeserializerProvider
func NewDeserializerProvider() *DeserializerProvider {
	return &DeserializerProvider{}
}

// Deserialize returns the deserializer matching the passed public parameters
func (d *DeserializerProvider) Deserialize(params *crypto.PublicParams) (driver.Deserializer, error) {
	d.mux.Lock()
	defer d.mux.Unlock()

	logger.Infof("Deserialize: [%s][%s]", params.Hash, d.oldHash)
	if bytes.Equal(d.oldHash, params.Hash) {
		return d.des, nil
	}

	des, err := NewDeserializer(params)
	if err != nil {
		return nil, err
	}
	d.des = des
	d.oldHash = params.Hash
	return des, nil
}

// enrollmentService returns enrollment IDs behind the owners of token
type enrollmentService struct {
}

// NewEnrollmentIDDeserializer returns an enrollmentService
func NewEnrollmentIDDeserializer() *enrollmentService {
	return &enrollmentService{}
}

// GetEnrollmentID returns the enrollmentID associated with the identity matched to the passed auditInfo
func (e *enrollmentService) GetEnrollmentID(auditInfo []byte) (string, error) {
	ai := &idemix2.AuditInfo{}
	if err := ai.FromBytes(auditInfo); err != nil {
		return "", errors.Wrapf(err, "failed unamrshalling audit info [%s]", auditInfo)
	}
	return ai.EnrollmentID(), nil
}
