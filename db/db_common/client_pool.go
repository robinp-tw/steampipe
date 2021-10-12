package db_common

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"golang.org/x/sync/semaphore"
)

var ErrPoolClosing = errors.New("pool is closing")
var ErrCannotReturnToPool = errors.New("cannot return to pool - was not released from pool")

type ClientPoolFactoryFunction = func(ctx context.Context) (*Client, error)
type ClientPoolCloseHook = func(ctx context.Context, obj *Client) error

type ClientPoolConfig struct {
	Capacity int
	Factory  ClientPoolFactoryFunction
	OnClose  ClientPoolCloseHook
}

type ClientPool struct {
	Capacity int
	Factory  ClientPoolFactoryFunction
	OnClose  ClientPoolCloseHook

	closing bool
	// returnChannel chan struct{}

	idle        []*Client
	active      []*Client
	borrowMutex *sync.Mutex
	returnMutex *sync.Mutex

	sema *semaphore.Weighted
}

func NewClientPool(config ClientPoolConfig) *ClientPool {
	pool := &ClientPool{
		Factory:  config.Factory,
		Capacity: config.Capacity,
		OnClose:  config.OnClose,

		closing: false,
		// returnChannel: make(chan struct{}),

		idle:        []*Client{},
		active:      []*Client{},
		borrowMutex: new(sync.Mutex),
		returnMutex: new(sync.Mutex),

		sema: semaphore.NewWeighted(int64(config.Capacity)),
	}

	return pool
}

func (pool *ClientPool) Borrow(ctx context.Context) (*Client, error) {
	if pool.closing {
		return nil, ErrPoolClosing
	}

	fmt.Println("**** borrowing")
	defer fmt.Println("**** borrowing end")

	fmt.Println("idle", pool.idle)
	fmt.Println("active", pool.active)
	fmt.Println("capacity", pool.Capacity)

	pool.borrowMutex.Lock()
	defer pool.borrowMutex.Unlock()

	if err := pool.sema.Acquire(ctx, 1); err != nil {
		return nil, err
	}

	if len(pool.idle) == 0 /* no idle */ {
		if pool.canCreate() {
			fmt.Println("invoking factory")
			element, err := pool.Factory(ctx)
			if err != nil {
				return nil, err
			}
			pool.idle = append(pool.idle, element)
			fmt.Println("invoked")
		}
	}

	fmt.Println("idle", pool.idle)
	fmt.Println("active", pool.active)
	fmt.Println("capacity", pool.Capacity)

	var element *Client
	pool.idle, pool.active, element = leftPopRightPush(pool.idle, pool.active)

	return element, nil
}

func (pool *ClientPool) canCreate() bool {
	return (len(pool.idle) + len(pool.active)) < pool.Capacity
}

func (pool *ClientPool) Return(ctx context.Context, client *Client) error {
	pool.returnMutex.Lock()
	defer pool.returnMutex.Unlock()

	fmt.Println("**** Return")
	defer fmt.Println("**** Return end")

	idx := indexOf(pool.active, client)
	if idx == -1 {
		return ErrCannotReturnToPool
	}

	// remove from the active list
	copy(pool.active[idx:], pool.active[idx+1:])   // Shift a[i+1:] left one index.
	pool.active[len(pool.active)-1] = nil          // Erase last element (write zero value).
	pool.active = pool.active[:len(pool.active)-1] // Truncate slice.

	// put it back in the idle list
	pool.idle = append(pool.idle, client)

	// release the semaphore acquired for this client
	pool.sema.Release(1)
	return nil
}

func (pool *ClientPool) Close(ctx context.Context) error {
	pool.closing = true

	pool.borrowMutex.Lock()
	defer pool.borrowMutex.Unlock()

	// try to acquire all semaphores.
	// this ensures that all clients have been returned
	pool.sema.Acquire(ctx, int64(pool.Capacity))

	for _, element := range pool.idle {
		if pool.OnClose != nil {
			pool.OnClose(ctx, element)
		}
	}

	pool.active = []*Client{}
	pool.idle = []*Client{}

	return nil
}

func indexOf(slice []*Client, cl *Client) int {
	for idx, c := range slice {
		if c == cl {
			return idx
		}
	}
	return -1
}

func leftPopRightPush(source []*Client, destination []*Client) ([]*Client, []*Client, *Client) {
	if len(source) == 0 {
		return source, destination, nil
	}
	element := source[0]
	source = source[1:]
	destination = append(destination, element)

	return source, destination, element
}
