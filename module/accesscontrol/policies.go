package accesscontrol

import (
	"chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/protocol/v2"
)

func (acs *accessControlService) createDefaultResourcePolicyForCert(localOrgId string) {
	// for msg_type
	{
		acs.msgTypePolicyMap.Store(protocol.ResourceNameConsensusNode, policyConsensus)
		acs.msgTypePolicyMap.Store(protocol.ResourceNameP2p, policyP2P)
	}

	// for tx_type
	{
		// 任意的查询请求对应的读
		acs.txTypePolicyMap.Store(common.TxType_QUERY_CONTRACT.String(), policySpecialRead)
		acs.txTypePolicyMap.Store(common.TxType_INVOKE_CONTRACT.String(), policySpecialRead)
		acs.txTypePolicyMap.Store(common.TxType_SUBSCRIBE.String(), policySubscribe)
		acs.txTypePolicyMap.Store(common.TxType_ARCHIVE.String(), policyArchive)

		//自己的管理员可以做链更新操作（如：添加/删除子链，更新Seeds、CustomTrustRoots）
		//todo: self admin 校验失败，错误信息：
		//	SELF keyword requires the owner of the affected target
		//  txTypePolicyMap 不支持设置SELF
		acs.txTypePolicyMap.Store(common.TxType_NODE_CONFIG.String(), policyAdmin)
	}

	// sender & endorsements policy map
	{
		// gas management
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_ACCOUNT_MANAGER.String()+"-"+
			syscontract.GasAccountFunction_SET_ADMIN.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_ACCOUNT_MANAGER.String()+"-"+
			syscontract.GasAccountFunction_CHARGE_GAS_FOR_MULTI_ACCOUNT.String(), policyConsensus)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_ACCOUNT_MANAGER.String()+"-"+
			syscontract.GasAccountFunction_SET_CONTRACT_METHOD_PAYER.String(), policyWrite)

		// cert alias management
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERT_ALIAS_ADD.String(), policySpecialWrite)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERT_ALIAS_UPDATE.String(), policyAdmin)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERTS_ALIAS_DELETE.String(), policyAdmin)

		// cert management
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERT_ADD.String(), policySpecialWrite)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERTS_FREEZE.String(), policyAdmin)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERTS_UNFREEZE.String(), policyAdmin)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERTS_DELETE.String(), policyAdmin)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERTS_REVOKE.String(), policyAdmin)

		// address type management
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_ALTER_ADDR_TYPE.String(), policyForbidden)

		// block management
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_BLOCK_UPDATE.String(), policyConfig)

		// consensus ext management
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_CONSENSUS_EXT_ADD.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_CONSENSUS_EXT_UPDATE.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_CONSENSUS_EXT_DELETE.String(), policyConfig)

		// core management
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_CORE_UPDATE.String(), policyConfig)

		// enable gas flag management
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_ENABLE_OR_DISABLE_GAS.String(), policyConfig)

		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_ENABLE_ONLY_CREATOR_UPGRADE.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_DISABLE_ONLY_CREATOR_UPGRADE.String(), policyConfig)

		// enable 3 phrase multi-sign flag
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_MULTI_SIGN_ENABLE_MANUAL_RUN.String(), policyConfig)

		// node-id management
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_NODE_ID_ADD.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_NODE_ID_DELETE.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_NODE_ID_UPDATE.String(), policySelfConfig)

		// node-org management
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_NODE_ORG_ADD.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_NODE_ORG_UPDATE.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_NODE_ORG_DELETE.String(), policyConfig)

		// permission management
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_PERMISSION_ADD.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_PERMISSION_UPDATE.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_PERMISSION_DELETE.String(), policyConfig)

		// account-manager management
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_SET_ACCOUNT_MANAGER_ADMIN.String(), policyConfig)

		// gas calculation management
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_SET_INSTALL_BASE_GAS.String(), pubPolicyMajorityAdmin)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_SET_INSTALL_GAS_PRICE.String(), pubPolicyMajorityAdmin)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_SET_INVOKE_BASE_GAS.String(), pubPolicyMajorityAdmin)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_SET_INVOKE_GAS_PRICE.String(), pubPolicyMajorityAdmin)

		// trust-member management
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_TRUST_MEMBER_ADD.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_TRUST_MEMBER_DELETE.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_TRUST_MEMBER_UPDATE.String(), policyConfig)

		// trust-root management
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_TRUST_ROOT_ADD.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_TRUST_ROOT_DELETE.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_TRUST_ROOT_UPDATE.String(), policySelfConfig)

		// version management
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_UPDATE_VERSION.String(), policyConfig)

		// archive  management
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_ARCHIVE_MANAGE.String()+"-"+
			syscontract.ArchiveFunction_ARCHIVE_BLOCK.String(), policySelfConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ArchiveFunction_RESTORE_BLOCK.String(), policySelfConfig)

		// contract management
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ContractManageFunction_INIT_CONTRACT.String(), policyAdmin)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ContractManageFunction_UPGRADE_CONTRACT.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ContractManageFunction_FREEZE_CONTRACT.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ContractManageFunction_UNFREEZE_CONTRACT.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ContractManageFunction_REVOKE_CONTRACT.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ContractManageFunction_GRANT_CONTRACT_ACCESS.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ContractManageFunction_REVOKE_CONTRACT_ACCESS.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ContractManageFunction_VERIFY_CONTRACT_ACCESS.String(), policyConfig)

		// for vm support list add new vm type
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_VM_SUPPORT_LIST_ADD.String(), policyConfig)

		// multi-sign management, use default policy

		// private-compute management
		acs.resourceNamePolicyMap.Store(protocol.ResourceNamePrivateCompute, policyWrite)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_PRIVATE_COMPUTE.String()+"-"+
			syscontract.PrivateComputeFunction_SAVE_CA_CERT.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_PRIVATE_COMPUTE.String()+"-"+
			syscontract.PrivateComputeFunction_SAVE_ENCLAVE_REPORT.String(), policyConfig)

		// public-key management
		acs.senderPolicyMap.Store(syscontract.SystemContract_PUBKEY_MANAGE.String()+"-"+
			syscontract.PubkeyManageFunction_PUBKEY_ADD.String(), policyForbidden)
		acs.senderPolicyMap.Store(syscontract.SystemContract_PUBKEY_MANAGE.String()+"-"+
			syscontract.PubkeyManageFunction_PUBKEY_DELETE.String(), policyForbidden)

		// relay cross
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_RELAY_CROSS.String()+"-"+
			syscontract.RelayCrossFunction_SET_CROSS_ADMIN.String(), policyRelayCross)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_RELAY_CROSS.String()+"-"+
			syscontract.RelayCrossFunction_DELETE_CROSS_ADEMIN.String(), policyRelayCross)

		// for sync rule
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_SYNC_RULE.String()+"-"+
			syscontract.SyncRuleFunction_ADD.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_SYNC_RULE.String()+"-"+
			syscontract.SyncRuleFunction_CLEAR.String(), policyConfig)

	}
}
func (acs *accessControlService) createDefaultResourcePolicyForPWK(localOrgId string) {
	// for msg_type
	{
		acs.msgTypePolicyMap.Store(protocol.ResourceNameConsensusNode, policyConsensus)
		acs.msgTypePolicyMap.Store(protocol.ResourceNameP2p, policyP2P)
	}

	// for tx_type
	{
		acs.txTypePolicyMap.Store(common.TxType_QUERY_CONTRACT.String(), policySpecialRead)
		acs.txTypePolicyMap.Store(common.TxType_INVOKE_CONTRACT.String(), policyWrite)
		acs.txTypePolicyMap.Store(common.TxType_SUBSCRIBE.String(), policySubscribe)
		acs.txTypePolicyMap.Store(common.TxType_ARCHIVE.String(), policyArchive)
		acs.txTypePolicyMap.Store(common.TxType_NODE_CONFIG.String(), policyAdmin)
	}

	// sender policies
	{
		// certs alias management
		acs.senderPolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERT_ALIAS_ADD.String(), policyForbidden)
		acs.senderPolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERT_ALIAS_UPDATE.String(), policyForbidden)
		acs.senderPolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERTS_ALIAS_DELETE.String(), policyForbidden)
		acs.senderPolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERTS_ALIAS_QUERY.String(), policyForbidden)

		// certs management
		acs.senderPolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERTS_QUERY.String(), policyForbidden)
		acs.senderPolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERT_ADD.String(), policyForbidden)
		acs.senderPolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERTS_FREEZE.String(), policyForbidden)
		acs.senderPolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERTS_UNFREEZE.String(), policyForbidden)
		acs.senderPolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERTS_DELETE.String(), policyForbidden)
		acs.senderPolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERTS_REVOKE.String(), policyForbidden)

		// address type management
		acs.senderPolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_ALTER_ADDR_TYPE.String(), policyForbidden)
	}
	// for resource
	{
		// gas management
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_ACCOUNT_MANAGER.String()+"-"+
			syscontract.GasAccountFunction_SET_ADMIN.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_ACCOUNT_MANAGER.String()+"-"+
			syscontract.GasAccountFunction_CHARGE_GAS_FOR_MULTI_ACCOUNT.String(), policyConsensus)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_ACCOUNT_MANAGER.String()+"-"+
			syscontract.GasAccountFunction_SET_CONTRACT_METHOD_PAYER.String(), policyWrite)

		// block management
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_BLOCK_UPDATE.String(), policyConfig)

		// consensus ext management
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_CONSENSUS_EXT_ADD.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_CONSENSUS_EXT_UPDATE.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_CONSENSUS_EXT_DELETE.String(), policyConfig)

		// core management
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_CORE_UPDATE.String(), policyConfig)

		// enable gas flag management
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_ENABLE_OR_DISABLE_GAS.String(), policyConfig)

		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_ENABLE_ONLY_CREATOR_UPGRADE.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_DISABLE_ONLY_CREATOR_UPGRADE.String(), policyConfig)

		// enable 3 phrase multi-sign flag
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_MULTI_SIGN_ENABLE_MANUAL_RUN.String(), policyConfig)

		// node-id management
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_NODE_ID_ADD.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_NODE_ID_DELETE.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_NODE_ID_UPDATE.String(), policySelfConfig)

		// node-org management
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_NODE_ORG_ADD.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_NODE_ORG_UPDATE.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_NODE_ORG_DELETE.String(), policyConfig)

		// permission management
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_PERMISSION_ADD.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_PERMISSION_UPDATE.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_PERMISSION_DELETE.String(), policyConfig)

		// account-manager management
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_SET_ACCOUNT_MANAGER_ADMIN.String(), policyConfig)

		// gas calculation management
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_SET_INSTALL_BASE_GAS.String(), pubPolicyMajorityAdmin)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_SET_INSTALL_GAS_PRICE.String(), pubPolicyMajorityAdmin)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_SET_INVOKE_BASE_GAS.String(), pubPolicyMajorityAdmin)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_SET_INVOKE_GAS_PRICE.String(), pubPolicyMajorityAdmin)

		// trust-member management
		acs.senderPolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_TRUST_MEMBER_ADD.String(), policyForbidden)
		acs.senderPolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_TRUST_MEMBER_DELETE.String(), policyForbidden)
		acs.senderPolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_TRUST_MEMBER_UPDATE.String(), policyForbidden)

		// trust-root management
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_TRUST_ROOT_ADD.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_TRUST_ROOT_DELETE.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_TRUST_ROOT_UPDATE.String(), policySelfConfig)

		// version management
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_UPDATE_VERSION.String(), policyConfig)

		// archive management
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_ARCHIVE_MANAGE.String()+"-"+
			syscontract.ArchiveFunction_ARCHIVE_BLOCK.String(), policySelfConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ArchiveFunction_RESTORE_BLOCK.String(), policySelfConfig)

		// contract management
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ContractManageFunction_INIT_CONTRACT.String(), policyAdmin)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ContractManageFunction_UPGRADE_CONTRACT.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ContractManageFunction_FREEZE_CONTRACT.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ContractManageFunction_UNFREEZE_CONTRACT.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ContractManageFunction_REVOKE_CONTRACT.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ContractManageFunction_GRANT_CONTRACT_ACCESS.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ContractManageFunction_REVOKE_CONTRACT_ACCESS.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ContractManageFunction_VERIFY_CONTRACT_ACCESS.String(), policyConfig)

		// for vm support list add new vm type
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_VM_SUPPORT_LIST_ADD.String(), policyConfig)

		// private-compute management
		acs.resourceNamePolicyMap.Store(protocol.ResourceNamePrivateCompute, policyWrite)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_PRIVATE_COMPUTE.String()+"-"+
			syscontract.PrivateComputeFunction_SAVE_CA_CERT.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_PRIVATE_COMPUTE.String()+"-"+
			syscontract.PrivateComputeFunction_SAVE_ENCLAVE_REPORT.String(), policyConfig)

		// public-key management
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_PUBKEY_MANAGE.String()+"-"+
			syscontract.PubkeyManageFunction_PUBKEY_ADD.String(), policySelfConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_PUBKEY_MANAGE.String()+"-"+
			syscontract.PubkeyManageFunction_PUBKEY_DELETE.String(), policySelfConfig)

		// relay cross
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_RELAY_CROSS.String()+"-"+
			syscontract.RelayCrossFunction_SET_CROSS_ADMIN.String(), policyRelayCross)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_RELAY_CROSS.String()+"-"+
			syscontract.RelayCrossFunction_DELETE_CROSS_ADEMIN.String(), policyRelayCross)

		// for sync rule
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_SYNC_RULE.String()+"-"+
			syscontract.SyncRuleFunction_ADD.String(), policyConfig)
		acs.resourceNamePolicyMap.Store(syscontract.SystemContract_SYNC_RULE.String()+"-"+
			syscontract.SyncRuleFunction_CLEAR.String(), policyConfig)
	}
}

func (pk *pkACProvider) createDefaultResourcePolicyForPK() {
	// for msg_type policy
	{
		pk.msgTypePolicyMap.Store(protocol.ResourceNameConsensusNode, policyConsensus)
		pk.msgTypePolicyMap.Store(protocol.ResourceNameP2p, policyP2P)
	}

	// for tx_type policy
	{
		pk.txTypePolicyMap.Store(common.TxType_QUERY_CONTRACT.String(), pubPolicyTransaction)
		pk.txTypePolicyMap.Store(common.TxType_INVOKE_CONTRACT.String(), pubPolicyTransaction)
		pk.txTypePolicyMap.Store(common.TxType_SUBSCRIBE.String(), pubPolicyTransaction)
		pk.txTypePolicyMap.Store(common.TxType_ARCHIVE.String(), pubPolicyManage)

		pk.txTypePolicyMap.Store(common.TxType_NODE_CONFIG.String(), policyAdmin)
	}

	// sender & endorsements policy map
	{
		// gas management
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_ACCOUNT_MANAGER.String()+"-"+
			syscontract.GasAccountFunction_SET_ADMIN.String(), pubPolicyMajorityAdmin)
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_ACCOUNT_MANAGER.String()+"-"+
			syscontract.GasAccountFunction_CHARGE_GAS_FOR_MULTI_ACCOUNT.String(), policyConsensus)
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_ACCOUNT_MANAGER.String()+"-"+
			syscontract.GasAccountFunction_SET_CONTRACT_METHOD_PAYER.String(), policyWrite)

		// cert alias management
		pk.senderPolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERT_ALIAS_ADD.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERT_ALIAS_UPDATE.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERTS_ALIAS_DELETE.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERTS_ALIAS_QUERY.String(), pubPolicyForbidden)

		// cert management
		pk.senderPolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERT_ADD.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERTS_FREEZE.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERTS_UNFREEZE.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERTS_DELETE.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERTS_REVOKE.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERTS_QUERY.String(), policyForbidden)

		// address type management
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_ALTER_ADDR_TYPE.String(), policyForbidden)

		// block management
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_BLOCK_UPDATE.String(), pubPolicyMajorityAdmin)

		// consensus ext management
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_CONSENSUS_EXT_ADD.String(), policyConfig)
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_CONSENSUS_EXT_UPDATE.String(), policyConfig)
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_CONSENSUS_EXT_DELETE.String(), policyConfig)

		// core management
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_CORE_UPDATE.String(), pubPolicyMajorityAdmin)

		// enable gas flag management
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_ENABLE_OR_DISABLE_GAS.String(), pubPolicyMajorityAdmin)

		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_ENABLE_ONLY_CREATOR_UPGRADE.String(), policyConfig)
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_DISABLE_ONLY_CREATOR_UPGRADE.String(), policyConfig)

		// enable 3 phrase multi-sign flag
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_MULTI_SIGN_ENABLE_MANUAL_RUN.String(), pubPolicyMajorityAdmin)

		// node-id management
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_NODE_ID_ADD.String(), pubPolicyMajorityAdmin)
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_NODE_ID_DELETE.String(), pubPolicyMajorityAdmin)
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_NODE_ID_UPDATE.String(), pubPolicyMajorityAdmin)

		// node-org management
		pk.senderPolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_NODE_ORG_ADD.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_NODE_ORG_DELETE.String(), pubPolicyForbidden)
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_NODE_ORG_UPDATE.String(), pubPolicyMajorityAdmin)

		// permission management
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_PERMISSION_ADD.String(), pubPolicyMajorityAdmin)
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_PERMISSION_UPDATE.String(), pubPolicyMajorityAdmin)
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_PERMISSION_DELETE.String(), pubPolicyMajorityAdmin)

		// account-manager management
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_SET_ACCOUNT_MANAGER_ADMIN.String(), pubPolicyMajorityAdmin)

		// gas calculation management
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_SET_INSTALL_BASE_GAS.String(), pubPolicyMajorityAdmin)
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_SET_INSTALL_GAS_PRICE.String(), pubPolicyMajorityAdmin)
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_SET_INVOKE_BASE_GAS.String(), pubPolicyMajorityAdmin)
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_SET_INVOKE_GAS_PRICE.String(), pubPolicyMajorityAdmin)

		// trust-member management
		pk.senderPolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_TRUST_MEMBER_ADD.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_TRUST_MEMBER_DELETE.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_TRUST_MEMBER_UPDATE.String(), pubPolicyForbidden)

		// trust-root management
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_TRUST_ROOT_ADD.String(), pubPolicyMajorityAdmin)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_TRUST_ROOT_ADD.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_TRUST_ROOT_DELETE.String(), pubPolicyForbidden)

		// version management
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_UPDATE_VERSION.String(), pubPolicyMajorityAdmin)

		// archive management
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_ARCHIVE_MANAGE.String()+"-"+
			syscontract.ArchiveFunction_ARCHIVE_BLOCK.String(), policyAdmin)
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ArchiveFunction_RESTORE_BLOCK.String(), policyAdmin)

		// contract management
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ContractManageFunction_INIT_CONTRACT.String(), pubPolicyManage)
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ContractManageFunction_UPGRADE_CONTRACT.String(), pubPolicyManage)
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ContractManageFunction_FREEZE_CONTRACT.String(), pubPolicyManage)
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ContractManageFunction_UNFREEZE_CONTRACT.String(), pubPolicyManage)
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ContractManageFunction_REVOKE_CONTRACT.String(), pubPolicyManage)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ContractManageFunction_GRANT_CONTRACT_ACCESS.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ContractManageFunction_REVOKE_CONTRACT_ACCESS.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ContractManageFunction_VERIFY_CONTRACT_ACCESS.String(), pubPolicyForbidden)

		// for vm support list add new vm type
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_VM_SUPPORT_LIST_ADD.String(), pubPolicyMajorityAdmin)

		// multi-sign management, use default policy

		// private-compute management
		pk.senderPolicyMap.Store(protocol.ResourceNamePrivateCompute, pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_PRIVATE_COMPUTE.String()+"-"+
			syscontract.PrivateComputeFunction_SAVE_CA_CERT.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_PRIVATE_COMPUTE.String()+"-"+
			syscontract.PrivateComputeFunction_SAVE_ENCLAVE_REPORT.String(), pubPolicyForbidden)

		// public-key management
		pk.senderPolicyMap.Store(syscontract.SystemContract_PUBKEY_MANAGE.String()+"-"+
			syscontract.PubkeyManageFunction_PUBKEY_ADD.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_PUBKEY_MANAGE.String()+"-"+
			syscontract.PubkeyManageFunction_PUBKEY_DELETE.String(), pubPolicyForbidden)

		// relay cross
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_RELAY_CROSS.String()+"-"+
			syscontract.RelayCrossFunction_SET_CROSS_ADMIN.String(), policyRelayCross)
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_RELAY_CROSS.String()+"-"+
			syscontract.RelayCrossFunction_DELETE_CROSS_ADEMIN.String(), policyRelayCross)

		// for sync rule
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_SYNC_RULE.String()+"-"+
			syscontract.SyncRuleFunction_ADD.String(), policyConfig)
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_SYNC_RULE.String()+"-"+
			syscontract.SyncRuleFunction_CLEAR.String(), policyConfig)

	}
}

func (pk *pkACProvider) createDefaultResourcePolicyForPKDPoS() {
	// for msg_type policy
	{
		pk.msgTypePolicyMap.Store(protocol.ResourceNameConsensusNode, policyConsensus)
		pk.msgTypePolicyMap.Store(protocol.ResourceNameP2p, policyP2P)
	}

	// for tx_type policy
	{
		pk.txTypePolicyMap.Store(common.TxType_QUERY_CONTRACT.String(), pubPolicyTransaction)
		pk.txTypePolicyMap.Store(common.TxType_INVOKE_CONTRACT.String(), pubPolicyTransaction)
		pk.txTypePolicyMap.Store(common.TxType_SUBSCRIBE.String(), pubPolicyTransaction)
		pk.txTypePolicyMap.Store(common.TxType_ARCHIVE.String(), pubPolicyManage)

		pk.txTypePolicyMap.Store(common.TxType_NODE_CONFIG.String(), policyAdmin)
	}

	// sender & endorsements policy map
	{
		// gas management
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_ACCOUNT_MANAGER.String()+"-"+
			syscontract.GasAccountFunction_SET_ADMIN.String(), pubPolicyMajorityAdmin)
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_ACCOUNT_MANAGER.String()+"-"+
			syscontract.GasAccountFunction_CHARGE_GAS_FOR_MULTI_ACCOUNT.String(), policyConsensus)

		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_ACCOUNT_MANAGER.String()+"-"+
			syscontract.GasAccountFunction_SET_CONTRACT_METHOD_PAYER.String(), policyWrite)

		// certs alias management
		pk.senderPolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERT_ALIAS_ADD.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERT_ALIAS_UPDATE.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERTS_ALIAS_DELETE.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERTS_ALIAS_QUERY.String(), pubPolicyForbidden)

		// certs management
		pk.senderPolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERT_ADD.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERTS_FREEZE.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERTS_UNFREEZE.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERTS_DELETE.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERTS_REVOKE.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
			syscontract.CertManageFunction_CERTS_QUERY.String(), policyForbidden)

		// address type management
		pk.senderPolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_ALTER_ADDR_TYPE.String(), pubPolicyForbidden)

		// block management
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_BLOCK_UPDATE.String(), pubPolicyManage)

		// consensus ext management
		pk.senderPolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_CONSENSUS_EXT_ADD.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_CONSENSUS_EXT_UPDATE.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_CONSENSUS_EXT_DELETE.String(), pubPolicyForbidden)

		// core management
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_CORE_UPDATE.String(), pubPolicyManage)

		// enable gas flag management
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_ENABLE_OR_DISABLE_GAS.String(), pubPolicyMajorityAdmin)

		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_ENABLE_ONLY_CREATOR_UPGRADE.String(), policyConfig)
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_DISABLE_ONLY_CREATOR_UPGRADE.String(), policyConfig)

		// enable 3 phrase multi-sign flag
		pk.senderPolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_MULTI_SIGN_ENABLE_MANUAL_RUN.String(), pubPolicyForbidden)

		// node-id management
		pk.senderPolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_NODE_ID_ADD.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_NODE_ID_DELETE.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_NODE_ID_UPDATE.String(), pubPolicyForbidden)

		// node-org management
		pk.senderPolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_NODE_ORG_ADD.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_NODE_ORG_DELETE.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_NODE_ORG_UPDATE.String(), pubPolicyForbidden)

		// permission management
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_PERMISSION_ADD.String(), pubPolicyMajorityAdmin)
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_PERMISSION_UPDATE.String(), pubPolicyMajorityAdmin)
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_PERMISSION_DELETE.String(), pubPolicyMajorityAdmin)

		// account-manager management
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_SET_ACCOUNT_MANAGER_ADMIN.String(), pubPolicyMajorityAdmin)

		// gas calculation management
		pk.senderPolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_SET_INSTALL_BASE_GAS.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_SET_INSTALL_GAS_PRICE.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_SET_INVOKE_BASE_GAS.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_SET_INVOKE_GAS_PRICE.String(), pubPolicyForbidden)

		// trust-member management
		pk.senderPolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_TRUST_MEMBER_ADD.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_TRUST_MEMBER_DELETE.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_TRUST_MEMBER_UPDATE.String(), pubPolicyForbidden)

		// trust-member management
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_TRUST_ROOT_UPDATE.String(), pubPolicyMajorityAdmin)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_TRUST_ROOT_ADD.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_TRUST_ROOT_DELETE.String(), pubPolicyForbidden)

		// version management
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_UPDATE_VERSION.String(), pubPolicyMajorityAdmin)

		// contract management
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ContractManageFunction_INIT_CONTRACT.String(), pubPolicyManage)
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ContractManageFunction_UPGRADE_CONTRACT.String(), pubPolicyManage)
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ContractManageFunction_FREEZE_CONTRACT.String(), pubPolicyManage)
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ContractManageFunction_UNFREEZE_CONTRACT.String(), pubPolicyManage)
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ContractManageFunction_REVOKE_CONTRACT.String(), pubPolicyManage)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ContractManageFunction_GRANT_CONTRACT_ACCESS.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ContractManageFunction_REVOKE_CONTRACT_ACCESS.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
			syscontract.ContractManageFunction_VERIFY_CONTRACT_ACCESS.String(), pubPolicyForbidden)

		// for vm support list add new vm type
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
			syscontract.ChainConfigFunction_VM_SUPPORT_LIST_ADD.String(), pubPolicyMajorityAdmin)

		// multi-sign management
		pk.senderPolicyMap.Store(syscontract.SystemContract_MULTI_SIGN.String()+"-"+
			syscontract.MultiSignFunction_REQ.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_MULTI_SIGN.String()+"-"+
			syscontract.MultiSignFunction_VOTE.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_MULTI_SIGN.String()+"-"+
			syscontract.MultiSignFunction_QUERY.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_MULTI_SIGN.String()+"-"+
			syscontract.MultiSignFunction_TRIG.String(), pubPolicyForbidden)

		// private-compute management
		pk.senderPolicyMap.Store(protocol.ResourceNamePrivateCompute, pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_PRIVATE_COMPUTE.String()+"-"+
			syscontract.PrivateComputeFunction_SAVE_CA_CERT.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_PRIVATE_COMPUTE.String()+"-"+
			syscontract.PrivateComputeFunction_SAVE_ENCLAVE_REPORT.String(), pubPolicyForbidden)

		// public-key management
		pk.senderPolicyMap.Store(syscontract.SystemContract_PUBKEY_MANAGE.String()+"-"+
			syscontract.PubkeyManageFunction_PUBKEY_ADD.String(), pubPolicyForbidden)
		pk.senderPolicyMap.Store(syscontract.SystemContract_PUBKEY_MANAGE.String()+"-"+
			syscontract.PubkeyManageFunction_PUBKEY_DELETE.String(), pubPolicyForbidden)

		// relay cross
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_RELAY_CROSS.String()+"-"+
			syscontract.RelayCrossFunction_SET_CROSS_ADMIN.String(), policyRelayCross)
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_RELAY_CROSS.String()+"-"+
			syscontract.RelayCrossFunction_DELETE_CROSS_ADEMIN.String(), policyRelayCross)

		// for sync rule
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_SYNC_RULE.String()+"-"+
			syscontract.SyncRuleFunction_ADD.String(), policyConfig)
		pk.resourceNamePolicyMap.Store(syscontract.SystemContract_SYNC_RULE.String()+"-"+
			syscontract.SyncRuleFunction_CLEAR.String(), policyConfig)

	}
}
