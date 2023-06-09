package fog

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

// LogMux is a log multiplexer.
// It accepts writes for multiple registered log streams and merges the output.
//
// Lines for a given stream are prefixed with the name of the stream and a
// color. Interleaving of streams is minimized as much as possible.
type LogMux struct {
	ctx     context.Context
	w       io.Writer
	wc      chan []byte
	timeout time.Duration
	streams map[string]*LogStream
}

// LogStream is an individual log stream of the multiplexer.
// A stream should not be shared across goroutines.
type LogStream struct {
	name    string
	timeout time.Duration
	// color to use for log entries
	clr colorful.Color
	// log line prefix, possibly with ANSI escape sequences
	prefix string
	mu     sync.Mutex
	// buffer for partial lines
	buf bytes.Buffer
	// channel to send writes
	wc *chan []byte
	// timer to flush partial lines
	t *time.Timer
}

// NewLogMux allocates and returns a new LogMux.
func NewLogMux(ctx context.Context, w io.Writer) *LogMux {
	m := &LogMux{
		ctx:     ctx,
		streams: make(map[string]*LogStream),
		timeout: time.Millisecond * 10,
		w:       w,
		wc:      make(chan []byte),
	}

	go m.flush()

	return m
}

// flush receives writes from log streams and flushes them to the output writer.
func (m *LogMux) flush() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case buf := <-m.wc:
			m.w.Write(buf)
		}
	}
}

// Stream adds an additional log stream to the multiplexer.
// If a stream already exists for name, Stream panics.
func (m *LogMux) Stream(name string) *LogStream {
	_, exists := m.streams[name]

	if exists {
		panic(fmt.Errorf("Stream %s already exists", name))
	}

	s := &LogStream{
		name:    name,
		wc:      &m.wc,
		timeout: m.timeout,
	}

	m.streams[name] = s

	m.refreshColors()

	return s
}

// refresh the colors of the streams to re-distribute them across the color space.
// Expects the mutex to be held already when called.
func (m *LogMux) refreshColors() {
	pal := colorful.FastHappyPalette(len(m.streams))

	maxlen := 0
	for _, v := range m.streams {
		l := len(v.name)
		if l > maxlen {
			maxlen = l
		}
	}

	i := 0
	for _, v := range m.streams {
		v.clr = pal[i]

		style := lipgloss.NewStyle().
			Foreground(lipgloss.Color(v.clr.Hex())).
			BorderStyle(lipgloss.NormalBorder()).
			PaddingRight((maxlen - len(v.name)) + 2).
			MarginRight(2).
			BorderForeground(lipgloss.Color("#3C3C3C")).
			BorderRight(true)

		v.prefix = style.Render(v.name)

		i++
	}
}

// Write implements io.Writer for a log stream.
func (s *LogStream) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.t != nil {
		s.t.Stop()
	}

	n, err = s.buf.Write(p)

	if bytes.ContainsRune(p, '\n') {
		for {
			l, err := s.buf.ReadBytes('\n')

			if err != nil {
				break
			}

			*s.wc <- []byte(s.prefix)
			*s.wc <- l
		}
	} else {
		s.t = time.AfterFunc(s.timeout, func() {
			s.mu.Lock()
			defer s.mu.Unlock()

			l := s.buf.Bytes()

			*s.wc <- []byte(s.prefix)
			*s.wc <- l
			*s.wc <- []byte("\n")
		})
	}

	return n, err
}
