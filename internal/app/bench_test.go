package app

import (
	"fmt"
	"testing"

	"simmer/internal/device"

	tea "charm.land/bubbletea/v2"
)

func benchModel(b *testing.B, n int) model {
	b.Helper()
	m := newTestModel()
	r, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 45})
	m = r.(model)
	m.sidebar.SetDevices(benchDevices(n))
	return m
}

func benchDevices(n int) []device.Device {
	devs := make([]device.Device, 0, n)
	for i := 0; i < n; i++ {
		d := device.Device{
			ID:       fmt.Sprintf("id-%d", i),
			Name:     fmt.Sprintf("Device %d", i),
			Platform: device.PlatformIOS,
			Version:  "17.0",
			Status:   device.StatusOff,
		}
		if i%2 == 0 {
			d.Platform = device.PlatformAndroid
		}
		if i%5 == 0 {
			d.Status = device.StatusRunning
		}
		devs = append(devs, d)
	}
	return devs
}

func BenchmarkView(b *testing.B) {
	for _, n := range []int{5, 50, 200} {
		b.Run(fmt.Sprintf("devices=%d", n), func(b *testing.B) {
			m := benchModel(b, n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = m.View()
			}
		})
	}
}

func BenchmarkUpdateKeyNav(b *testing.B) {
	m := benchModel(b, 50)
	keys := []tea.KeyPressMsg{{Code: 'j'}, {Code: 'k'}}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r, _ := m.Update(keys[i%2])
		m = r.(model)
	}
}

func BenchmarkUpdateAndView(b *testing.B) {
	m := benchModel(b, 50)
	keys := []tea.KeyPressMsg{{Code: 'j'}, {Code: 'k'}}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r, _ := m.Update(keys[i%2])
		m = r.(model)
		_ = m.View()
	}
}

func BenchmarkResize(b *testing.B) {
	m := benchModel(b, 50)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r, _ := m.Update(tea.WindowSizeMsg{Width: 100 + i%40, Height: 30 + i%10})
		m = r.(model)
	}
}
