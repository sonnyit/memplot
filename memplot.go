package memplot

import (
	"errors"
	"fmt"
	"github.com/shirou/gopsutil/process"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"image/color"
	"time"
)

// Process data for a given instant
type Instant struct {
	MemoryInfo *process.MemoryInfoStat
	NumThreads int32
	Instant    time.Duration
}

type Collection struct {
	Pid            int32
	StartTime      time.Time
	SampleDuration time.Duration // Time between samples
	Samples        []Instant
}

// Gather a process resident size in memory in batch
func NewCollection(pid int32, sd, dur time.Duration) (*Collection, error) {
	numsamples := dur / sd
	if dur != 0 && numsamples < 2 {
		return nil, errors.New("There must be at least two samples. Sample Duration too short")
	}

	proc, err := process.NewProcess(pid)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	var mem *process.MemoryInfoStat
	var nthreads int32
	coll := &Collection{
		Pid:            pid,
		StartTime:      start,
		SampleDuration: sd,
		Samples:        make([]Instant, 0),
	}

	// el = elapsed time, dur = total duration
	running, err := proc.IsRunning()
	if err != nil {
		return nil, err
	}
	for el := time.Since(start); (dur == 0 || el <= dur) && running; el = time.Since(start) {
		mem, err = proc.MemoryInfo()
		if err != nil {
			return nil, err
		}
		nthreads, err = proc.NumThreads()
		if err != nil {
			return nil, err
		}

		instant := Instant{
			MemoryInfo: mem,
			Instant:    el,
			NumThreads: nthreads,
		}

		coll.Samples = append(coll.Samples, instant)
		time.Sleep(sd)
		running, err = proc.IsRunning()
		if err != nil {
			return nil, err
		}
	}

	return coll, nil
}

// Gather RSS points from a memory collection
func (m *Collection) GatherRSSXYs() plotter.XYs {
	pts := make(plotter.XYs, len(m.Samples))
	for i, s := range m.Samples {
		pts[i].X = s.Instant.Seconds()
		pts[i].Y = float64(m.Samples[i].MemoryInfo.RSS) / 1024
	}

	return pts
}

// Gather VSZ points from a memory collection
func (m *Collection) GatherVSZXYs() plotter.XYs {
	pts := make(plotter.XYs, len(m.Samples))
	for i, s := range m.Samples {
		pts[i].X = s.Instant.Seconds()
		pts[i].Y = float64(m.Samples[i].MemoryInfo.VMS) / 1024
	}

	return pts
}

type PlotOptions struct {
	PlotRss bool
	PlotVsz bool
	// PlotNumThreads bool
}

// Plot a memory collection
func (m *Collection) Plot(opt PlotOptions) (*plot.Plot, error) {
	p, err := plot.New()
	if err != nil {
		return nil, err
	}

	p.Title.Text = fmt.Sprintf("Memory Plot of PID %d", m.Pid)
	p.X.Label.Text = "Time (Seconds)"
	p.Y.Label.Text = "KiloBytes"
	// Draw a grid behind the area
	p.Add(plotter.NewGrid())

	if opt.PlotRss {
		// RSS line plotter and style
		rssData := m.GatherRSSXYs()
		rssLine, err := plotter.NewLine(rssData)
		if err != nil {
			return nil, err
		}
		rssLine.LineStyle.Width = vg.Points(1)
		rssLine.LineStyle.Color = color.RGBA{R: 0, G: 0, B: 0, A: 255}

		// Add the plotters to the plot, with legend entries
		p.Add(rssLine)
		p.Legend.Add("RSS", rssLine)
	}

	// TODO add another Y axis for vsz
	if opt.PlotVsz {
		// RSS line plotter and style
		vszData := m.GatherVSZXYs()
		vszLine, err := plotter.NewLine(vszData)
		if err != nil {
			return nil, err
		}
		vszLine.LineStyle.Width = vg.Points(1)
		vszLine.LineStyle.Color = color.RGBA{R: 0, G: 0, B: 255, A: 255}

		// Add the plotters to the plot, with legend entries
		p.Add(vszLine)
		p.Legend.Add("VSZ", vszLine)
	}

	return p, nil
}

func SavePlot(p *plot.Plot, width, height vg.Length, filename string) error {
	return p.Save(width, height, filename)
}
