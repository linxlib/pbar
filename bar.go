package pbar

import (
	"bytes"
	"fmt"

	"io"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"text/template"
	"time"

	"github.com/linxlib/pbar/termutil"
)

type key int

const (
	// Bytes means we're working with byte sizes. Numbers will print as Kb, Mb, etc
	// bar.Set(pb.Bytes, true)
	Bytes key = 1 << iota

	// Use SI bytes prefix names (kB, MB, etc) instead of IEC prefix names (KiB, MiB, etc)
	SIBytesPrefix
)

const (
	defaultBarWidth = 100
)

// New creates new Bar object
func NewBar(total int) *Bar {
	return NewBar64(int64(total))
}

// New64 creates new Bar object using int64 as total
func NewBar64(total int64) *Bar {
	pb := new(Bar)
	return pb.SetTotal(total)
}

// StartNew starts new Bar with Default template
func StartNew(total int) *Bar {
	return NewBar(total).Start()
}

// Start64 starts new Bar with Default template. Using int64 as total.
func Start64(total int64) *Bar {
	return NewBar64(total).Start()
}

var (
	terminalWidth = termutil.TerminalWidth
)

// Bar is the main object of bar
type Bar struct {
	current, total int64
	width          int
	maxWidth       int
	mu             sync.RWMutex
	rm             sync.Mutex
	vars           map[interface{}]interface{}
	elements       map[string]Element
	startTime      time.Time
	tmpl           *template.Template
	state          *State
	buf            *bytes.Buffer
	finished       bool
	configured     bool
	err            error
}

func (pb *Bar) configure() {
	if pb.configured {
		return
	}
	pb.configured = true

	if pb.vars == nil {
		pb.vars = make(map[interface{}]interface{})
	}
	if pb.tmpl == nil {
		pb.tmpl, pb.err = getTemplate(string(Building))
		if pb.err != nil {
			return
		}
	}

}

// Start starts the bar
func (pb *Bar) Start() *Bar {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	pb.configure()
	if pb.finished {
		return pb
	}
	pb.finished = false
	pb.state = nil
	pb.startTime = time.Now()
	return pb
}

func (pb *Bar) bytes(finish bool) string {
	result, width := pb.render()
	if pb.Err() != nil {
		return ""
	}
	if r := width - CellCount(result); r > 0 {
		result += strings.Repeat(" ", r)
	}

	result = "\r" + result
	if finish {
		result += "\n"
	}

	return result
}

// Total return current total bar value
func (pb *Bar) Total() int64 {
	return atomic.LoadInt64(&pb.total)
}

// SetTotal sets the total bar value
func (pb *Bar) SetTotal(value int64) *Bar {
	atomic.StoreInt64(&pb.total, value)
	if pb.current >= pb.total {
		pb.Finish()
	}
	return pb
}

// SetCurrent sets the current bar value
func (pb *Bar) SetCurrent(value int64) *Bar {
	atomic.StoreInt64(&pb.current, value)
	if pb.current >= pb.total {
		pb.Finish()
	}
	return pb
}

// Current return current bar value
func (pb *Bar) Current() int64 {
	return atomic.LoadInt64(&pb.current)
}

// Add adding given int64 value to bar value
func (pb *Bar) Add64(value int64) *Bar {
	atomic.AddInt64(&pb.current, value)
	if pb.current >= pb.total {
		pb.Finish()
	}
	return pb
}

// Add adding given int value to bar value
func (pb *Bar) Add(value int) *Bar {
	return pb.Add64(int64(value))
}

// Inc atomically increments the progress
func (pb *Bar) Inc() *Bar {
	return pb.Add64(1)
}

// Set sets any value by any key
func (pb *Bar) Set(key, value interface{}) *Bar {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	if pb.vars == nil {
		pb.vars = make(map[interface{}]interface{})
	}
	pb.vars[key] = value
	return pb
}

// Get return value by key
func (pb *Bar) Get(key interface{}) interface{} {
	pb.mu.RLock()
	defer pb.mu.RUnlock()
	if pb.vars == nil {
		return nil
	}
	return pb.vars[key]
}

// GetBool return value by key and try to convert there to boolean
// If value doesn't set or not boolean - return false
func (pb *Bar) GetBool(key interface{}) bool {
	if v, ok := pb.Get(key).(bool); ok {
		return v
	}
	return false
}

// SetWidth sets the bar width
// When given value <= 0 would be using the terminal width (if possible) or default value.
func (pb *Bar) SetWidth(width int) *Bar {
	pb.mu.Lock()
	pb.width = width
	pb.mu.Unlock()
	return pb
}

// SetMaxWidth sets the bar maximum width
// When given value <= 0 would be using the terminal width (if possible) or default value.
func (pb *Bar) SetMaxWidth(maxWidth int) *Bar {
	pb.mu.Lock()
	pb.maxWidth = maxWidth
	pb.mu.Unlock()
	return pb
}

// Width return the bar width
// It's current terminal width or settled over 'SetWidth' value.
func (pb *Bar) Width() (width int) {
	defer func() {
		if r := recover(); r != nil {
			width = defaultBarWidth
		}
	}()
	pb.mu.RLock()
	width = pb.width
	maxWidth := pb.maxWidth
	pb.mu.RUnlock()
	if width <= 0 {
		var err error
		if width, err = terminalWidth(); err != nil {
			return defaultBarWidth
		}
	}
	if maxWidth > 0 && width > maxWidth {
		width = maxWidth
	}
	return
}

// StartTime return the time when bar started
func (pb *Bar) StartTime() time.Time {
	pb.mu.RLock()
	defer pb.mu.RUnlock()
	return pb.startTime
}

// Format convert int64 to string according to the current settings
func (pb *Bar) Format(v int64) string {
	if pb.GetBool(Bytes) {
		return formatBytes(v, pb.GetBool(SIBytesPrefix))
	}
	return strconv.FormatInt(v, 10)
}

// FinishAll stops the bar
func (pb *Bar) Finish() *Bar {
	pb.mu.Lock()
	if pb.finished {
		pb.mu.Unlock()
		return pb
	}
	pb.finished = true
	pb.mu.Unlock()
	return pb
}

// IsStarted indicates progress bar state
func (pb *Bar) IsStarted() bool {
	pb.mu.RLock()
	defer pb.mu.RUnlock()
	return pb.finished
}

// SetTemplateString sets Bar tempate string and parse it
func (pb *Bar) SetTemplateString(tmpl string) *Bar {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.tmpl, pb.err = getTemplate(tmpl)
	return pb
}

// SetTemplateString sets ProgressBarTempate and parse it
func (pb *Bar) SetTemplate(tmpl ProgressBarTemplate) *Bar {
	return pb.SetTemplateString(string(tmpl))
}

// NewProxyReader creates a wrapper for given reader, but with progress handle
// Takes io.Reader or io.ReadCloser
// Also, it automatically switches progress bar to handle units as bytes
func (pb *Bar) NewProxyReader(r io.Reader) *Reader {
	pb.Set(Bytes, true)
	return &Reader{r, pb}
}

// NewProxyWriter creates a wrapper for given writer, but with progress handle
// Takes io.Writer or io.WriteCloser
// Also, it automatically switches progress bar to handle units as bytes
func (pb *Bar) NewProxyWriter(r io.Writer) *Writer {
	pb.Set(Bytes, true)
	return &Writer{r, pb}
}

func (pb *Bar) render() (result string, width int) {
	defer func() {
		if r := recover(); r != nil {
			pb.SetErr(fmt.Errorf("render panic: %v", r))
		}
	}()
	pb.rm.Lock()
	defer pb.rm.Unlock()
	pb.mu.Lock()
	pb.configure()
	if pb.state == nil {
		pb.state = &State{Bar: pb}
		pb.buf = bytes.NewBuffer(nil)
	}
	if pb.startTime.IsZero() {
		pb.startTime = time.Now()
	}
	pb.state.id++
	pb.state.finished = pb.finished
	pb.state.time = time.Now()
	pb.mu.Unlock()

	pb.state.width = pb.Width()
	width = pb.state.width
	pb.state.total = pb.Total()
	pb.state.current = pb.Current()
	pb.buf.Reset()

	if e := pb.tmpl.Execute(pb.buf, pb.state); e != nil {
		pb.SetErr(e)
		return "", 0
	}

	result = pb.buf.String()

	aec := len(pb.state.recalc)
	if aec == 0 {
		// no adaptive elements
		return
	}

	staticWidth := CellCount(result) - (aec * adElPlaceholderLen)

	if pb.state.Width()-staticWidth <= 0 {
		result = strings.Replace(result, adElPlaceholder, "", -1)
		result = StripString(result, pb.state.Width())
	} else {
		pb.state.adaptiveElWidth = (width - staticWidth) / aec
		for _, el := range pb.state.recalc {
			result = strings.Replace(result, adElPlaceholder, el.ProgressElement(pb.state), 1)
		}
	}
	pb.state.recalc = pb.state.recalc[:0]
	return
}

// SetErr sets error to the Bar
// Error will be available over Err()
func (pb *Bar) SetErr(err error) *Bar {
	pb.mu.Lock()
	pb.err = err
	pb.mu.Unlock()
	return pb
}

// Err return possible error
// When all ok - will be nil
// May contain template.Execute errors
func (pb *Bar) Err() error {
	pb.mu.RLock()
	defer pb.mu.RUnlock()
	return pb.err
}

// String return currrent string representation of Bar
func (pb *Bar) String() string {
	res, _ := pb.render()
	return res
}

// ProgressElement implements Element interface
func (pb *Bar) ProgressElement(s *State, args ...string) string {
	if s.IsAdaptiveWidth() {
		pb.SetWidth(s.AdaptiveElWidth())
	}
	return pb.String()
}

// State represents the current state of bar
// Need for bar elements
type State struct {
	*Bar

	id                     uint64
	total, current         int64
	width, adaptiveElWidth int
	finished, adaptive     bool
	time                   time.Time

	recalc []Element
}

// Id it's the current state identifier
// - incremental
// - starts with 1
// - resets after finish/start
func (s *State) Id() uint64 {
	return s.id
}

// Total it's bar int64 total
func (s *State) Total() int64 {
	return s.total
}

// Value it's current value
func (s *State) Value() int64 {
	return s.current
}

// Width of bar
func (s *State) Width() int {
	return s.width
}

// AdaptiveElWidth - adaptive elements must return string with given cell count (when AdaptiveElWidth > 0)
func (s *State) AdaptiveElWidth() int {
	return s.adaptiveElWidth
}

// IsAdaptiveWidth returns true when element must be shown as adaptive
func (s *State) IsAdaptiveWidth() bool {
	return s.adaptive
}

// IsFinished return true when bar is finished
func (s *State) IsFinished() bool {
	return s.finished
}

// IsFirst return true only in first render
func (s *State) IsFirst() bool {
	return s.id == 1
}

// Time when state was created
func (s *State) Time() time.Time {
	return s.time
}
