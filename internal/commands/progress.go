package commands

import (
	"fmt"
	"sync"
	"time"
)

const uploadProgressMinInterval = 5 * time.Second

type uploadProgressPrinter struct {
	mu            sync.Mutex
	print         func(format string, args ...any)
	lastPercent   int64
	lastPrintedAt time.Time
	printed       bool
	printedDone   bool
}

func newUploadProgressPrinter(print func(format string, args ...any)) *uploadProgressPrinter {
	return &uploadProgressPrinter{
		print:       print,
		lastPercent: -1,
	}
}

func (p *uploadProgressPrinter) Callback(done int64, total int64) {
	p.printProgress(done, total, false)
}

func (p *uploadProgressPrinter) Finish(total int64) {
	if total > 0 {
		p.printProgress(total, total, true)
		return
	}
	p.printProgress(0, 0, true)
}

func (p *uploadProgressPrinter) printProgress(done int64, total int64, force bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.print == nil || p.printedDone {
		return
	}
	if done < 0 {
		done = 0
	}
	now := time.Now()
	if total <= 0 {
		if !force && p.printed && now.Sub(p.lastPrintedAt) < uploadProgressMinInterval {
			return
		}
		p.print("upload progress done=%s total=unknown", formatBytes(done))
		p.printed = true
		p.lastPrintedAt = now
		return
	}
	if done > total {
		done = total
	}

	percentFloat := float64(done) * 100 / float64(total)
	percent := int64(percentFloat)
	shouldPrint := force ||
		!p.printed ||
		percent >= 100 ||
		percent-p.lastPercent >= 5 ||
		now.Sub(p.lastPrintedAt) >= uploadProgressMinInterval
	if !shouldPrint {
		return
	}

	p.print("upload progress %.1f%% done=%s total=%s", percentFloat, formatBytes(done), formatBytes(total))
	p.printed = true
	p.lastPercent = percent
	p.lastPrintedAt = now
	if percent >= 100 {
		p.printedDone = true
	}
}

func formatBytes(value int64) string {
	if value < 0 {
		value = 0
	}
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%dB", value)
	}
	units := []string{"KiB", "MiB", "GiB", "TiB"}
	size := float64(value)
	for _, suffix := range units {
		size /= unit
		if size < unit {
			return fmt.Sprintf("%.2f%s", size, suffix)
		}
	}
	return fmt.Sprintf("%.2fPiB", size/unit)
}
