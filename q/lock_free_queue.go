package q

/*
 @File : lock_free_queue.go
 @Description: an bounded lock free queue use slice as circle queue
               clone from https://github.com/yireyun/go-queue & and changed
 @Author : gxl
 @Time : 2021/3/29 14:16
 @Update:
*/

import (
	"fmt"
	"go.uber.org/atomic"
	"runtime"
)

const (
	MinCap = 8
	MaxWait = 100
)

type IQueue interface {
	Info() string
	Capacity()  uint32
	Count()     uint32
	Put(val interface{}) (ok bool, count uint32)
	Get() (val interface{}, ok bool, count uint32)

	Gets(values []interface{}) (gets, count uint32)
	Puts(values []interface{}) (puts, count uint32)
}

// slot of queue, each slot has a putID and getID
// when new value put, putID will increase by cap, and that mean's it's has value
// only getID + cap == putID, can get value from this slot, then getID increase by cap
// when getID == putID, this slot is empty
type slot struct {
	putID    *atomic.Uint32
	getID    *atomic.Uint32
	value    interface{}
}

// LFQueue An bounded lock free Queue
type LFQueue struct {
	capacity    uint32  // const after init, always 2's power
	capMod      uint32  // cap - 1, const after init
	putPos      *atomic.Uint32
	getPos      *atomic.Uint32
	carrier     []slot
}

func NewQueue(cap uint32) *LFQueue {
	if cap < 1 {
		cap = MinCap
	}
	q := new(LFQueue)
	q.capacity = minRoundNumBy2(cap)
	q.capMod = q.capacity - 1
	q.putPos = atomic.NewUint32(0)
	q.getPos = atomic.NewUint32(0)
	q.carrier = make([]slot, q.capacity)

	// putID/getID 提前分配好，每用一个，delta增加一轮
	tmp := &q.carrier[0]
	tmp.putID = atomic.NewUint32(q.capacity)
	tmp.getID = atomic.NewUint32(q.capacity)

	var i uint32 = 1
	for ; i < q.capacity; i++ {
		tmp = &q.carrier[i]
		tmp.getID = atomic.NewUint32(i)
		tmp.putID = atomic.NewUint32(i)
	}

	return q
}

func (q *LFQueue) Info() string {
	return fmt.Sprintf("Queue{capacity: %v, capMod: %v, putPos: %v, getPos: %v}",
		q.capacity, q.capMod, q.putPos.Load(), q.getPos.Load())
}

func (q *LFQueue) Capacity() uint32 {
	return q.capacity
}


func (q *LFQueue) Count() uint32 {
	getPos := q.getPos.Load()
	putPos := q.putPos.Load()
	return q.posCount(getPos, putPos)
}

// Put May failed if lock slot failed or full
// caller should retry if failed
// should not put nil for normal logic
func (q *LFQueue) Put(val interface{}) (ok bool, count uint32) {
	getPos := q.getPos.Load()
	putPos := q.putPos.Load()

	cnt := q.posCount(getPos, putPos)
	// 如果满了，就直接失败
	if cnt >= q.capMod - 1 {
		runtime.Gosched()
		return false, cnt
	}

	// 先占一个坑，如果占坑失败，就直接返回
	posNext := putPos + 1
	if !q.putPos.CAS(putPos, posNext) {
		runtime.Gosched()
		return false, cnt
	}

	var cache *slot = &q.carrier[posNext & q.capMod]
	var waitCounter = 0
	for {
		getID := cache.getID.Load()
		putID := cache.putID.Load()
		if posNext == putID && getID == putID {
			cache.value = val
			cache.putID.Add(q.capacity)
			return true, cnt + 1
		} else {
			// 存线程的竞争过多，而队列cap过小，前面的如果写数据动作比较慢，而后来的进程已经lock到这个位置的下一轮了
			// 此时，这个位置等于已经被他预约了，但是数据还没取走，需要等待下次get了数据之后，才能重新put
			// 所以就先让出cpu，等待下次调度
			// 为啥不直接返回？ 因为位置已经占了，其他线程不会占用这个地方了
			// fmt.Printf("put too quick: getID %v, putID %v and putPosNext: %v\n", getID, putID, posNext)
			waitCounter++
			if waitCounter > MaxWait {
				// 实在put不进去，一直没有消费, 那就扔一条吧, 这里主要是防止调用进程死等, 理论上极小概率到这里
				val, ok, cnt := q.Get()
				if ok {
					fmt.Printf("throw val: %v away, cnt: %v", val, cnt)
					continue
				}
			}
			runtime.Gosched()
		}

	}
}

// Get May failed if lock slot failed or empty
// caller should retry if failed
func (q *LFQueue) Get() (val interface{}, ok bool, count uint32) {
	getPos := q.getPos.Load()
	putPos := q.putPos.Load()

	cnt := q.posCount(getPos, putPos)
	if cnt < 1 {
		runtime.Gosched()
		return nil, false, cnt
	}

	getPosNext := getPos + 1
	if !q.getPos.CAS(getPos, getPosNext) {
		runtime.Gosched()
		return nil, false, cnt
	}

	cache := &q.carrier[getPosNext & q.capMod]

	var waitCounter = 0
	for {
		getID := cache.getID.Load()
		putID := cache.putID.Load()
		if getPosNext == getID && (getID + q.capacity == putID) {
			val = cache.value
			cache.value = nil
			cache.getID.Add(q.capacity)
			return val, true, cnt - 1
		} else {
			// 可能是取的竞争过多，而队列cap过小，前面的如果取数据动作比较慢，而后来的进程已经取到这个位置的下一轮了
			// 此时，这个位置等于已经被他预约了，但是却没数据，需要等待下次put了数据之后，才能重新取到
			// 所以就先让出cpu，等待下次调度
			// fmt.Printf("get too quick: getID %v, putID %v and getPosNext: %v\n", getID, putID, getPosNext)
			waitCounter++
			if waitCounter > MaxWait {
				// 实在get不到，一直没有put, 那就put一个假数据吧, 这里主要是防止调用进程死等, 理论上极小概率到这里
				ok, _ := q.Put(nil)
				if ok {
					fmt.Printf("put nil to escape\n")
					continue
				}
			}
			runtime.Gosched()
		}
	}
}

func (q *LFQueue) Gets(values []interface{}) (gets, count uint32) {
	getPos := q.getPos.Load()
	putPos := q.putPos.Load()

	cnt := q.posCount(getPos, putPos)
	if cnt < 1 {
		runtime.Gosched()
		return 0, cnt
	}

	var getCnt uint32
	if size := uint32(len(values)); cnt >= size {
		getCnt = size
	} else {
		getCnt = cnt
	}
	getPosNew := getPos + getCnt

	if !q.getPos.CAS(getPos, getPosNew) {
		runtime.Gosched()
		return 0, cnt
	}

	for posNew, v := getPos+1, uint32(0); v < getCnt; posNew, v = posNew+1, v+1 {
		var cache *slot = &q.carrier[posNew & q.capMod]
		for {
			if q.canGet(posNew, cache) {
				values[v] = cache.value
				cache.value = nil
				cache.getID.Add(q.capacity)
				break
			} else {
				runtime.Gosched()
			}
		}
	}
	return getCnt, cnt - getCnt
}

func (q *LFQueue) Puts(values []interface{}) (puts, count uint32) {
	getPos := q.getPos.Load()
	putPos := q.putPos.Load()

	cnt := q.posCount(getPos, putPos)
	if cnt >= q.capMod -1 {
		runtime.Gosched()
		return 0, cnt
	}

	var putCnt uint32
	if capPuts, size := q.capacity - cnt, uint32(len(values)); capPuts >= size {
		putCnt = size
	} else {
		putCnt = capPuts
	}
	putPosNew := putPos + putCnt
	if !q.putPos.CAS(putPos, putPosNew) {
		runtime.Gosched()
		return 0, cnt
	}

	for posNew, v := putPos+1, uint32(0); v < putCnt; posNew, v = posNew+1, v+1 {
		var cache *slot = &q.carrier[posNew & q.capMod]
		for {
			if q.canPut(posNew, cache) {
				cache.value = values[v]
				cache.putID.Add(q.capacity)
				break
			} else {
				runtime.Gosched()
			}
		}
	}

	return putCnt, cnt + putCnt
}

func (q *LFQueue) canPut(posNew uint32, cache *slot) bool {
	getID := cache.getID.Load()
	putID := cache.putID.Load()

	// putID == getID 才代表此处为空，可以写新数据
	return  posNew == putID && getID == putID
}

func (q *LFQueue) canGet(getPosNew uint32, cache *slot) bool {
	getID := cache.getID.Load()
	putID := cache.putID.Load()
	return getPosNew == getID && (getID + q.capacity == putID)
}


func (q *LFQueue) isFull() bool {
	return q.Count() >= q.capMod - 1
}

func (q *LFQueue) posCount(getPos, putPos uint32) uint32 {
	if putPos >= getPos {
		return putPos - getPos
	}
	return q.capMod - getPos + putPos
}


// minRoundNumBy2 round 到 >=N的 最近的2的倍数，
// eg f(3) = 4
func minRoundNumBy2(v uint32) uint32 {
	v--
	v |= v >> 1
	v |= v >> 2
	v |= v >> 4
	v |= v >> 8
	v |= v >> 16
	v++
	return v
}









