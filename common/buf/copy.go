package buf

import (
	"io"
	"time"
	"fmt"

	"v2ray.com/core/common/errors"
	"v2ray.com/core/common/signal"
)

type dataHandler func(MultiBuffer)

type copyHandler struct {
	onData []dataHandler
	goType string
}

// SizeCounter is for counting bytes copied by Copy().
type SizeCounter struct {
	Size int64
}

// CopyOption is an option for copying data.
type CopyOption func(*copyHandler)

// UpdateActivity is a CopyOption to update activity on each data copy operation.
func UpdateActivity(timer signal.ActivityUpdater) CopyOption {
	return func(handler *copyHandler) {
		handler.onData = append(handler.onData, func(MultiBuffer) {
			timer.Update()
		})
	}
}

func SetGoType(gtype string) CopyOption {
	return func(handler *copyHandler) {
		handler.goType = gtype
	}
}

// CountSize is a CopyOption that sums the total size of data copied into the given SizeCounter.
func CountSize(sc *SizeCounter) CopyOption {
	return func(handler *copyHandler) {
		handler.onData = append(handler.onData, func(b MultiBuffer) {
			sc.Size += int64(b.Len())
		})
	}
}

type readError struct {
	error
}

func (e readError) Error() string {
	return e.error.Error()
}

func (e readError) Inner() error {
	return e.error
}

// IsReadError returns true if the error in Copy() comes from reading.
func IsReadError(err error) bool {
	_, ok := err.(readError)
	return ok
}

type writeError struct {
	error
}

func (e writeError) Error() string {
	return e.error.Error()
}

func (e writeError) Inner() error {
	return e.error
}

// IsWriteError returns true if the error in Copy() comes from writing.
func IsWriteError(err error) bool {
	_, ok := err.(writeError)
	return ok
}

/**
	 	&ReadVReader{
			Reader:  客户端conn的抽象Reader接口,
			rawConn: conn.(syscall.Conn)的转换,
			alloc: allocStrategy{
				current: 1,
			},
			mr: newMultiReader(),
		}

		handler: &copyHandler{
				onData = append(handler.onData, func(MultiBuffer) {
					timer.Update()
				})
		}

*/
func copyInternal(reader Reader, writer Writer, handler *copyHandler) error {
	for {

		// reader动态类: /Users/demon/Desktop/work/gowork/src/v2ray.com/core/common/buf/readv_reader.go
		/**
			1. 会阻塞, 调用原生: reader.Read(b.v[b.end:]), 读进内部[]
			2. 返回 MultiBuffer{b}, 一个 []*Buffer切片
		*/

		// var gotype string = "[freedom]"
		// if _, ok := reader.(*ReadVReader); ok {
		// 	gotype = "[ClientConn]"
		// }

		var gotype string = ""
		gotype = handler.goType
		fmt.Println(gotype, "reader.ReadMultiBuffer...")
		buffer, err := reader.ReadMultiBuffer() // 是会阻塞

		printMultiBuffer(gotype, buffer, err)

		if !buffer.IsEmpty() { // 只要其中有一个不空, 就是false
			for _, handler := range handler.onData {
				// 就只是调用timer.Update() 没了？
				handler(buffer)
			}

			fmt.Println(gotype, "writer.WriteMultiBuffer...")
			if werr := writer.WriteMultiBuffer(buffer); werr != nil {
				fmt.Println(gotype, "WriteMultiBuffer returned with err:", werr)
				return writeError{werr}
			}
			fmt.Println(gotype, "WriteMultiBuffer returned")
		}

		if err != nil {
			return readError{err}
		}
	}
}

func printMultiBuffer(gotype string, mb MultiBuffer, err error) {
	for i := range mb {
		if mb[i] != nil {
			mb[i].SetGtype(gotype)
			bytes := mb[i].Bytes()
			fmt.Println(gotype, "ReadMultiBuffer returned read:", string(bytes), "err:", err)
		} else {
			fmt.Println(gotype, "ReadMultiBuffer returned read:", "err:", err)
		}
	}
}

// Copy dumps all payload from reader to writer or stops when an error occurs. It returns nil when EOF.

/**
	 	&ReadVReader{
			Reader:  客户端conn的抽象Reader接口,
			rawConn: conn.(syscall.Conn)的转换,
			alloc: allocStrategy{
				current: 1,
			},
			mr: newMultiReader(),
		}

	options: 
	return func(handler *copyHandler) {
		handler.onData = append(handler.onData, func(MultiBuffer) {
			timer.Update()
		})
	}


	目标conn
	reader: &Reader {
		pipe: ...
	}

	writer: &BufferToBytesWriter{
				Writer: conn的Writer抽象接口
	}

*/
func Copy(reader Reader, writer Writer, options ...CopyOption) error {
	var handler copyHandler
	for _, option := range options {
		option(&handler) // 追加 onData
	}

	err := copyInternal(reader, writer, &handler)
	if err != nil && errors.Cause(err) != io.EOF { // 这啥意思, EOF不算错误
		return err
	}
	return nil
}

var ErrNotTimeoutReader = newError("not a TimeoutReader")

func CopyOnceTimeout(reader Reader, writer Writer, timeout time.Duration) error {
	timeoutReader, ok := reader.(TimeoutReader)
	if !ok {
		return ErrNotTimeoutReader
	}
	mb, err := timeoutReader.ReadMultiBufferTimeout(timeout)
	if err != nil {
		return err
	}
	return writer.WriteMultiBuffer(mb)
}
