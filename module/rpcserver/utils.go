/*
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/
package rpcserver

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"encoding/hex"

	"chainmaker.org/chainmaker-go/module/snapshot"
	cmx509 "chainmaker.org/chainmaker/common/v2/crypto/x509"
	"chainmaker.org/chainmaker/logger/v2"
	pbac "chainmaker.org/chainmaker/pb-go/v2/accesscontrol"
	"chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/protocol/v2"
	"chainmaker.org/chainmaker/utils/v2"
	"github.com/pkg/errors"
)

func createVerifyPeerCertificateFunc(
	accessControls []protocol.AccessControlProvider,
) func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		revoked, err := isRevoked(accessControls, rawCerts, verifiedChains)
		if err != nil {
			return err
		}

		if revoked {
			return fmt.Errorf("certificate revoked")
		}

		return nil
	}
}

func createGMVerifyPeerCertificateFunc(
	accessControls []protocol.AccessControlProvider,
) func(rawCerts [][]byte, verifiedChains [][]*cmx509.Certificate) error {
	return func(rawCerts [][]byte, verifiedChains [][]*cmx509.Certificate) error {
		revoked, err := isGMRevoked(accessControls, rawCerts, verifiedChains)
		if err != nil {
			return err
		}

		if revoked {
			return fmt.Errorf("certificate revoked")
		}

		return nil
	}
}

func createMixVerifyPeerCertificateFunc(
	accessControls []protocol.AccessControlProvider,
	log *logger.CMLogger,
) func(rawCerts [][]byte, verifiedChains [][]*cmx509.Certificate) error {
	return func(rawCerts [][]byte, verifiedChains [][]*cmx509.Certificate) error {
		revoked, err := isGMRevoked(accessControls, rawCerts, verifiedChains)
		if err != nil {
			log.Info(err)
			return err
		}

		if revoked {
			log.Info("certificate revoked")
			return fmt.Errorf("certificate revoked")
		}

		return nil
	}
}

func isRevoked(accessControls []protocol.AccessControlProvider, rawCerts [][]byte,
	verifiedChains [][]*x509.Certificate) (bool, error) {

	members := make([]*pbac.Member, 0)
	for idx := range rawCerts {
		m := &pbac.Member{
			OrgId:      "",
			MemberType: pbac.MemberType_CERT,
			MemberInfo: rawCerts[idx],
		}
		members = append(members, m)
	}

	for i := range verifiedChains {
		for j := range verifiedChains[i] {
			certBytes := pem.EncodeToMemory(&pem.Block{
				Type:    "CERTIFICATE",
				Headers: nil,
				Bytes:   verifiedChains[i][j].Raw,
			})

			m := &pbac.Member{
				OrgId:      "",
				MemberType: pbac.MemberType_CERT,
				MemberInfo: certBytes,
			}
			members = append(members, m)
		}
	}

	return checkMemberStatusIsRevoked(accessControls, members)
}

func isGMRevoked(accessControls []protocol.AccessControlProvider, rawCerts [][]byte,
	verifiedChains [][]*cmx509.Certificate) (bool, error) {

	members := make([]*pbac.Member, 0)
	for idx := range rawCerts {
		m := &pbac.Member{
			OrgId:      "",
			MemberType: pbac.MemberType_CERT,
			MemberInfo: rawCerts[idx],
		}
		members = append(members, m)
	}

	for i := range verifiedChains {
		for j := range verifiedChains[i] {
			certBytes := pem.EncodeToMemory(&pem.Block{
				Type:    "CERTIFICATE",
				Headers: nil,
				Bytes:   verifiedChains[i][j].Raw,
			})

			m := &pbac.Member{
				OrgId:      "",
				MemberType: pbac.MemberType_CERT,
				MemberInfo: certBytes,
			}
			members = append(members, m)
		}
	}

	return checkMemberStatusIsRevoked(accessControls, members)
}

// ValidateMemberStatus check the status of members.
func checkMemberStatusIsRevoked(accessControls []protocol.AccessControlProvider,
	members []*pbac.Member) (bool, error) {

	var err error

	for _, ac := range accessControls {
		if ac == nil {
			return false, fmt.Errorf("ac is nil")
		}

		for _, member := range members {
			var s pbac.MemberStatus
			s, err = ac.GetMemberStatus(member)
			if err != nil {
				return false, err
			}

			if s == pbac.MemberStatus_INVALID || s == pbac.MemberStatus_FROZEN || s == pbac.MemberStatus_REVOKED {
				return true, errors.New("cert status is " + s.String())
			}

		}
	}

	return false, nil
}

// checkTxSignCert check if sign cert is valid
func checkTxSignCert(tx *common.Transaction) error {
	if tx.Sender.Signer.MemberType != pbac.MemberType_CERT {
		return nil
	}
	b, rest := pem.Decode(tx.Sender.Signer.MemberInfo)
	if len(rest) != 0 {
		return errors.New("failed to decode sign cert, rest not nil")
	}
	cert, err := cmx509.ParseCertificate(b.Bytes)
	if err != nil {
		return errors.WithMessage(err, "failed to parse sign cert")
	}
	//signCert's keyUsage: 	Digital Signature, Non Repudiation
	if cert.KeyUsage == 0 || (cert.KeyUsage&(x509.KeyUsageDigitalSignature) == 0) {
		return errors.New("tx sign certificate is not valid for Digital Signature")
	}
	//tlsCert's keyUsage:   Digital Signature, Non Repudiation, Key Encipherment, Data Encipherment, Key Agreement
	if cert.KeyUsage != 0 && (cert.KeyUsage&(x509.KeyUsageKeyAgreement) != 0) {
		return errors.New("tls certificate is misused for tx sign")
	}
	return nil
}

func publicKeyPEMFromMember(member *pbac.Member, store protocol.BlockchainStore) ([]byte, error) {

	var pk []byte
	var err error
	switch member.MemberType {
	case pbac.MemberType_CERT:
		pk, err = publicKeyFromCert(member.MemberInfo)
		if err != nil {
			return nil, err
		}

	case pbac.MemberType_CERT_HASH:
		var certInfo *common.CertInfo
		infoHex := hex.EncodeToString(member.MemberInfo)

		var snap protocol.Snapshot
		snap, err = snapshot.NewQuerySnapshot(store, log)
		if err != nil {
			return nil, err
		}

		if certInfo, err = wholeCertInfoFromSnapshot(snap, infoHex); err != nil {
			return nil, fmt.Errorf(" can not load the whole cert info,member[%s],reason: %s", infoHex, err)
		}

		pk, err = publicKeyFromCert(certInfo.Cert)
		if err != nil {
			return nil, err
		}

	case pbac.MemberType_PUBLIC_KEY:
		pk = member.MemberInfo

	default:
		err = fmt.Errorf("invalid member type: %s", member.MemberType)
		return nil, err
	}

	return pk, nil
}

// parseUserAddress
func publicKeyFromCert(member []byte) ([]byte, error) {
	certificate, err := utils.ParseCert(member)
	if err != nil {
		return nil, err
	}
	pubKeyStr, err := certificate.PublicKey.String()
	if err != nil {
		return nil, err
	}
	return []byte(pubKeyStr), nil
}

func wholeCertInfoFromSnapshot(snapshot protocol.Snapshot, certHash string) (*common.CertInfo, error) {
	certBytes, err := snapshot.GetKey(-1, syscontract.SystemContract_CERT_MANAGE.String(), []byte(certHash))
	if err != nil {
		return nil, err
	}

	return &common.CertInfo{
		Hash: certHash,
		Cert: certBytes,
	}, nil
}
