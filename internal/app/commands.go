package app

import (
	"context"
	"time"

	"simmer/internal/device"

	tea "charm.land/bubbletea/v2"
)

func (m model) fetchDevicesCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return discoveryMsg(m.coordinator.Discover(ctx))
	}
}

func (m model) fetchDeviceTypesCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		types, err := m.coordinator.ListDeviceTypes(ctx, device.PlatformIOS)
		return deviceTypesMsg{types: types, err: err}
	}
}

func (m model) fetchRuntimesCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		runtimes, err := m.coordinator.ListRuntimes(ctx, device.PlatformIOS)
		return runtimesMsg{runtimes: runtimes, err: err}
	}
}

func (m model) createIOSSimulatorCmd(name, deviceTypeID, runtimeID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		udid, err := m.coordinator.Create(ctx, device.PlatformIOS, name, deviceTypeID, runtimeID)
		return createSimulatorResultMsg{name: name, udid: udid, err: err}
	}
}

func (m model) deleteDeviceCmd(dev device.Device) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := m.coordinator.Delete(ctx, dev)
		return deleteSimulatorResultMsg{name: dev.Name, err: err}
	}
}

func (m model) fetchAndroidSystemImagesCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		imgs, err := m.coordinator.ListRuntimes(ctx, device.PlatformAndroid)
		return androidSystemImagesMsg{images: imgs, err: err}
	}
}

func (m model) fetchAndroidDeviceProfilesCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		profiles, err := m.coordinator.ListDeviceTypes(ctx, device.PlatformAndroid)
		return androidDeviceProfilesMsg{profiles: profiles, err: err}
	}
}

func (m model) createAndroidEmulatorCmd(name, systemImagePkg, deviceProfileID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		_, err := m.coordinator.Create(ctx, device.PlatformAndroid, name, deviceProfileID, systemImagePkg)
		return createAndroidEmulatorResultMsg{name: name, err: err}
	}
}

func (m model) bootDeviceCmd(dev device.Device) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return bootResultMsg{device: dev, err: m.coordinator.Boot(ctx, dev)}
	}
}

func (m model) shutdownDeviceCmd(dev device.Device) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return shutdownResultMsg{device: dev, err: m.coordinator.Shutdown(ctx, dev)}
	}
}

func (m model) loadIOSAppFileTreeCmd(dev device.Device, app device.App) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		fs := device.NewIOSAppFileSystem(app.BundleID)
		root, err := fs.Tree(ctx, dev)
		return fileTreeMsg{device: dev, root: root, err: err}
	}
}

func (m model) loadIOSRootFileTreeCmd(dev device.Device) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		fs := device.NewIOSRootFileSystem()
		root, err := fs.Tree(ctx, dev)
		return fileTreeMsg{device: dev, root: root, err: err}
	}
}

func (m model) loadAndroidFileTreeCmd(dev device.Device, app device.App) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		fs := device.NewAndroidFileSystem(app.BundleID)
		root, err := fs.Tree(ctx, dev)
		return fileTreeMsg{device: dev, root: root, err: err}
	}
}

func (m model) loadAndroidRootFileTreeCmd(dev device.Device) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		fs := device.NewAndroidRootFileSystem()
		root, err := fs.Tree(ctx, dev)
		return fileTreeMsg{device: dev, root: root, err: err}
	}
}

func (m model) loadAppsCmd(dev device.Device) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		apps, err := m.coordinator.ListApps(ctx, dev)

		return appsListMsg{device: dev, apps: apps, err: err}
	}
}

func (m model) loadInfoCmd(dev device.Device) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		info, err := m.coordinator.Info(ctx, dev)
		return infoMsg{device: dev, info: info, err: err}
	}
}

// scheduleBootPoll returns a Cmd that fires bootPollMsg after 4 seconds.
// Used to keep refreshing after an Android emulator boot until adb sees it.
func scheduleBootPoll(dev device.Device, remaining int) tea.Cmd {
	if remaining <= 0 {
		return nil
	}
	return tea.Tick(4*time.Second, func(_ time.Time) tea.Msg {
		return bootPollMsg{device: dev, remaining: remaining}
	})
}

func nextLogLineCmd(stream *device.LogStream, bundleID string) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-stream.Lines
		if !ok {
			err := <-stream.Done
			return logEndedMsg{bundleID: bundleID, err: err}
		}
		return logLineMsg{bundleID: bundleID, line: line}
	}
}
