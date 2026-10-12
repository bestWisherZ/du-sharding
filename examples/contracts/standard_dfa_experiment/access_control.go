package main

import (
	"errors"

	"chainmaker.org/chainmaker/contract-sdk-go/v2/sdk"
	"chainmaker.org/chainmaker/contract-utils/standard"
)

// dfaAdminStore adapts DFA admin state to the shared AccessControl interface.
type dfaAdminStore struct {
}

// getAccessControl returns the DFA access control adapter.
func (c *CmdfaContract) getAccessControl() standard.AccessControl {
	if c.accessControl == nil {
		c.accessControl = standard.NewDefaultAccessControl(dfaAdminStore{})
	}
	return c.accessControl
}

// GetAdmins loads the single DFA admin from contract state.
func (dfaAdminStore) GetAdmins() ([]string, error) {
	admin, err := sdk.Instance.GetState(adminKey, "")
	if err != nil {
		return nil, err
	}
	if len(admin) == 0 {
		return nil, errors.New("admin is empty")
	}
	return []string{admin}, nil
}

// SetAdmins stores the single DFA admin in contract state.
func (dfaAdminStore) SetAdmins(admins []string) error {
	if len(admins) != 1 {
		return errors.New("dfa only supports one admin")
	}
	return sdk.Instance.PutState(adminKey, "", admins[0])
}
