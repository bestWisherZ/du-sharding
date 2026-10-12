package accesscontrol

import (
	"chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/protocol/v2"
)

func (acs *accessControlService) createDefaultResourcePolicyForCert_2320() {

	acs.resourceNamePolicyMap2320.Store(protocol.ResourceNameReadData, policyRead)
	acs.resourceNamePolicyMap2320.Store(protocol.ResourceNameWriteData, policyWrite)
	acs.resourceNamePolicyMap2320.Store(protocol.ResourceNameUpdateSelfConfig, policySelfConfig)
	acs.resourceNamePolicyMap2320.Store(protocol.ResourceNameUpdateConfig, policyConfig)
	acs.resourceNamePolicyMap2320.Store(protocol.ResourceNameConsensusNode, policyConsensus)
	acs.resourceNamePolicyMap2320.Store(protocol.ResourceNameP2p, policyP2P)

	// for txtype
	acs.resourceNamePolicyMap2320.Store(common.TxType_QUERY_CONTRACT.String(), policyRead)
	acs.resourceNamePolicyMap2320.Store(common.TxType_INVOKE_CONTRACT.String(), policyWrite)
	acs.resourceNamePolicyMap2320.Store(common.TxType_SUBSCRIBE.String(), policySubscribe)
	acs.resourceNamePolicyMap2320.Store(common.TxType_ARCHIVE.String(), policyArchive)

	// exceptional resourceName opened for light user
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_QUERY.String()+"-"+
		syscontract.ChainQueryFunction_GET_BLOCK_BY_HEIGHT.String(), policySpecialRead)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_QUERY.String()+"-"+
		syscontract.ChainQueryFunction_GET_BLOCK_WITH_TXRWSETS_BY_HEIGHT.String(), policySpecialRead)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_QUERY.String()+"-"+
		syscontract.ChainQueryFunction_GET_BLOCK_BY_HASH.String(), policySpecialRead)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_QUERY.String()+"-"+
		syscontract.ChainQueryFunction_GET_BLOCK_WITH_TXRWSETS_BY_HASH.String(), policySpecialRead)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_QUERY.String()+"-"+
		syscontract.ChainQueryFunction_GET_BLOCK_BY_TX_ID.String(), policySpecialRead)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_QUERY.String()+"-"+
		syscontract.ChainQueryFunction_GET_TX_BY_TX_ID.String(), policySpecialRead)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_QUERY.String()+"-"+
		syscontract.ChainQueryFunction_GET_LAST_CONFIG_BLOCK.String(), policySpecialRead)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_QUERY.String()+"-"+
		syscontract.ChainQueryFunction_GET_LAST_BLOCK.String(), policySpecialRead)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_QUERY.String()+"-"+
		syscontract.ChainQueryFunction_GET_FULL_BLOCK_BY_HEIGHT.String(), policySpecialRead)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_QUERY.String()+"-"+
		syscontract.ChainQueryFunction_GET_BLOCK_HEIGHT_BY_TX_ID.String(), policySpecialRead)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_QUERY.String()+"-"+
		syscontract.ChainQueryFunction_GET_BLOCK_HEIGHT_BY_HASH.String(), policySpecialRead)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_QUERY.String()+"-"+
		syscontract.ChainQueryFunction_GET_BLOCK_HEADER_BY_HEIGHT.String(), policySpecialRead)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_QUERY.String()+"-"+
		syscontract.ChainQueryFunction_GET_ARCHIVED_BLOCK_HEIGHT.String(), policySpecialRead)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_GET_CHAIN_CONFIG.String(), policySpecialRead)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERTS_QUERY.String(), policySpecialRead)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERT_ADD.String(), policySpecialWrite)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERTS_ALIAS_QUERY.String(), policySpecialRead)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERT_ALIAS_ADD.String(), policySpecialWrite)

	// Disable pubkey management for cert mode
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_PUBKEY_MANAGE.String()+"-"+
		syscontract.PubkeyManageFunction_PUBKEY_ADD.String(), policyForbidden)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_PUBKEY_MANAGE.String()+"-"+
		syscontract.PubkeyManageFunction_PUBKEY_DELETE.String(), policyForbidden)

	//for private compute
	acs.resourceNamePolicyMap2320.Store(protocol.ResourceNamePrivateCompute, policyWrite)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_PRIVATE_COMPUTE.String()+"-"+
		syscontract.PrivateComputeFunction_SAVE_CA_CERT.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_PRIVATE_COMPUTE.String()+"-"+
		syscontract.PrivateComputeFunction_SAVE_ENCLAVE_REPORT.String(), policyConfig)

	// system contract interface resource definitions
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_CORE_UPDATE.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_UPDATE_VERSION.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_BLOCK_UPDATE.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_UPDATE_VERSION.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_TRUST_ROOT_ADD.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_TRUST_ROOT_DELETE.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_TRUST_ROOT_UPDATE.String(), policySelfConfig)

	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_TRUST_MEMBER_ADD.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_TRUST_MEMBER_DELETE.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_TRUST_MEMBER_UPDATE.String(), policyConfig)

	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_NODE_ID_ADD.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_NODE_ID_DELETE.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_NODE_ID_UPDATE.String(), policySelfConfig)

	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_NODE_ORG_ADD.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_NODE_ORG_UPDATE.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_NODE_ORG_DELETE.String(), policyConfig)

	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_CONSENSUS_EXT_ADD.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_CONSENSUS_EXT_UPDATE.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_CONSENSUS_EXT_DELETE.String(), policyConfig)

	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_PERMISSION_ADD.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_PERMISSION_UPDATE.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_PERMISSION_DELETE.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_ALTER_ADDR_TYPE.String(), policyConfig)
	// add majority permission for gas enable/disable config under cert mode
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_ENABLE_OR_DISABLE_GAS.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_ACCOUNT_MANAGER.String()+"-"+
		syscontract.GasAccountFunction_SET_ADMIN.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_SET_ACCOUNT_MANAGER_ADMIN.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_SET_INVOKE_BASE_GAS.String(), policyConfig)
	// add majority permission for multi sign enable/disable enable_manual_run mode
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_MULTI_SIGN_ENABLE_MANUAL_RUN.String(), policyConfig)

	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
		syscontract.ContractManageFunction_INIT_CONTRACT.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
		syscontract.ContractManageFunction_UPGRADE_CONTRACT.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
		syscontract.ContractManageFunction_FREEZE_CONTRACT.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
		syscontract.ContractManageFunction_UNFREEZE_CONTRACT.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
		syscontract.ContractManageFunction_REVOKE_CONTRACT.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
		syscontract.ContractManageFunction_GRANT_CONTRACT_ACCESS.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
		syscontract.ContractManageFunction_REVOKE_CONTRACT_ACCESS.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
		syscontract.ContractManageFunction_VERIFY_CONTRACT_ACCESS.String(), policyConfig)

	// certificate management
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERTS_FREEZE.String(), policyAdmin)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERTS_UNFREEZE.String(), policyAdmin)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERTS_DELETE.String(), policyAdmin)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERTS_REVOKE.String(), policyAdmin)
	// for cert_alias
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERT_ALIAS_UPDATE.String(), policyAdmin)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERTS_ALIAS_DELETE.String(), policyAdmin)

	// for charge gas in optimize mode
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_ACCOUNT_MANAGER.String()+"-"+
		syscontract.GasAccountFunction_CHARGE_GAS_FOR_MULTI_ACCOUNT.String(), policyConsensus)
}

func (acs *accessControlService) createDefaultResourcePolicyForPK_2320() {

	acs.resourceNamePolicyMap2320.Store(protocol.ResourceNameReadData, policyRead)
	acs.resourceNamePolicyMap2320.Store(protocol.ResourceNameWriteData, policyWrite)
	acs.resourceNamePolicyMap2320.Store(protocol.ResourceNameUpdateSelfConfig, policySelfConfig)
	acs.resourceNamePolicyMap2320.Store(protocol.ResourceNameUpdateConfig, policyConfig)
	acs.resourceNamePolicyMap2320.Store(protocol.ResourceNameConsensusNode, policyConsensus)
	acs.resourceNamePolicyMap2320.Store(protocol.ResourceNameP2p, policyP2P)

	// for txtype
	acs.resourceNamePolicyMap2320.Store(common.TxType_QUERY_CONTRACT.String(), policyRead)
	acs.resourceNamePolicyMap2320.Store(common.TxType_INVOKE_CONTRACT.String(), policyWrite)
	acs.resourceNamePolicyMap2320.Store(common.TxType_SUBSCRIBE.String(), policySubscribe)
	acs.resourceNamePolicyMap2320.Store(common.TxType_ARCHIVE.String(), policyArchive)

	// exceptional resourceName opened for light user
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_QUERY.String()+"-"+
		syscontract.ChainQueryFunction_GET_BLOCK_BY_HEIGHT.String(), policySpecialRead)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_QUERY.String()+"-"+
		syscontract.ChainQueryFunction_GET_BLOCK_WITH_TXRWSETS_BY_HEIGHT.String(), policySpecialRead)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_QUERY.String()+"-"+
		syscontract.ChainQueryFunction_GET_BLOCK_BY_HASH.String(), policySpecialRead)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_QUERY.String()+"-"+
		syscontract.ChainQueryFunction_GET_BLOCK_WITH_TXRWSETS_BY_HASH.String(), policySpecialRead)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_QUERY.String()+"-"+
		syscontract.ChainQueryFunction_GET_BLOCK_BY_TX_ID.String(), policySpecialRead)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_QUERY.String()+"-"+
		syscontract.ChainQueryFunction_GET_TX_BY_TX_ID.String(), policySpecialRead)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_QUERY.String()+"-"+
		syscontract.ChainQueryFunction_GET_LAST_CONFIG_BLOCK.String(), policySpecialRead)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_QUERY.String()+"-"+
		syscontract.ChainQueryFunction_GET_LAST_BLOCK.String(), policySpecialRead)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_QUERY.String()+"-"+
		syscontract.ChainQueryFunction_GET_FULL_BLOCK_BY_HEIGHT.String(), policySpecialRead)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_QUERY.String()+"-"+
		syscontract.ChainQueryFunction_GET_BLOCK_HEIGHT_BY_TX_ID.String(), policySpecialRead)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_QUERY.String()+"-"+
		syscontract.ChainQueryFunction_GET_BLOCK_HEIGHT_BY_HASH.String(), policySpecialRead)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_QUERY.String()+"-"+
		syscontract.ChainQueryFunction_GET_BLOCK_HEADER_BY_HEIGHT.String(), policySpecialRead)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_QUERY.String()+"-"+
		syscontract.ChainQueryFunction_GET_ARCHIVED_BLOCK_HEIGHT.String(), policySpecialRead)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_GET_CHAIN_CONFIG.String(), policySpecialRead)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERTS_QUERY.String(), policySpecialRead)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERT_ADD.String(), policySpecialWrite)

	// Disable certificate management for pk mode
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERT_ADD.String(), policyForbidden)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERTS_FREEZE.String(), policyForbidden)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERTS_UNFREEZE.String(), policyForbidden)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERTS_DELETE.String(), policyForbidden)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERTS_REVOKE.String(), policyForbidden)

	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERT_ALIAS_ADD.String(), policyForbidden)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERT_ALIAS_UPDATE.String(), policyForbidden)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERTS_ALIAS_DELETE.String(), policyForbidden)

	// Disable trust member management for pk mode
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_TRUST_MEMBER_ADD.String(), policyForbidden)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_TRUST_MEMBER_DELETE.String(), policyForbidden)
	acs.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_TRUST_MEMBER_UPDATE.String(), policyForbidden)

	//for private compute
	acs.resourceNamePolicyMap2320.Store(protocol.ResourceNamePrivateCompute, policyWrite)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_PRIVATE_COMPUTE.String()+"-"+
		syscontract.PrivateComputeFunction_SAVE_CA_CERT.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_PRIVATE_COMPUTE.String()+"-"+
		syscontract.PrivateComputeFunction_SAVE_ENCLAVE_REPORT.String(), policyConfig)

	// system contract interface resource definitions
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_CORE_UPDATE.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_UPDATE_VERSION.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_BLOCK_UPDATE.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_UPDATE_VERSION.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_TRUST_ROOT_ADD.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_TRUST_ROOT_DELETE.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_TRUST_ROOT_UPDATE.String(), policySelfConfig)

	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_NODE_ID_ADD.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_NODE_ID_DELETE.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_NODE_ID_UPDATE.String(), policySelfConfig)

	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_NODE_ORG_ADD.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_NODE_ORG_UPDATE.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_NODE_ORG_DELETE.String(), policyConfig)

	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_CONSENSUS_EXT_ADD.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_CONSENSUS_EXT_UPDATE.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_CONSENSUS_EXT_DELETE.String(), policyConfig)

	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_PERMISSION_ADD.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_PERMISSION_UPDATE.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_PERMISSION_DELETE.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_ALTER_ADDR_TYPE.String(), policyConfig)
	// add majority permission for gas enable/disable config under pwk mode
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_ENABLE_OR_DISABLE_GAS.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_ACCOUNT_MANAGER.String()+"-"+
		syscontract.GasAccountFunction_SET_ADMIN.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_SET_ACCOUNT_MANAGER_ADMIN.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_SET_INVOKE_BASE_GAS.String(), policyConfig)
	// add majority permission for multi sign enable/disable enable_manual_run mode
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_MULTI_SIGN_ENABLE_MANUAL_RUN.String(), policyConfig)

	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
		syscontract.ContractManageFunction_INIT_CONTRACT.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
		syscontract.ContractManageFunction_UPGRADE_CONTRACT.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
		syscontract.ContractManageFunction_FREEZE_CONTRACT.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
		syscontract.ContractManageFunction_UNFREEZE_CONTRACT.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
		syscontract.ContractManageFunction_REVOKE_CONTRACT.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
		syscontract.ContractManageFunction_GRANT_CONTRACT_ACCESS.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
		syscontract.ContractManageFunction_REVOKE_CONTRACT_ACCESS.String(), policyConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
		syscontract.ContractManageFunction_VERIFY_CONTRACT_ACCESS.String(), policyConfig)

	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_PUBKEY_MANAGE.String()+"-"+
		syscontract.PubkeyManageFunction_PUBKEY_ADD.String(), policySelfConfig)
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_PUBKEY_MANAGE.String()+"-"+
		syscontract.PubkeyManageFunction_PUBKEY_DELETE.String(), policySelfConfig)

	// for charging gas in optimize mode
	acs.resourceNamePolicyMap2320.Store(syscontract.SystemContract_ACCOUNT_MANAGER.String()+"-"+
		syscontract.GasAccountFunction_CHARGE_GAS_FOR_MULTI_ACCOUNT.String(), policyConsensus)
}

func (pk *pkACProvider) createDefaultResourcePolicyForCommon_2320() {
	pk.resourceNamePolicyMap2320.Store(protocol.ResourceNameReadData, policyRead)
	pk.resourceNamePolicyMap2320.Store(protocol.ResourceNameWriteData, policyWrite)
	pk.resourceNamePolicyMap2320.Store(protocol.ResourceNameUpdateSelfConfig, policySelfConfig)
	pk.resourceNamePolicyMap2320.Store(protocol.ResourceNameUpdateConfig, policyConfig)
	pk.resourceNamePolicyMap2320.Store(protocol.ResourceNameConsensusNode, policyConsensus)
	pk.resourceNamePolicyMap2320.Store(protocol.ResourceNameP2p, policyP2P)

	// for txtype
	pk.resourceNamePolicyMap2320.Store(common.TxType_QUERY_CONTRACT.String(), pubPolicyTransaction)
	pk.resourceNamePolicyMap2320.Store(common.TxType_INVOKE_CONTRACT.String(), pubPolicyTransaction)
	pk.resourceNamePolicyMap2320.Store(common.TxType_SUBSCRIBE.String(), pubPolicyTransaction)
	pk.resourceNamePolicyMap2320.Store(common.TxType_ARCHIVE.String(), pubPolicyManage)

	// exceptional resourceName
	pk.exceptionalPolicyMap2320.Store(protocol.ResourceNamePrivateCompute, pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_PRIVATE_COMPUTE.String()+"-"+
		syscontract.PrivateComputeFunction_SAVE_CA_CERT.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_PRIVATE_COMPUTE.String()+"-"+
		syscontract.PrivateComputeFunction_SAVE_ENCLAVE_REPORT.String(), pubPolicyForbidden)

	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_TRUST_MEMBER_ADD.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_TRUST_MEMBER_DELETE.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_TRUST_MEMBER_UPDATE.String(), pubPolicyForbidden)

	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_NODE_ID_ADD.String(), pubPolicyMajorityAdmin)
	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_NODE_ID_DELETE.String(), pubPolicyMajorityAdmin)
	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_NODE_ID_UPDATE.String(), pubPolicyMajorityAdmin)
	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_NODE_ORG_UPDATE.String(), pubPolicyMajorityAdmin)

	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_NODE_ORG_ADD.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_NODE_ORG_DELETE.String(), pubPolicyForbidden)

	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_PERMISSION_ADD.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_PERMISSION_UPDATE.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_PERMISSION_DELETE.String(), pubPolicyForbidden)

	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERT_ADD.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERTS_FREEZE.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERTS_UNFREEZE.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERTS_DELETE.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERTS_REVOKE.String(), pubPolicyForbidden)

	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERT_ALIAS_ADD.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERT_ALIAS_UPDATE.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERTS_ALIAS_DELETE.String(), pubPolicyForbidden)

	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_PUBKEY_MANAGE.String()+"-"+
		syscontract.PubkeyManageFunction_PUBKEY_ADD.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_PUBKEY_MANAGE.String()+"-"+
		syscontract.PubkeyManageFunction_PUBKEY_DELETE.String(), pubPolicyForbidden)

	// disable contract access for public mode
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
		syscontract.ContractManageFunction_GRANT_CONTRACT_ACCESS.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
		syscontract.ContractManageFunction_REVOKE_CONTRACT_ACCESS.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
		syscontract.ContractManageFunction_VERIFY_CONTRACT_ACCESS.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
		syscontract.ContractQueryFunction_GET_DISABLED_CONTRACT_LIST.String(), pubPolicyForbidden)

	// forbidden charge gas by go sdk
	//pk.exceptionalPolicyMap.Store(syscontract.SystemContract_ACCOUNT_MANAGER.String()+"-"+
	//	syscontract.GasAccountFunction_CHARGE_GAS.String(), pubPolicyForbidden)

	// forbidden refund gas vm by go sdk
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_ACCOUNT_MANAGER.String()+"-"+
		syscontract.GasAccountFunction_REFUND_GAS_VM.String(), pubPolicyForbidden)

	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
		syscontract.ContractManageFunction_INIT_CONTRACT.String(), pubPolicyManage)
	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
		syscontract.ContractManageFunction_UPGRADE_CONTRACT.String(), pubPolicyManage)
	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
		syscontract.ContractManageFunction_FREEZE_CONTRACT.String(), pubPolicyManage)
	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
		syscontract.ContractManageFunction_UNFREEZE_CONTRACT.String(), pubPolicyManage)
	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
		syscontract.ContractManageFunction_REVOKE_CONTRACT.String(), pubPolicyManage)

	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_CORE_UPDATE.String(), pubPolicyMajorityAdmin)
	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_BLOCK_UPDATE.String(), pubPolicyMajorityAdmin)
	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_UPDATE_VERSION.String(), pubPolicyMajorityAdmin)
	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_ENABLE_OR_DISABLE_GAS.String(), pubPolicyMajorityAdmin)
	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_ALTER_ADDR_TYPE.String(), pubPolicyMajorityAdmin)

	// for admin management
	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_TRUST_ROOT_UPDATE.String(), pubPolicyMajorityAdmin)
	// disable trust root add & delete for public mode
	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_TRUST_ROOT_ADD.String(), pubPolicyMajorityAdmin)
	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_TRUST_ROOT_DELETE.String(), pubPolicyMajorityAdmin)

	// for consensus ext xxx
	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_CONSENSUS_EXT_ADD.String(), policyConfig)
	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_CONSENSUS_EXT_UPDATE.String(), policyConfig)
	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_CONSENSUS_EXT_DELETE.String(), policyConfig)

	// for gas admin
	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_ACCOUNT_MANAGER.String()+"-"+
		syscontract.GasAccountFunction_SET_ADMIN.String(), pubPolicyMajorityAdmin)
	// for charge gas in optimize mode
	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_ACCOUNT_MANAGER.String()+"-"+
		syscontract.GasAccountFunction_CHARGE_GAS_FOR_MULTI_ACCOUNT.String(), policyConsensus)
	// for set invoke base gas
	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_SET_INVOKE_BASE_GAS.String(), pubPolicyMajorityAdmin)
	// move set admin method to chain config module
	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_SET_ACCOUNT_MANAGER_ADMIN.String(), pubPolicyMajorityAdmin)

	// for multi-sign enable_manual_run
	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_MULTI_SIGN_ENABLE_MANUAL_RUN.String(), pubPolicyMajorityAdmin)
}

// need to consistent with 2.1.0 for dpos
func (pk *pkACProvider) createDefaultResourcePolicyForDPoS_2320() {
	pk.resourceNamePolicyMap2320.Store(protocol.ResourceNameReadData, policyRead)
	pk.resourceNamePolicyMap2320.Store(protocol.ResourceNameWriteData, policyWrite)
	pk.resourceNamePolicyMap2320.Store(protocol.ResourceNameUpdateSelfConfig, policySelfConfig)
	pk.resourceNamePolicyMap2320.Store(protocol.ResourceNameUpdateConfig, policyConfig)
	pk.resourceNamePolicyMap2320.Store(protocol.ResourceNameConsensusNode, policyConsensus)
	pk.resourceNamePolicyMap2320.Store(protocol.ResourceNameP2p, policyP2P)

	// for txtype
	pk.resourceNamePolicyMap2320.Store(common.TxType_QUERY_CONTRACT.String(), pubPolicyTransaction)
	pk.resourceNamePolicyMap2320.Store(common.TxType_INVOKE_CONTRACT.String(), pubPolicyTransaction)
	pk.resourceNamePolicyMap2320.Store(common.TxType_SUBSCRIBE.String(), pubPolicyTransaction)
	pk.resourceNamePolicyMap2320.Store(common.TxType_ARCHIVE.String(), pubPolicyManage)

	// exceptional resourceName
	pk.exceptionalPolicyMap2320.Store(protocol.ResourceNamePrivateCompute, pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_PRIVATE_COMPUTE.String()+"-"+
		syscontract.PrivateComputeFunction_SAVE_CA_CERT.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_PRIVATE_COMPUTE.String()+"-"+
		syscontract.PrivateComputeFunction_SAVE_ENCLAVE_REPORT.String(), pubPolicyForbidden)

	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_TRUST_MEMBER_ADD.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_TRUST_MEMBER_DELETE.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_TRUST_MEMBER_UPDATE.String(), pubPolicyForbidden)

	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_NODE_ID_ADD.String(), pubPolicyForbidden)
	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_NODE_ID_DELETE.String(), pubPolicyForbidden)
	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_NODE_ID_UPDATE.String(), pubPolicyForbidden)

	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_NODE_ORG_ADD.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_NODE_ORG_UPDATE.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_NODE_ORG_DELETE.String(), pubPolicyForbidden)

	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_PERMISSION_ADD.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_PERMISSION_UPDATE.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_PERMISSION_DELETE.String(), pubPolicyForbidden)

	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERT_ADD.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERTS_FREEZE.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERTS_UNFREEZE.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERTS_DELETE.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERTS_REVOKE.String(), pubPolicyForbidden)

	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERT_ALIAS_ADD.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERT_ALIAS_UPDATE.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CERT_MANAGE.String()+"-"+
		syscontract.CertManageFunction_CERTS_ALIAS_DELETE.String(), pubPolicyForbidden)

	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_PUBKEY_MANAGE.String()+"-"+
		syscontract.PubkeyManageFunction_PUBKEY_ADD.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_PUBKEY_MANAGE.String()+"-"+
		syscontract.PubkeyManageFunction_PUBKEY_DELETE.String(), pubPolicyForbidden)

	// multisign enable_manual_run
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_MULTI_SIGN_ENABLE_MANUAL_RUN.String(), pubPolicyForbidden)
	// disable multisign for public mode
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_MULTI_SIGN.String()+"-"+
		syscontract.MultiSignFunction_REQ.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_MULTI_SIGN.String()+"-"+
		syscontract.MultiSignFunction_VOTE.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_MULTI_SIGN.String()+"-"+
		syscontract.MultiSignFunction_QUERY.String(), pubPolicyForbidden)

	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_CORE_UPDATE.String(), pubPolicyManage)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_BLOCK_UPDATE.String(), pubPolicyManage)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_UPDATE_VERSION.String(), pubPolicyManage)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
		syscontract.ContractManageFunction_UPGRADE_CONTRACT.String(), pubPolicyManage)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
		syscontract.ContractManageFunction_FREEZE_CONTRACT.String(), pubPolicyManage)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
		syscontract.ContractManageFunction_UNFREEZE_CONTRACT.String(), pubPolicyManage)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
		syscontract.ContractManageFunction_REVOKE_CONTRACT.String(), pubPolicyManage)
	// disable contract access for public mode
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
		syscontract.ContractManageFunction_GRANT_CONTRACT_ACCESS.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
		syscontract.ContractManageFunction_REVOKE_CONTRACT_ACCESS.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
		syscontract.ContractManageFunction_VERIFY_CONTRACT_ACCESS.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CONTRACT_MANAGE.String()+"-"+
		syscontract.ContractQueryFunction_GET_DISABLED_CONTRACT_LIST.String(), pubPolicyForbidden)

	// disable gas related native contract
	//pk.exceptionalPolicyMap.Store(syscontract.SystemContract_ACCOUNT_MANAGER.String()+"-"+
	//	syscontract.GasAccountFunction_CHARGE_GAS.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_ACCOUNT_MANAGER.String()+"-"+
		syscontract.GasAccountFunction_REFUND_GAS_VM.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_ACCOUNT_MANAGER.String()+"-"+
		syscontract.GasAccountFunction_SET_ADMIN.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_ENABLE_OR_DISABLE_GAS.String(), pubPolicyForbidden)
	pk.exceptionalPolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_ALTER_ADDR_TYPE.String(), pubPolicyForbidden)
	// for charge gas in optimize mode
	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_ACCOUNT_MANAGER.String()+"-"+
		syscontract.GasAccountFunction_CHARGE_GAS_FOR_MULTI_ACCOUNT.String(), policyConsensus)

	// for admin management
	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_TRUST_ROOT_UPDATE.String(), pubPolicyMajorityAdmin)
	// disable trust root add & delete for public mode
	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_TRUST_ROOT_ADD.String(), pubPolicyMajorityAdmin)
	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_TRUST_ROOT_DELETE.String(), pubPolicyMajorityAdmin)

	// for consensus ext xxx
	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_CONSENSUS_EXT_ADD.String(), pubPolicyForbidden)
	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_CONSENSUS_EXT_UPDATE.String(), pubPolicyForbidden)
	pk.resourceNamePolicyMap2320.Store(syscontract.SystemContract_CHAIN_CONFIG.String()+"-"+
		syscontract.ChainConfigFunction_CONSENSUS_EXT_DELETE.String(), pubPolicyForbidden)
}
