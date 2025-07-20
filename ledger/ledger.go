package ledger

import (
	"errors"
	"sort"
	"sync"

	"github.com/google/uuid"
)

// Pre-defined errors
var (
	ErrAccountNotFound     = errors.New("account not found")
	ErrInsufficientFunds   = errors.New("insufficient funds")
	ErrInvalidAmount       = errors.New("amount must be positive")
	ErrTransactionNotFound = errors.New("transaction not found")
)

// AccountID represents a unique identifier for an account.
type AccountID string
type TransactionID string


type TransactionalLedger interface {
	Begin() TransactionID
	Commit(txID TransactionID) error
	Rollback(txID TransactionID) error
	Deposit(txID TransactionID, accountID AccountID, amount uint) error
	Withdraw(txID TransactionID, accountID AccountID, amount uint) error
	GetBalance(txID TransactionID, accountID AccountID) (uint, error)
}

type userAccount struct {
	mu     sync.RWMutex
	amount uint
}

type transaction struct {
	pendingChanges map[AccountID]int
}

type myLedger struct {
	mu                  sync.RWMutex
	accounts            map[AccountID]*userAccount
	pendingTransactions map[TransactionID]*transaction
}

func NewTransactionalLedger() TransactionalLedger {
	return &myLedger{
		mu:                  sync.RWMutex{},
		accounts:            make(map[AccountID]*userAccount),
		pendingTransactions: make(map[TransactionID]*transaction),
	}
}

func (l *myLedger) Begin() TransactionID {
	txID := TransactionID(uuid.NewString())
	transaction := &transaction{
		pendingChanges: make(map[AccountID]int),
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pendingTransactions[txID] = transaction
	return txID
}

func (l *myLedger) getTransaction(txID TransactionID) (*transaction, error) {
	l.mu.RLock()
	transaction, exists := l.pendingTransactions[txID]
	l.mu.RUnlock()
	if !exists {
		return nil, ErrTransactionNotFound
	}
	return transaction, nil
}

func (l *myLedger) getAccount(accountID AccountID) (*userAccount, error) {
	l.mu.RLock()
	account, exists := l.accounts[accountID]
	l.mu.RUnlock()
	if !exists {
		return nil, ErrAccountNotFound
	}
	return account, nil
}

func (l *myLedger) getCommittedBalance(accountID AccountID) (uint, error) {
	account, err := l.getAccount(accountID)
	if err != nil {
		return 0, err
	}
	account.mu.RLock()
	defer account.mu.RUnlock()
	return account.amount, nil
}

func (l *myLedger) GetBalance(txID TransactionID, accountID AccountID) (amount uint, err error) {
	amount, err = l.getCommittedBalance(accountID)
	if txID == "" {
		return
	}
	transaction, err := l.getTransaction(txID)
	if err != nil && err != ErrAccountNotFound {
		return 0, err
	}
	if delta, ok := transaction.pendingChanges[accountID]; ok {
		if int(amount) + delta < 0 {
			return 0, ErrInsufficientFunds
		}
		return uint(int(amount) + delta), nil
	}
	return
}

func (l *myLedger) depositWithoutTransaction(accountID AccountID, amount uint) error {
	if amount == 0 {
		return ErrInvalidAmount
	}

	l.mu.Lock()
	account, exists := l.accounts[accountID]
	if !exists {
		// create the account for deposit
		l.accounts[accountID] = &userAccount{
			amount: amount,
		}
	}
	l.mu.Unlock()
	if exists {
		account.mu.Lock()
		account.amount += amount
		account.mu.Unlock()
	}
	return nil
}

func (l *myLedger) delta(txID TransactionID, accountID AccountID, amount int) error {
	transaction, err := l.getTransaction(txID)
	if err != nil {
		return err
	}
	if amount < 0 {
		currentBalance, err := l.GetBalance(txID, accountID)
		if err != nil && err != ErrAccountNotFound {
			return err
		}
		if int(currentBalance) + transaction.pendingChanges[accountID] + amount < 0 {
			return ErrInsufficientFunds
		}
	}
	transaction.pendingChanges[accountID] += amount
	return nil
}

func (l *myLedger) Deposit(txID TransactionID, accountID AccountID, amount uint) error {
	if amount == 0 {
		return ErrInvalidAmount
	}
	if txID == "" {
		return l.depositWithoutTransaction(accountID, amount)
	}
	return l.delta(txID, accountID, int(amount))
}

func (l *myLedger) withdrawWithoutTransaction(accountID AccountID, amount uint) error {
	if amount == 0 {
		return ErrInvalidAmount
	}
	account, err := l.getAccount(accountID)
	if err != nil {
		return err
	}
	account.mu.Lock()
	defer account.mu.Unlock()
	if account.amount < amount {
		return ErrInsufficientFunds
	}
	account.amount -= amount
	return nil
}

func (l *myLedger) Withdraw(txID TransactionID, accountID AccountID, amount uint) error {
	if amount == 0 {
		return ErrInvalidAmount
	}
	if txID == "" {
		return l.withdrawWithoutTransaction(accountID, amount)
	}
	return l.delta(txID, accountID, -int(amount))
}

func (l *myLedger) Commit(txID TransactionID) error {
	transaction, err := l.getTransaction(txID)
	if err != nil {
		return err
	}

	affectedIDs := []string{}
	for id := range transaction.pendingChanges {
		affectedIDs = append(affectedIDs, string(id))
	}
	sort.Strings(affectedIDs)
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, id := range affectedIDs {
		account, exists := l.accounts[AccountID(id)]
		if !exists {
			l.accounts[AccountID(id)] = &userAccount{amount: uint(transaction.pendingChanges[AccountID(id)])}
		} else {
			after := int(account.amount) + transaction.pendingChanges[AccountID(id)]
			if after < 0 {
				return ErrInsufficientFunds
			}
			account.amount = uint(after)
		}
	}

	delete(l.pendingTransactions, txID)
	return nil
}

func (l *myLedger) Rollback(txID TransactionID) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, ok := l.pendingTransactions[txID]
	if !ok {
		return ErrTransactionNotFound
	}
	delete(l.pendingTransactions, txID)
	return nil
}