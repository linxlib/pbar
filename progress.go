package pbar

import (
	"fmt"
	"github.com/mattn/go-colorable"
	"io"
	"sync"
	"time"

	"github.com/gosuri/uilive"
)

// RefreshInterval in the default time duration to wait for refreshing the output
var defaultRefreshInterval = time.Millisecond * 200

// defaultProgress is the default progress
var defaultProgress = NewProgress()

// Progress represents the container that renders progress bars
type Progress struct {
	// Out is the writer to render progress bars to
	lw       *uilive.Writer
	ticker   *time.Ticker
	finish   chan bool
	mtx      *sync.RWMutex
	interval time.Duration

	// Width is the width of the progress bars
	Width int

	// Bars is the collection of progress bars
	Bars []*Bar
}

// New returns a new progress bar with defaults
func NewProgress() *Progress {
	lw := uilive.New()
	lw.Out = colorable.NewColorableStdout()

	return &Progress{
		Width:    defaultBarWidth,
		Bars:     make([]*Bar, 0),
		lw:       lw,
		interval: defaultRefreshInterval,
		mtx:      &sync.RWMutex{},
	}
}

// AddBar creates a new progress bar and adds it to the default progress container
func AddBar(total int) *Bar {
	return defaultProgress.AddBar(total)
}

// Start starts the rendering the progress of progress bars using the DefaultProgress. It listens for updates using `bar.Set(n)` and new bars when added using `AddBar`
func Start() {
	defaultProgress.Start()
}

// Stop stops listening
func FinishAll() {
	defaultProgress.FinishAll()
}

func (p *Progress) SetOut(o io.Writer) {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	p.lw.Out = o
}

func (p *Progress) SetRefreshInterval(interval time.Duration) {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	p.interval = interval
	p.ticker = time.NewTicker(interval)
}

// AddBar creates a new progress bar and adds to the container
func (p *Progress) AddBar(total int) *Bar {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	bar := Building.New(total)
	bar.SetWidth(defaultBarWidth)

	p.Bars = append(p.Bars, bar)
	return bar
}

// Start starts the rendering the progress of progress bars. It listens for updates using `bar.Set(n)` and new bars when added using `AddBar`
func (p *Progress) Start() {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	for i, _ := range p.Bars {
		p.Bars[i].Start()
	}
	p.ticker = time.NewTicker(p.interval)
	p.finish = make(chan bool)
	go p.listen()
}

// Stop stops listening
func (p *Progress) FinishAll() {
	p.finish <- true
	close(p.finish)
	<-p.finish
}

// listen listens for updates and renders the progress bars
func (p *Progress) listen() {
	for {
		select {
		case <-p.ticker.C:
			p.print()
		case <-p.finish:
			for _, v := range p.Bars {
				v.Finish()
			}
			p.print()

			return
		}
	}
}

func (p *Progress) print() {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	for _, bar := range p.Bars {
		fmt.Fprintln(p.lw, bar.bytes(false))
	}
	p.lw.Flush()
}
