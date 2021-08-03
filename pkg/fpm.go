package pkg

import (
	"fmt"
	"path/filepath"
	"sort"
	"time"
)

type PHPCGI struct {
	address, binPath string
	c                *Command
	serving, port    int
	running          bool
}

func (p *PHPCGI) Pid() int {
	if p.c == nil {
		return 0
	}

	return p.c.pid
}

func (p *PHPCGI) Address() string {
	return p.address
}

func (p *PHPCGI) Served() {
	p.serving--
	if p.serving < 0 {
		p.serving = 0
	}
}

func (p *PHPCGI) Serving() int {
	return p.serving
}

func (p *PHPCGI) Sort() int {
	if p.running && p.serving <= 0 {
		return 0
	}
	if !p.running {
		return 1
	}

	return 10 + p.serving
}

func (p *PHPCGI) Running() bool {
	return p.running
}

func (p *PHPCGI) Serve() *PHPCGI {
	if !p.running {
		c := NewCommand(fmt.Sprintf("%s -b %s", filepath.Join(p.binPath, "php-cgi.exe"), p.address), nil)
		c.GoExec(p.binPath)
		p.c = c
		p.running = true
	}
	p.serving++
	return p
}

func (p *PHPCGI) Stop() *PHPCGI {
	if p.running {
		p.c.Stop()
		p.running = false
		p.serving = 0
		p.c = nil
	}
	return p
}

type CGIS []*PHPCGI

func (p CGIS) Len() int {
	return len(p)
}

func (p CGIS) Less(i, j int) bool {
	return p[i].Sort() < p[j].Sort()
}

func (p CGIS) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func (p CGIS) HasIdel() bool {
	for _, p := range p {
		if p.serving <= 0 {
			return true
		}
	}
	return false
}

func (p CGIS) IdelCount() int {
	var i int

	for _, p := range p {
		if p.serving <= 0 && p.running {
			i++
		}
	}

	return i
}

type FPM struct {
	startPort, maxProcess, processNum, currentPort int
	binPath                                        string
	cgis                                           CGIS
}

func (f *FPM) Select() *PHPCGI {
	if f.currentPort == 0 {
		f.currentPort = f.startPort
	}

	if f.processNum < f.maxProcess && !f.cgis.HasIdel() {
		f.processNum++
		addr := fmt.Sprintf("127.0.0.1:%d", f.currentPort)

		f.cgis = append(f.cgis, &PHPCGI{
			address: addr,
			serving: 0,
			binPath: f.binPath,
			port:    f.currentPort,
		})

		f.startPort++
		f.currentPort++
	}
	sort.Sort(f.cgis)

	p := f.cgis[0]

	return p.Serve()
}

func (f *FPM) Prune(left int) {
	sort.Sort(f.cgis)
	count := f.cgis.IdelCount()
	if count > left {
		kills := count - left
		var i int
		for _, p := range f.cgis {
			if i >= kills {
				break
			}
			if p.serving <= 0 {
				p.Stop()
				i++
			}
		}
	}
}

func (f *FPM) PruneInterval(interval time.Duration, left int) {
	go func() {
		for {
			<-time.After(interval)
			f.Prune(left)
		}
	}()
}

func (f *FPM) Close() {
	for _, p := range f.cgis {
		p.Stop()
	}
}

func (f *FPM) Cgis() CGIS {
	return f.cgis
}

func NewFPM(startPort, maxProcess int, binPath string) *FPM {
	return &FPM{startPort: startPort, maxProcess: maxProcess, binPath: binPath, cgis: make(CGIS, 0)}
}
