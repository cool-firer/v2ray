package pipe

import (
	"errors"
	"io"
	"runtime"
	"sync"
	"time"
	"fmt"

	"v2ray.com/core/common"
	"v2ray.com/core/common/buf"
	"v2ray.com/core/common/signal"
	"v2ray.com/core/common/signal/done"
)

type state byte

const (
	open state = iota
	closed
	errord
)

type pipeOption struct {
	limit           int32 // maximum buffer size in bytes
	discardOverflow bool
}

func (o *pipeOption) isFull(curSize int32) bool {
	return o.limit >= 0 && curSize > o.limit
}

type pipe struct {
	sync.Mutex
	data        buf.MultiBuffer
	readSignal  *signal.Notifier
	writeSignal *signal.Notifier
	done        *done.Instance
	option      pipeOption
	state       state
}

var errBufferFull = errors.New("buffer full")
var errSlowDown = errors.New("slow down")

func (p *pipe) getState(forRead bool) error {
	fmt.Println("getStatus open111 forRead:", forRead)

	switch p.state {
		// 默认
	case open:
		fmt.Println("getStatus open111 forRead:", forRead)
		if !forRead && p.option.isFull(p.data.Len()) {
			return errBufferFull
		}
		fmt.Println("getStatus open222 forRead:", forRead)
		return nil
	case closed:
		fmt.Println("getStatus closed111 forRead:", forRead)
		if !forRead {
			fmt.Println("getStatus closed222 ErrClosedPipe forRead:", forRead)
			return io.ErrClosedPipe
		}
		if !p.data.IsEmpty() {
			fmt.Println("getStatus closed222 nil forRead:", forRead)
			return nil
		}
		fmt.Println("getStatus closed222 io.EOF forRead:", forRead)
		return io.EOF
	case errord:
		fmt.Println("getStatus errord forRead:", forRead)
		return io.ErrClosedPipe
	default:
		panic("impossible case")
	}
}

func (p *pipe) readMultiBufferInternal() (buf.MultiBuffer, error) {
	p.Lock()
	defer p.Unlock()

	if err := p.getState(true); err != nil {
		return nil, err
	}

	data := p.data
	p.data = nil
	return data, nil
}

func (p *pipe) ReadMultiBuffer() (buf.MultiBuffer, error) {
	for {
		data, err := p.readMultiBufferInternal()
		if data != nil || err != nil {
			p.writeSignal.Signal() // 这里是不有问题, 没有释放readSignal.Wait()
			return data, err
		}

		// 阻塞在这里
		select {
		case <-p.readSignal.Wait(): // 1缓冲
		case <-p.done.Wait(): // 0缓冲
		}
	}
}

func (p *pipe) ReadMultiBufferTimeout(d time.Duration) (buf.MultiBuffer, error) {
	timer := time.NewTimer(d)
	defer timer.Stop()

	for {
		data, err := p.readMultiBufferInternal()
		if data != nil || err != nil {
			p.writeSignal.Signal()
			return data, err
		}

		select {
		case <-p.readSignal.Wait():
		case <-p.done.Wait():
		case <-timer.C:
			return nil, buf.ErrReadTimeout
		}
	}
}

func (p *pipe) writeMultiBufferInternal(mb buf.MultiBuffer) error {
	p.Lock()
	defer p.Unlock()

	if err := p.getState(false); err != nil {
		return err
	}

	// 最开始是nil
	if p.data == nil {
		p.data = mb
		return nil
	}

	p.data, _ = buf.MergeMulti(p.data, mb)
	return errSlowDown
}

func (p *pipe) WriteMultiBuffer(mb buf.MultiBuffer) error {
	if mb.IsEmpty() {
		return nil
	}

	for {
		err := p.writeMultiBufferInternal(mb)

		fmt.Println("WriteMultiBuffer returned:", err, " p:", p)

		if err == nil {
			// 最开始是是返回了nil
			p.readSignal.Signal()
			return nil
		}

		if err == errSlowDown {
			fmt.Println("WriteMultiBuffer slow down")
			p.readSignal.Signal()

			// Yield current goroutine. Hopefully the reading counterpart can pick up the payload.
			runtime.Gosched()
			return nil
		}

		if err == errBufferFull && p.option.discardOverflow {
			buf.ReleaseMulti(mb)
			return nil
		}

		if err != errBufferFull {
			buf.ReleaseMulti(mb)
			p.readSignal.Signal()
			return err
		}

		select {
		case <-p.writeSignal.Wait():
		case <-p.done.Wait():
			return io.ErrClosedPipe
		}
	}
}

func (p *pipe) Close() error {
	p.Lock()
	defer p.Unlock()

	if p.state == closed || p.state == errord {
		return nil
	}
	p.state = closed
	common.Must(p.done.Close())
	fmt.Println("fuck i m closed:", p)
	return nil
}

// Interrupt implements common.Interruptible.
func (p *pipe) Interrupt() {
	p.Lock()
	defer p.Unlock()

	if p.state == closed || p.state == errord {
		return
	}

	p.state = errord

	if !p.data.IsEmpty() {
		buf.ReleaseMulti(p.data)
		p.data = nil
	}

	common.Must(p.done.Close())
}
