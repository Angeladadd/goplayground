package leaderelection_test

import (
	"testing"
	"time"
	. "playground/leader_election"
)

// Helper function to create a cluster of nodes.
// NOTE: You must implement the NewNode function for this to compile.
func createCluster(nodeIDs []string) map[string]Node {
	nodes := make(map[string]Node, len(nodeIDs))
	// First, create all node instances.
	for _, id := range nodeIDs {
		// We pass a nil map initially.
		nodes[id] = NewNode(id, nil)
	}

	// Now, create the final map with all peers and re-initialize nodes.
	// This is a bit of a trick to ensure every node's constructor
	// gets a complete map of all its peers.
	finalNodes := make(map[string]Node, len(nodeIDs))
	for _, id := range nodeIDs {
		finalNodes[id] = NewNode(id, finalNodes)
	}

	return finalNodes
}

func TestSingleNodeBecomesLeader(t *testing.T) {
	t.Parallel()
	cluster := createCluster([]string{"node1"})
	node1 := cluster["node1"]
	node1.Start()
	defer node1.Stop()

	// In a single-node cluster, the node should become leader immediately.
	time.Sleep(300 * time.Millisecond) // Allow time for election

	if !node1.IsLeader() {
		t.Errorf("expected node1 to be leader, but it was not")
	}
}

func TestOneLeaderInThreeNodeCluster(t *testing.T) {
	t.Parallel()
	cluster := createCluster([]string{"node1", "node2", "node3"})
	for _, node := range cluster {
		node.Start()
		defer node.Stop()
	}

	// Allow time for election to settle.
	time.Sleep(500 * time.Millisecond)

	leaderCount := 0
	var leaderID string
	for id, node := range cluster {
		if node.IsLeader() {
			leaderCount++
			leaderID = id
		}
	}

	if leaderCount != 1 {
		t.Errorf("expected exactly one leader, but found %d", leaderCount)
	}
	t.Logf("Node %s was elected leader.", leaderID)
}

func TestLeaderFailureAndReElection(t *testing.T) {
	t.Parallel()
	cluster := createCluster([]string{"node1", "node2", "node3"})
	for _, node := range cluster {
		node.Start()
		defer node.Stop()
	}

	// Allow time for initial election.
	time.Sleep(500 * time.Millisecond)

	var originalLeader Node
	var originalLeaderID string
	for id, node := range cluster {
		if node.IsLeader() {
			originalLeader = node
			originalLeaderID = id
			break
		}
	}

	if originalLeader == nil {
		t.Fatal("no leader was elected initially")
	}
	t.Logf("Initial leader is %s. Stopping it now.", originalLeaderID)

	// Stop the leader.
	originalLeader.Stop()

	// Allow time for re-election among the remaining nodes.
	time.Sleep(500 * time.Millisecond)

	newLeaderCount := 0
	var newLeaderID string
	for id, node := range cluster {
		// Exclude the stopped node from our check.
		if id == originalLeaderID {
			continue
		}
		if node.IsLeader() {
			newLeaderCount++
			newLeaderID = id
		}
	}

	if newLeaderCount != 1 {
		t.Errorf("expected exactly one new leader, but found %d", newLeaderCount)
	}
	if newLeaderID == originalLeaderID {
		t.Errorf("a new leader should have been elected, but the old one is still considered leader")
	}
	t.Logf("New leader elected: %s", newLeaderID)
}