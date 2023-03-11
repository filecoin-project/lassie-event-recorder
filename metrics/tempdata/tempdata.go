package tempdata

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/filecoin-project/lassie/pkg/types"
)

const timeout = 1 * time.Minute

type TempData struct {
	timer                           *time.Timer
	lastStage                       uint32
	startTime                       uint64
	indexerCandidates               uint32
	indexerCandidateFirstResultTime uint64
	indexerFiltered                 uint32
	bitswapAttempt                  uint32
	graphsyncAttempt                uint32
	ttfbTime                        uint64
	failedCount                     uint32
}

func (t *TempData) RecordStartTime(startTime time.Time) (first bool) {
	return atomic.CompareAndSwapUint64(&t.startTime, 0, uint64(startTime.UnixMicro()))
}

func (t *TempData) StartTime() time.Time {
	return time.UnixMicro(int64(atomic.LoadUint64(&t.startTime)))
}

func (t *TempData) RecordIndexerCandidates(eventTime time.Time, candidatesFound uint32) (first bool) {
	if atomic.CompareAndSwapUint32(&t.indexerCandidates, 0, candidatesFound) {
		atomic.StoreUint64(&t.indexerCandidateFirstResultTime, uint64(eventTime.UnixMicro()))
		return true
	}
	atomic.AddUint32(&t.indexerCandidates, candidatesFound)
	return false
}

func (t *TempData) RecordIndexerFilteredCandidates(candidatesFoundFiltered uint32) (first bool) {
	if atomic.CompareAndSwapUint32(&t.indexerFiltered, 0, candidatesFoundFiltered) {
		return true
	}
	atomic.AddUint32(&t.indexerFiltered, candidatesFoundFiltered)
	return false
}

func (t *TempData) RecordBitswapAttempt() (first bool) {
	return atomic.CompareAndSwapUint32(&t.bitswapAttempt, 0, 1)
}
func (t *TempData) RecordGraphsyncAttempt() (first bool) {
	return atomic.CompareAndSwapUint32(&t.graphsyncAttempt, 0, 1)
}

func (t *TempData) RecordTimeToFirstByte(ttfbTime time.Time) (first bool) {
	return atomic.CompareAndSwapUint64(&t.ttfbTime, 0, uint64(ttfbTime.UnixMicro()))
}

func (t *TempData) RecordFailure() (first bool) {
	if atomic.CompareAndSwapUint32(&t.failedCount, 0, 1) {
		return true
	}

	atomic.AddUint32(&t.failedCount, 1)
	return false
}

func (t *TempData) RecordFinality() {
	t.timer.Stop()
}

type FinalValues struct {
	StartTime                       time.Time
	IndexerCandidates               uint64
	IndexerCandidateFirstResultTime time.Time
	IndexerFiltered                 uint64
	HasBitswapAttempt               bool
	HasGraphSyncAttempt             bool
	TimeToFirstByte                 time.Time
	FailedCount                     uint64
}

func (t *TempData) finalValues() FinalValues {
	return FinalValues{
		StartTime:                       time.UnixMicro(int64(atomic.LoadUint64(&t.startTime))),
		IndexerCandidates:               uint64(atomic.LoadUint32(&t.indexerCandidates)),
		IndexerCandidateFirstResultTime: time.UnixMicro(int64(atomic.LoadUint64(&t.indexerCandidateFirstResultTime))),
		IndexerFiltered:                 uint64(atomic.LoadUint32(&t.indexerFiltered)),
		HasBitswapAttempt:               atomic.LoadUint32(&t.bitswapAttempt) > 0,
		HasGraphSyncAttempt:             atomic.LoadUint32(&t.graphsyncAttempt) > 0,
		TimeToFirstByte:                 time.UnixMicro(int64(atomic.LoadUint64(&t.ttfbTime))),
		FailedCount:                     uint64(atomic.LoadUint32(&t.failedCount)),
	}
}

func (t *TempData) zeroOut() {
	atomic.StoreUint64(&t.startTime, 0)
	atomic.StoreUint32(&t.indexerCandidates, 0)
	atomic.StoreUint64(&t.indexerCandidateFirstResultTime, 0)
	atomic.StoreUint32(&t.indexerFiltered, 0)
	atomic.StoreUint32(&t.bitswapAttempt, 0)
	atomic.StoreUint32(&t.graphsyncAttempt, 0)
	atomic.StoreUint64(&t.ttfbTime, 0)
	atomic.StoreUint32(&t.failedCount, 0)
}

var tempDataPool = sync.Pool{
	New: func() interface{} {
		return new(TempData)
	},
}

type TempDataMap struct {
	internalMap sync.Map
}

func NewTempDataMap() *TempDataMap {
	return &TempDataMap{}
}

func (t *TempDataMap) GetOrCreate(id types.RetrievalID) *TempData {
	newTempData := tempDataPool.Get().(*TempData)
	actual, loaded := t.internalMap.LoadOrStore(id, newTempData)
	if loaded {
		tempDataPool.Put(newTempData)
	} else {
		actual.(*TempData).timer = time.AfterFunc(timeout, func() {
			t.Delete(id)
		})
	}
	return actual.(*TempData)
}

func (t *TempDataMap) Delete(id types.RetrievalID) *FinalValues {
	value, loaded := t.internalMap.LoadAndDelete(id)
	if loaded {
		tempData := value.(*TempData)
		finalValues := tempData.finalValues()
		tempData.zeroOut()
		tempDataPool.Put(tempData)
		return &finalValues
	}
	return nil
}
