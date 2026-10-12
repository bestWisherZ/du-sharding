package shardingsync

import (
	"errors"
	"testing"

	"github.com/libp2p/go-libp2p-core/peer"
)

func TestBroadcastDoesNotHideTransportFailures(t *testing.T) {
	failure := errors.New("stream disconnected")
	for _, peers := range [][]peer.ID{nil, {"one"}, {"one", "two"}} {
		calls := 0
		err := sendMessageToPeers(peers, []byte("immutable"), func(p peer.ID, data []byte) error {
			calls++
			if string(data) != "immutable" {
				t.Fatal("message changed")
			}
			if p == "one" {
				return failure
			}
			return nil
		})
		if err == nil || calls != len(peers) {
			t.Fatal("partial/total failure or empty peer set treated as success")
		}
	}
	if err := sendMessageToPeers([]peer.ID{"two"}, nil, func(peer.ID, []byte) error { return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestPeerAnnouncementReplyIsBidirectionalWithoutLoop(t *testing.T) {
	cfg := ShardingConfig{ShardId: "bridge-shard", ShardType: "BRIDGE"}
	response := peerAnnouncementReply(cfg, "HI. I am business-shard1 (BUSINESS), nice to meet you!")
	shard, kind, ok := parsePeerAnnouncement(response)
	if !ok || shard != cfg.ShardId || kind != cfg.ShardType {
		t.Fatal("peer did not learn the destination shard identity")
	}
	if peerAnnouncementReply(cfg, response) != "" || peerAnnouncementReply(cfg, "GLOBAL_STATE_DELTA_V3:business-shard1:data") != "" {
		t.Fatal("acknowledgement loop or state-message reply")
	}
}
