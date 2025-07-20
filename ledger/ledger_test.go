package ledger

import (
	"sync"
	"testing"
)

// TestTransaction_Commit verifies that changes are correctly applied after a commit.
func TestTransaction_Commit(t *testing.T) {
	ledger := NewTransactionalLedger()
	// An empty txID ("") means auto-commit for setting initial state.
	// We use this to set up the initial balance for Alice.
	if err := ledger.Deposit("", "alice", 500); err != nil {
		t.Fatalf("Initial deposit failed: %v", err)
	}

	// Start a new transaction.
	tx1 := ledger.Begin()
	if err := ledger.Deposit(tx1, "alice", 100); err != nil {
		t.Fatalf("Deposit within tx failed: %v", err)
	}

	// Check balance inside the transaction. It should reflect the pending deposit.
	balance, err := ledger.GetBalance(tx1, "alice")
	if err != nil || balance != 600 {
		t.Fatalf("Expected balance 600 in tx, got %d, err: %v", balance, err)
	}

	// Check balance outside the transaction. It should be unchanged.
	balance, err = ledger.GetBalance("", "alice")
	if err != nil || balance != 500 {
		t.Fatalf("Expected balance 500 outside tx, got %d, err: %v", balance, err)
	}

	// Commit the transaction.
	if err := ledger.Commit(tx1); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// After commit, the main balance should be updated.
	balance, err = ledger.GetBalance("", "alice")
	if err != nil || balance != 600 {
		t.Fatalf("Expected final balance 600 after commit, got %d, err: %v", balance, err)
	}
}

// TestTransaction_Rollback verifies that changes are correctly discarded after a rollback.
func TestTransaction_Rollback(t *testing.T) {
	ledger := NewTransactionalLedger()
	// Set up initial balance for Bob.
	if err := ledger.Deposit("", "bob", 300); err != nil {
		t.Fatalf("Initial deposit failed: %v", err)
	}

	// Start a transaction and withdraw funds.
	tx1 := ledger.Begin()
	if err := ledger.Withdraw(tx1, "bob", 100); err != nil {
		t.Fatalf("Withdraw within tx failed: %v", err)
	}

	// Rollback the transaction.
	if err := ledger.Rollback(tx1); err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	// After rollback, the balance should be restored to its original state.
	balance, err := ledger.GetBalance("", "bob")
	if err != nil || balance != 300 {
		t.Fatalf("Expected balance to be restored to 300, got %d", balance)
	}

	// Trying to use the rolled-back transaction should fail.
	if err := ledger.Commit(tx1); err != ErrTransactionNotFound {
		t.Fatalf("Expected ErrTransactionNotFound after rollback, got %v", err)
	}
}

// TestTransaction_Isolation verifies that transactions are isolated from each other.
// With a cumulative (delta-based) commit, the final result will be different.
func TestTransaction_Isolation(t *testing.T) {
	ledger := NewTransactionalLedger()
	// Set up initial balance for Charlie.
	if err := ledger.Deposit("", "charlie", 1000); err != nil {
		t.Fatalf("Initial deposit failed: %v", err)
	}

	// Start two concurrent transactions.
	tx1 := ledger.Begin()
	tx2 := ledger.Begin()

	// Perform withdrawals in each transaction.
	_ = ledger.Withdraw(tx1, "charlie", 100) // tx1's view of balance becomes 900
	_ = ledger.Withdraw(tx2, "charlie", 200) // tx2's view of balance becomes 800

	// Check balance within tx1.
	bal1, _ := ledger.GetBalance(tx1, "charlie")
	if bal1 != 900 {
		t.Fatalf("tx1 balance should be 900, got %d", bal1)
	}

	// Check balance within tx2.
	bal2, _ := ledger.GetBalance(tx2, "charlie")
	if bal2 != 800 {
		t.Fatalf("tx2 balance should be 800, got %d", bal2)
	}

	// Commit tx1. Main balance should now be 900.
	if err := ledger.Commit(tx1); err != nil {
		t.Fatalf("Commit tx1 failed: %v", err)
	}
	mainBal, _ := ledger.GetBalance("", "charlie")
	if mainBal != 900 {
		t.Fatalf("Main balance should be 900 after tx1 commit, got %d", mainBal)
	}

	// Commit tx2. This will apply its delta (-200) to the new main balance (900).
	if err := ledger.Commit(tx2); err != nil {
		t.Fatalf("Commit tx2 failed: %v", err)
	}

	// The final balance should be 700 (1000 - 100 - 200).
	finalBalance, _ := ledger.GetBalance("", "charlie")
	if finalBalance != 700 {
		t.Fatalf("Expected final balance 700, got %d", finalBalance)
	}
}

// TestTransaction_Concurrency ensures that the ledger handles concurrent commits correctly.
func TestTransaction_Concurrency(t *testing.T) {
	ledger := NewTransactionalLedger()
	_ = ledger.Deposit("", "concurrent_user", 0)

	var wg sync.WaitGroup
	numTransactions := 100

	// Run multiple transactions concurrently, each depositing 10.
	for i := 0; i < numTransactions; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tx := ledger.Begin()
			_ = ledger.Deposit(tx, "concurrent_user", 10)
			// In a real high-contention scenario, commits might fail and need retries.
			// For this test, we assume they will eventually succeed.
			_ = ledger.Commit(tx)
		}()
	}

	wg.Wait()

	finalBalance, err := ledger.GetBalance("", "concurrent_user")
	if err != nil {
		t.Fatalf("Error getting final balance: %v", err)
	}

	// Expected balance is 100 transactions * 10 deposit each.
	if finalBalance != 1000 {
		t.Fatalf("Expected final balance of 1000, got %d", finalBalance)
	}
}
