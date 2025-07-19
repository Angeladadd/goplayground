package distributedid

import (
	"fmt"
	"sync"
	"time"
)

// IDGenerator defines the interface for our distributed ID generator.
type IDGenerator interface {
	// NextID generates and returns a new unique ID.
	// It should be safe for concurrent use.
	NextID() (int64, error)
}

const (
	epoch = 1735689600000 
	maxSequence = 4095
	reserveBit = 1
	timestampBit = 41
	workerBit = 10
	sequenceBit = 12

)

type idGenerator struct {
	workerId int64
	mu sync.Mutex
	sequenceId int64
	lastTimestamp int64
}


var globalMutex = sync.Mutex{}
var globalGenerator = make(map[int64]*idGenerator)

func NewSnowflakeGenerator(workerId int64) (IDGenerator, error) {
	if workerId < 0 || workerId >= (1 << workerBit) {
		return nil, fmt.Errorf("invalid workerId")
	}
	globalMutex.Lock()
	defer globalMutex.Unlock()
	if idgen, exists := globalGenerator[workerId]; exists {
		return idgen, nil
	}
	globalGenerator[workerId] = &idGenerator{
		workerId: workerId,
		mu: sync.Mutex{},
		sequenceId: 0,
		lastTimestamp: time.Now().UnixMilli() - epoch,
	}
	return globalGenerator[workerId], nil
}


func (id *idGenerator) NextID() (int64, error) {
	id.mu.Lock()
	defer id.mu.Unlock()
	curTimestamp := time.Now().UnixMilli() - epoch
	if curTimestamp < id.lastTimestamp {
		return 0, fmt.Errorf("clock skew detected")
	}
	id.sequenceId++
	if id.sequenceId > maxSequence || curTimestamp > id.lastTimestamp {
		for curTimestamp == id.lastTimestamp {
			curTimestamp = time.Now().UnixMilli() - epoch
		}
		id.sequenceId = 0
	}
	id.lastTimestamp = curTimestamp
	workerIdShift, timestampShift := sequenceBit, sequenceBit + workerBit
	return (id.workerId << int64(workerIdShift)) | (id.lastTimestamp << int64(timestampShift)) | id.sequenceId, nil
}