package contextimpl

import "errors"
import "reflect"
import "sync"
import "time"

// Context is self-implement context interface
type Context interface {
	Deadline() (deadline time.Time, ok bool)
	Done() <-chan struct{}
	Err() error
	Value(key interface{}) interface{}
}

type emptyCtx int

func (emptyCtx) Deadline() (deadline time.Time, ok bool) { return }
func (emptyCtx) Done() <-chan struct{}                   { return nil }
func (emptyCtx) Err() error                              { return nil }
func (emptyCtx) Value(key interface{}) interface{}       { return nil }

// Variables
var (
	background       = new(emptyCtx)
	todo             = new(emptyCtx)
	Canceled         = errors.New("Context canceled")
	DeadlineExceeded = deadlineExceededErr{}
)

type deadlineExceededErr struct{}

func (deadlineExceededErr) Error() string   { return "Deadline exceeded" }
func (deadlineExceededErr) Timeout() bool   { return true }
func (deadlineExceededErr) Temporary() bool { return true }

// Background context
func Background() Context { return background }

// TODO context
func TODO() Context { return todo }

type cancelCtx struct {
	Context
	mu   sync.RWMutex
	done chan struct{}
	err  error
}

func (ctx *cancelCtx) Done() <-chan struct{} { return ctx.done }
func (ctx *cancelCtx) Err() error {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	return ctx.err
}
func (ctx *cancelCtx) Value(key interface{}) interface{} {
	return ctx.Value(key)
}

func (ctx *cancelCtx) cancel(err error) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	if ctx.err != nil {
		return
	}
	ctx.err = err
	close(ctx.done)
}

// CancelFunc type alias
type CancelFunc func()

// WithCancel return context associated with cascaded cancel function
func WithCancel(parent Context) (Context, CancelFunc) {
	ctx := &cancelCtx{
		Context: parent,
		done:    make(chan struct{}),
	}

	cancel := func() { ctx.cancel(Canceled) }

	go func() {
		select {
		case <-parent.Done():
			ctx.cancel(parent.Err())
		case <-ctx.Done():
		}
	}()

	return ctx, cancel
}

type deadlineCtx struct {
	*cancelCtx
	deadline time.Time
}

func (ctx *deadlineCtx) Deadline() (deadline time.Time, ok bool) { return ctx.deadline, true }

// WithDeadline return context associated with cascaded and deadline cancel function
func WithDeadline(parent Context, deadline time.Time) (Context, CancelFunc) {
	ctx, cancel := WithCancel(parent)

	dtx := &deadlineCtx{
		cancelCtx: ctx.(*cancelCtx),
		deadline:  deadline,
	}

	t := time.AfterFunc(time.Until(deadline), func() {
		dtx.cancel(DeadlineExceeded)
	})

	stop := func() {
		t.Stop()
		cancel()
	}

	return dtx, stop
}

// WithTimeout return context associated with cascaded and timeout cancel function
func WithTimeout(parent Context, timeout time.Duration) (Context, CancelFunc) {
	return WithDeadline(parent, time.Now().Add(timeout))
}

type valueCtx struct {
	Context
	value, key interface{}
}

func (ctx valueCtx) Value(key interface{}) interface{} {
	if key == ctx.key {
		return ctx.value
	}
	return ctx.Context.Value(key)
}

// WithValue return context associated with key-value pair
func WithValue(parent Context, key, value interface{}) Context {
	if key == nil {
		panic("Key is nil")
	}
	if !reflect.TypeOf(key).Comparable() {
		panic("Key is not Comparable")
	}
	return &valueCtx{
		Context: parent,
		key:     key,
		value:   value,
	}
}
