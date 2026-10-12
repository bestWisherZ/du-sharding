package crossshard

import (
	"errors"
	"fmt"
	"testing"

	"chainmaker.org/chainmaker-go/module/sharding/syncproposer"
	commonerrors "chainmaker.org/chainmaker/common/v2/errors"
	"chainmaker.org/chainmaker/localconf/v2"
)

func TestWritebackStopsWhenProposerChangesDuringLookup(t *testing.T) {
	oldLocal, oldCommitted, oldPool, oldSubmit := localconf.ChainMakerConfig, txCommitted, txInPool, submitTx
	oldProposer := syncproposer.IsLocalProposer()
	t.Cleanup(func() {
		localconf.ChainMakerConfig, txCommitted, txInPool, submitTx = oldLocal, oldCommitted, oldPool, oldSubmit
		syncproposer.SetLocalProposer(oldProposer)
	})
	localconf.ChainMakerConfig = &localconf.CMConfig{BlockChainConfig: []localconf.BlockchainConfig{{ChainId: "chain1"}}}
	syncproposer.SetLocalProposer(true)
	txCommitted = func(_, _ string) (bool, error) {
		syncproposer.SetLocalProposer(false)
		return false, nil
	}
	txInPool = func(_, _ string) (bool, error) { return false, nil }
	submitTx = func(_ string, _ interface{}) error {
		t.Fatal("old proposer submitted a transaction")
		return nil
	}
	chunk := &applyWriteChunk{batch: &CrossWriteBatch{}, txId: "test"}
	if err := submitApplyWriteChunk(chunk); err == nil {
		t.Fatal("proposer change must interrupt submission before signing")
	}
}

func TestDuplicateSubmissionRequiresTxpoolError(t *testing.T) {
	for _, err := range []error{commonerrors.ErrTxIdExist, fmt.Errorf("submit: %w", commonerrors.ErrTxIdExistDB)} {
		if !isDuplicateSubmitError(err) {
			t.Fatalf("known duplicate rejected: %v", err)
		}
	}
	for _, err := range []error{nil, errors.New("required certificate does not exist"), errors.New("already disconnected"), errors.New("duplicate state key")} {
		if isDuplicateSubmitError(err) {
			t.Fatalf("unrelated failure suppressed: %v", err)
		}
	}
}
