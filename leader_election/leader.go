package leaderelection

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// Node defines the interface for a single participant in the leader election algorithm.
type Node interface {
	// Propose is called by other nodes to request a vote in a given term.
	// The node should grant the vote if the proposal's term is not older
	// than the term it has already voted for.
	Propose(candidateID string, term int) (voteGranted bool)

	// Heartbeat is sent by the current leader to maintain its authority.
	// If a node receives a heartbeat with a term that is older than its current term,
	// it should be rejected.
	Heartbeat(term int) (success bool)

	// ID returns the unique identifier of the node.
	ID() string

	// IsLeader returns true if the node currently believes it is the leader.
	IsLeader() bool

	// Start initiates the node's participation in the leader election process.
	// This usually involves starting goroutines for timers and message handling.
	Start()

	// Stop cleanly shuts down the node.
	Stop()
}

type nodeState int
const (
	Follower nodeState = iota
	Cadidate
	Leader
)

type myNode struct {
	mu sync.Mutex

	// Cluster Info
	id string
	peers map[string]Node

	// Election info
	currentTerm int
	votedFor string
	// Node state
	state nodeState
	// Sync heartbeat to healthcheck goroutinue
	heartbeat chan struct{}
	// Context
	ctx context.Context
	cancel context.CancelFunc
}

func NewNode(id string, peers map[string]Node) Node {
	if peers == nil {
		peers = make(map[string]Node)
	}
	ctx, cancel := context.WithCancel(context.Background())
	node := &myNode{
		id: id,
		peers: peers,
		// skip currentTerm info
		currentTerm: 0,
		votedFor: "",
		state: Follower,
		heartbeat: make(chan struct{}),
		ctx: ctx,
		cancel: cancel,
	}
	return node
}

func randRaftTimeout() time.Duration {
	return time.Duration(rand.Intn(150)+150) * time.Millisecond
}

func (n *myNode) ID() string {
	return n.id
}

func (n *myNode) IsLeader() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.state == Leader
}

func (n *myNode) Stop() {
	n.cancel()
}

func (n *myNode) Start() {
	go n.run()
}

func (n *myNode) run() {
	for {
		n.mu.Lock()
		state := n.state
		n.mu.Unlock()
		switch state {
		case Follower:
			n.runAsFollower()
		case Cadidate:
			n.runAsCandidate()
		case Leader:
			n.runAsLeader()
		}

		select {
		case <- n.ctx.Done():
			fmt.Println("node stopped")
			return
		default:
			// continue the next round, avoid being blocked
			continue
		}
	}
}

func (n *myNode) runAsFollower() {
	electionTimeout := randRaftTimeout()
	select {
	case <- n.heartbeat:
		fmt.Println("[follower] received heartbeat signal")
	case <- time.After(electionTimeout):
		fmt.Println("[follower] election timeout")
		n.mu.Lock()
		n.state = Cadidate
		n.mu.Unlock()
	case <- n.ctx.Done():
		fmt.Println("[follower] node stopped")
	}
}

func (n *myNode) runAsCandidate() {
	n.mu.Lock()
	n.currentTerm++
	n.votedFor = n.id
	term := n.currentTerm
	n.mu.Unlock()

	var w sync.WaitGroup
	var votes int64 = 1
	for id, peer := range n.peers {
		if id == n.id {
			continue
		}
		w.Add(1)
		go func(p Node) {
			defer w.Done()
			if p.Propose(n.id, term) {
				atomic.AddInt64(&votes, 1)
			}
		}(peer)
	}
	w.Wait()

	n.mu.Lock()
	defer n.mu.Unlock()
	if int(votes) > len(n.peers)/2 {
		n.state = Leader
	} else {
		n.state = Follower
	}
}

func (n *myNode) Propose(candidateID string, term int) (voteGranted bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if term > n.currentTerm || (term == n.currentTerm && (candidateID == n.votedFor || n.votedFor == "")) {
		n.currentTerm = term
		n.votedFor = candidateID
		n.state = Follower
		return true
	}
	return false
}

func (n *myNode) runAsLeader() {
	ticker := time.NewTicker(75 * time.Millisecond)
	defer ticker.Stop()
	for {
		n.broadcastHeartbeat()
		select {
		case <- ticker.C:
			fmt.Println("[leader] heartbeat sent")
		case <- n.ctx.Done():
			fmt.Println("[leader] node stopped")
			return
		}
	}
}

func (n *myNode) broadcastHeartbeat() {
	n.mu.Lock()
	term := n.currentTerm
	n.mu.Unlock()
	for id, peer := range n.peers {
		if id == n.id {
			continue
		}
		go func(p Node) {
			p.Heartbeat(term)
		}(peer)
	}
}

func (n *myNode) Heartbeat(term int) (success bool) {
	n.mu.Lock()
	curTerm := n.currentTerm
	n.mu.Unlock()
	if curTerm > term {
		return false
	} else if curTerm <= term {
		n.mu.Lock()
		n.currentTerm = term
		n.state = Follower
		n.mu.Unlock()
	}
	// use non-blocking send
	select {
	case n.heartbeat <- struct{}{}:
	default:
	}
	return true
}