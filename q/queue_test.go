package q

import (
	"fmt"
	"github.com/smallnest/queue"
	"go.uber.org/atomic"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"
)

/*
 @File : queue_test.go
 @Description: simple test to kinds of queue
 @Author : gxl
 @Time : 2021/3/29 10:44
 @Update:
*/

// go test -v -race -run TestLFQueue
func TestLFQueue(t *testing.T) {
	qq := NewQueue(1000)

	wg := &sync.WaitGroup{}
	put := atomic.NewUint32(0)
	get := atomic.NewUint32(0)
	putFailed := atomic.NewUint32(0)
	getFailed := atomic.NewUint32(0)

	var threadCount = 10
	var workPerThread = 1000
	for i := 0; i < threadCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			s := "id:" + strconv.Itoa(id)
			if id % 2 == 0 {
				baseWork := id * workPerThread
				for j := 0; j < workPerThread; {
					s1 := s + ": work" + strconv.Itoa(j + baseWork)
					ok, cnt := qq.Put(s1)
					if ok {
						put.Inc()
						fmt.Printf("Enqueue: %s ok, cnt %v\n", s1, cnt)
						j++
					} else {
						putFailed.Inc()
						time.Sleep(time.Millisecond)
					}
				}

			} else {
				var wait = 0
				for j := 0; j < workPerThread;  {
					s1, ok, cnt := qq.Get()
					if ok {
						get.Inc()
						fmt.Printf("Dequeue: %s msg:%s\n", s, s1.(string))
						wait = 0
						j++
					} else {
						getFailed.Inc()
						fmt.Printf("%v get failed, cnt: %v\n", s, cnt)
						time.Sleep(time.Millisecond * 2)
						wait++
						if wait > 5 {
							j++
						}
					}
				}
			}
		}(i)
	}

	wg.Wait()
	fmt.Printf("put %v, and get %v\n", put.Load(), get.Load())
	fmt.Printf("put failed %v, and get failed %v\n", putFailed.Load(), getFailed.Load())
}


func TestLKQueue(t *testing.T) {
	qq := queue.NewLKQueue()

	wg := &sync.WaitGroup{}
	var threadCount = 10
	var workPerThread = 100

	for i := 0; i < threadCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			s := "id:" + strconv.Itoa(id)
			if id % 2 == 0 {
				for j := 0; j < workPerThread; j++ {
					s1 := s + ": child" + strconv.Itoa(j)
					qq.Enqueue(s1)
					fmt.Printf("Enqueue: %s\n", s1)
					runtime.Gosched()
				}

			} else {
				//time.Sleep(time.Millisecond * 2)
				for j := 0; j < workPerThread;  {
					s1 := qq.Dequeue()
					if s1 != nil {
						fmt.Printf("Dequeue: %s msg:%s\n", s, s1.(string))
						j++
					}
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestQueue(t *testing.T) {
	queues := map[string]queue.Queue{
		"lock-free queue":   queue.NewLKQueue(),
		"two-lock queue":    queue.NewCQueue(),
		"slice-based queue": queue.NewSliceQueue(0),
		"bounded queue":     queue.NewBoundedQueue(100),
	}

	for name, q := range queues {
		t.Run(name, func(t *testing.T) {
			count := 100
			for i := 0; i < count; i++ {
				q.Enqueue(i)
			}

			for i := 0; i < count; i++ {
				v := q.Dequeue()
				if v == nil {
					t.Fatalf("got a nil value")
				}
				if v.(int) != i {
					t.Fatalf("expect %d but got %v", i, v)
				}
			}
		})
	}

}
