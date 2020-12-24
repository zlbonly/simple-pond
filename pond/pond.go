package pond

import (
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// ErrInvalidPoolCap return if pool size <= 0
	ErrInvalidPoolCap = errors.New("invalid pool cap")
	// ErrPoolAlreadyClosed put task but pool already closed
	ErrPoolAlreadyClosed = errors.New("pool already closed")
)

const (
	// RUNNING pool is running
	RUNNING = 1
	// STOPED pool is stoped
	STOPED = 0
)

// Task task to-do
type Task struct {
	Handler func(v ...interface{})
	Params  []interface{}
}

// Pool task pool
type Pool struct {
	capacity       uint64
	runningWorkers uint64
	state          int64
	taskC          chan *Task
	PanicHandler   func(interface{})
	sync.Mutex
}

// NewPool init pool
func NewPool(capacity uint64) (*Pool, error) {
	if capacity <= 0 {
		return nil, ErrInvalidPoolCap
	}
	return &Pool{
		capacity: capacity,
		state:    RUNNING,
		taskC:    make(chan *Task, capacity),
	}, nil
}

// GetCap get capacity
func (p *Pool) GetCap() uint64 {
	return p.capacity
}

// GetRunningWorkers get running workers
func (p *Pool) GetRunningWorkers() uint64 {
	return atomic.LoadUint64(&p.runningWorkers)
}

func (p *Pool) incRunning() {
	atomic.AddUint64(&p.runningWorkers, 1)
}

func (p *Pool) decRunning() {
	atomic.AddUint64(&p.runningWorkers, ^uint64(0))
}

// Put put a task to pool
func (p *Pool) Put(task *Task) error {

	if p.getState() == STOPED {
		return ErrPoolAlreadyClosed
	}

	// safe run worker
	p.Lock()
	if p.GetRunningWorkers() < p.GetCap() { // 如果任务池满，则不再创建worker
		// 创建启动一个worker
		p.run()
	}
	p.Unlock()

	// send task safe
	p.Lock()
	if p.state == RUNNING {
		p.taskC <- task // 安全的推送任务, 以防在推送任务到 taskC 时 state 改变而关闭了 taskC
	}
	p.Unlock()
	return nil
}

func (p *Pool) run() {
	p.incRunning()

	go func() {
		defer func() {
			p.decRunning() // worker结束，运行中任务减1
			if r := recover(); r != nil {
				if p.PanicHandler != nil {
					p.PanicHandler(r)
				} else {
					log.Printf("Worker panic: %s\n", r)
				}
			}
		}()

		for {
			select { // 阻塞等待任务，结束信号到来
			case task, ok := <-p.taskC: // 从 channel中 消费任务
				if !ok { // 如果channel被关闭，结束worker运行
					return
				}
				// 执行任务
				task.Handler(task.Params...)
			}
		}
	}()
}

func (p *Pool) getState() int64 {
	p.Lock()
	defer p.Unlock()

	return p.state
}

func (p *Pool) setState(state int64) {
	p.Lock()
	defer p.Unlock()

	p.state = state
}

// close safe
func (p *Pool) close() {
	p.Lock()
	defer p.Unlock()

	close(p.taskC)
}

// Close close pool graceful
func (p *Pool) Close() {

	if p.getState() == STOPED {
		return
	}

	p.setState(STOPED) // stop put task

	for len(p.taskC) > 0 { // wait all task be consumed // 阻塞等待所有任务被 worker 消费
		time.Sleep(1e6) // reduce CPU load
	}

	p.close() // // 关闭任务队列
}
