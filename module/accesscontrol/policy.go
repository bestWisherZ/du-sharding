/*
Copyright (C) BABEC. All rights reserved.
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package accesscontrol

import (
	"strings"

	pbac "chainmaker.org/chainmaker/pb-go/v2/accesscontrol"
	"chainmaker.org/chainmaker/protocol/v2"
)

// policy rule 表示的是规则，是对 orgList 的验证， orgList 代表的是组织列表，为空时表示所有组织
// roleList 是在rule规则下对应的角色列表，该角色中任意一个均可
// rule 包括五种情况：RuleAny、RuleAll、RuleMajority、RuleForbidden和RuleSelf
// RuleAny表示任意一个 orgList 中的 roleList 中的任意一个角色均可
// RuleAll表示必须全部的 orgList 中的 roleList 中的任意一个角色，这个全部是对组织进行的约束
// RuleMajority表示超过一半的 orgList 中的 roleList 中的任意一个角色
// RuleForbidden表示禁止所有的组织 orgList 和 roleList 不生效
// RuleSelf表示资源所属的组织提供符合 roleList 要求角色的签名， 在此关键字下，orgList 中的组织列表信息不生效，
// 该规则目前只适用于修改组织根证书、修改组织共识节点地址这两个操作的权限配置，例如某个角色修改自己组织的根证书
// 一个典型的例子，该例子表示的策略是交易中的背书数量必须满足大多数组织中的Admin或Client背书
// protocol.RuleMajority,
// nil,
//
//	[]protocol.Role{
//	  protocol.RoleAdmin,
//	  protocol.RoleClient,
//		},
type policy struct {
	rule     protocol.Rule
	orgList  []string
	roleList []protocol.Role
}

func (p *policy) GetRule() protocol.Rule {
	return p.rule
}

func (p *policy) GetPbPolicy() *pbac.Policy {
	var pbRoleList []string
	for _, role := range p.roleList {
		var roleStr = string(role)
		pbRoleList = append(pbRoleList, roleStr)
	}
	return &pbac.Policy{
		Rule:     string(p.rule),
		OrgList:  p.orgList,
		RoleList: pbRoleList,
	}
}

func (p *policy) GetOrgList() []string {
	return p.orgList
}

func (p *policy) GetRoleList() []protocol.Role {
	return p.roleList
}

func newPolicy(rule protocol.Rule, orgList []string, roleList []protocol.Role) *policy {
	return &policy{
		rule:     rule,
		orgList:  orgList,
		roleList: roleList,
	}
}

func newPolicyFromPb(input *pbac.Policy) *policy {

	p := &policy{
		rule:     protocol.Rule(input.Rule),
		orgList:  input.OrgList,
		roleList: nil,
	}

	for _, role := range input.RoleList {
		role = strings.ToUpper(role)
		p.roleList = append(p.roleList, protocol.Role(role))
	}

	return p
}

func newPbPolicyFromPolicy(input *policy) *pbac.Policy {
	p := &pbac.Policy{
		Rule:     string(input.rule),
		OrgList:  input.orgList,
		RoleList: nil,
	}

	for _, role := range input.roleList {
		p.RoleList = append(p.RoleList, string(role))
	}
	return p
}
