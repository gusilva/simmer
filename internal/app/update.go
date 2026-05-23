package app

import (
	"context"
	"time"

	"simmer/internal/device"
	"simmer/internal/ui"
	"simmer/internal/ui/dbviewer"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/spinner"
)

func (m model) Init() tea.Cmd {
	return tea.Batch(m.fetchDevicesCmd(), scheduleAutoRefresh())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Body wrapper applies 1 line of top padding around each panel; pass
		// the remaining height so panels fill exactly.
		bodyH := m.bodyHeight() - 1
		m.sidebar.SetSize(ui.DefaultSidebarWidth, bodyH)
		// Sidebar wrapper: pad 1 left + 1 right around DefaultSidebarWidth.
		// MainPane wrapper: pad 0 left + 1 right around its width.
		mainW := max(m.width-(ui.DefaultSidebarWidth+2)-1, 0)
		m.mainPane.SetSize(mainW, bodyH)
		if m.sqliteModal != nil {
			m.sqliteModal.SetSize(m.width, m.height)
		}
		if m.dbViewerModal != nil {
			m.dbViewerModal.SetSize(m.width, m.height)
		}
		return m, nil

	case tea.KeyPressMsg:
		// ctrl+c always quits, even when an overlay is active.
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}

		// ? toggles context-aware help; esc also closes it.
		if msg.String() == "?" || (msg.String() == "esc" && m.helpOverlay != nil) {
			if m.helpOverlay != nil {
				m.helpOverlay = nil
				return m, nil
			}
			ov := m.contextHelpOverlay()
			m.helpOverlay = &ov
			return m, nil
		}
		// Consume all other keys while help is open.
		if m.helpOverlay != nil {
			return m, nil
		}

		// ctrl+d toggles the DB viewer regardless of focus or overlay state.
		// [TODO] remove it. Used only for testing and debugging.
		if msg.String() == "ctrl+d" {
			if m.dbViewerModal != nil {
				m.dbViewerModal = nil
				return m, nil
			}
			if m.platformPicker == nil && m.createIOSModal == nil && m.createAndModal == nil && m.deleteAlert == nil && m.deleteAppAlert == nil && m.installAppModal == nil && m.sqliteModal == nil {
				modal := dbviewer.New(func() tea.Msg { return ui.CancelOverlayMsg{} })
				modal.SetSize(m.width, m.height)
				m.dbViewerModal = &modal
			}
			return m, nil
		}

		// Overlay intercepts all other keys when active.
		if m.platformPicker != nil {
			updated, cmd := m.platformPicker.Update(msg)
			m.platformPicker = &updated
			return m, cmd
		}
		if m.createIOSModal != nil {
			updated, cmd := m.createIOSModal.Update(msg)
			m.createIOSModal = &updated
			return m, cmd
		}
		if m.createAndModal != nil {
			updated, cmd := m.createAndModal.Update(msg)
			m.createAndModal = &updated
			return m, cmd
		}
		if m.deleteAlert != nil {
			updated, cmd := m.deleteAlert.Update(msg)
			m.deleteAlert = &updated
			return m, cmd
		}
		if m.deleteAppAlert != nil {
			updated, cmd := m.deleteAppAlert.Update(msg)
			m.deleteAppAlert = &updated
			return m, cmd
		}
		if m.installAppModal != nil {
			updated, cmd := m.installAppModal.Update(msg)
			m.installAppModal = &updated
			return m, cmd
		}
		if m.sqliteModal != nil {
			updated, cmd := m.sqliteModal.Update(msg)
			m.sqliteModal = &updated
			return m, cmd
		}
		if m.dbViewerModal != nil {
			updated, cmd := m.dbViewerModal.Update(msg)
			m.dbViewerModal = &updated
			return m, cmd
		}

		// Global keys (no overlay active).
		if msg.String() == "q" {
			m.quitting = true
			return m, tea.Quit
		}

		if m.focus == focusMain {
			if msg.String() == "esc" {
				m.focus = focusSidebar
				m.applyFocus()
				return m, nil
			}
			var cmd tea.Cmd
			m.mainPane, cmd = m.mainPane.Update(msg)
			return m, cmd
		}

		// focus == focusSidebar
		switch msg.String() {
		case "r":
			m.loading = true
			m.errs = nil
			return m, m.fetchDevicesCmd()
		case "b":
			sel := m.sidebar.SelectedDevice()
			if sel == nil || sel.Status == device.StatusRunning {
				return m, nil
			}
			m.booting = true
			return m, tea.Batch(
				m.bootDeviceCmd(*sel),
				m.setStatus("Booting "+sel.Name+"…", ui.StatusInfo),
				tea.Cmd(m.installSpinner.Tick),
			)
		case "s":
			sel := m.sidebar.SelectedDevice()
			if sel == nil || sel.Status != device.StatusRunning || sel.Kind == device.KindPhysical {
				return m, nil
			}
			return m, m.shutdownDeviceCmd(*sel)
		case "space":
			sel := m.sidebar.SelectedDevice()
			if sel == nil || sel.Status != device.StatusRunning {
				return m, nil
			}

			if m.logStream != nil {
				m.logStream.Stop()
				m.logStream = nil
				m.logBundleID = ""
				m.logDeviceID = ""
				m.mainPane.SetLogBundle("")
			}

			m.focus = focusMain
			m.applyFocus()

			dev := *sel
			m.mainPane.SetDevice(&dev, nil)

			return m, tea.Batch(m.loadInfoCmd(dev), m.loadAppsCmd(dev))
		}
		var cmd tea.Cmd
		m.sidebar, cmd = m.sidebar.Update(msg)

		return m, cmd

	case tea.PasteMsg, tea.ClipboardMsg:
		if m.dbViewerModal != nil {
			updated, cmd := m.dbViewerModal.Update(msg)
			m.dbViewerModal = &updated
			return m, cmd
		}
		if m.sqliteModal != nil {
			updated, cmd := m.sqliteModal.Update(msg)
			m.sqliteModal = &updated
			return m, cmd
		}
		return m, nil

	case spinner.TickMsg:
		if m.installing || m.booting {
			var spinCmd tea.Cmd
			m.installSpinner, spinCmd = m.installSpinner.Update(msg)
			return m, spinCmd
		}
		return m, nil

	case autoRefreshMsg:
		return m, tea.Batch(m.fetchDevicesCmd(), scheduleAutoRefresh())

	case discoveryMsg:
		m.loading = false
		m.errs = msg.Errors
		m.toolVersions = msg.ToolVersions
		m.lastRefresh = time.Now()

		booted, ios, android := 0, 0, 0
		for _, dev := range msg.Devices {
			if dev.Status == device.StatusRunning {
				booted++
			}
			switch dev.Platform {
			case device.PlatformIOS:
				ios++
			case device.PlatformAndroid:
				android++
			}
		}
		m.bootedCount = booted
		m.iosCount = ios
		m.androidCount = android
		m.sidebar.SetDevices(msg.Devices)
		m.mainPane.SyncActiveDevice(msg.Devices)
		if !m.mainPane.HasDevice() && m.logStream != nil {
			m.logStream.Stop()
			m.logStream = nil
			m.logBundleID = ""
			m.logDeviceID = ""
		}
		return m, nil

	case bootResultMsg:
		if msg.err != nil {
			m.booting = false
			m.errs = append(m.errs, msg.err)
			return m, m.setStatus("boot failed: "+errPreview(msg.err), ui.StatusErr)
		}

		m.loading = true

		if msg.device.Platform == device.PlatformAndroid {
			// Emulator registers with adb asynchronously; keep spinner and poll
			// until adb confirms it's running before clearing booting state.
			return m, tea.Batch(m.fetchDevicesCmd(), scheduleBootPoll(msg.device, 15))
		}

		m.booting = false
		return m, tea.Batch(m.fetchDevicesCmd(), m.setStatus("booted "+msg.device.Name, ui.StatusOk))

	case bootPollMsg:
		// Check if the device now shows as running; if so, stop polling.
		for _, dev := range m.sidebar.Devices() {
			if dev.ID == msg.device.ID && dev.Status == device.StatusRunning {
				m.booting = false
				return m, m.setStatus("booted "+msg.device.Name, ui.StatusOk)
			}
		}

		if msg.remaining-1 <= 0 {
			m.booting = false
		}

		return m, tea.Batch(
			m.fetchDevicesCmd(),
			scheduleBootPoll(msg.device, msg.remaining-1),
		)

	case shutdownResultMsg:
		if msg.err != nil {
			m.errs = append(m.errs, msg.err)

			return m, m.setStatus("shutdown failed: "+errPreview(msg.err), ui.StatusErr)
		}

		m.loading = true

		return m, tea.Batch(
			m.fetchDevicesCmd(),
			m.setStatus("shut down "+msg.device.Name, ui.StatusOk),
		)

	case fileTreeMsg:
		if msg.err != nil {
			m.errs = append(m.errs, msg.err)

			return m, m.setStatus("files load failed: "+errPreview(msg.err), ui.StatusErr)
		}

		m.mainPane.SetTree(&msg.root)

		return m, nil

	case appsListMsg:
		if msg.err != nil {
			m.errs = append(m.errs, msg.err)

			return m, m.setStatus("apps load failed: "+errPreview(msg.err), ui.StatusErr)
		}

		m.mainPane.SetApps(msg.apps)

		return m, nil

	case infoMsg:
		if msg.err != nil {
			m.errs = append(m.errs, msg.err)

			return m, m.setStatus("info load failed: "+errPreview(msg.err), ui.StatusErr)
		}

		m.mainPane.SetInfo(msg.info)

		return m, nil

	case ui.ClipboardCopiedMsg:
		if msg.Err != nil {
			m.errs = append(m.errs, msg.Err)

			return m, m.setStatus("copy failed: "+errPreview(msg.Err), ui.StatusErr)
		}

		return m, m.setStatus("copied "+textPreview(msg.Text, 40), ui.StatusOk)

	case ui.AppFocusedMsg:
		return m, m.setStatus("app: "+msg.App.Label(), ui.StatusInfo)

	case clearStatusMsg:
		if int(msg) == m.statusSeq && !m.installing && !m.booting {
			m.status = ""
		}

		return m, nil

	case ui.RequestFileTreeMsg:
		sel := m.sidebar.SelectedDevice()
		if sel == nil {
			return m, nil
		}

		if msg.App == nil {
			switch sel.Platform {
			case device.PlatformIOS:
				if sel.Kind == device.KindPhysical {
					return m, m.loadIOSPhysicalFileTreeCmd(*sel)
				}
				return m, m.loadIOSRootFileTreeCmd(*sel)
			case device.PlatformAndroid:
				if sel.Kind == device.KindPhysical {
					return m, m.loadAndroidPhysicalFileTreeCmd(*sel)
				}
				return m, m.loadAndroidRootFileTreeCmd(*sel)
			}
			return m, nil
		}

		switch sel.Platform {
		case device.PlatformIOS:
			if sel.Kind == device.KindPhysical {
				return m, m.loadIOSPhysicalAppFileTreeCmd(*sel, *msg.App)
			}
			return m, m.loadIOSAppFileTreeCmd(*sel, *msg.App)
		case device.PlatformAndroid:
			if sel.Kind == device.KindPhysical {
				// Physical Android: show external storage (no run-as access without root)
				return m, m.loadAndroidPhysicalFileTreeCmd(*sel)
			}
			return m, m.loadAndroidFileTreeCmd(*sel, *msg.App)
		}

		return m, nil

	case ui.StopLogStreamMsg:
		if m.logStream != nil {
			m.logStream.Stop()
			m.logStream = nil
		}
		m.logBundleID = ""
		m.logDeviceID = ""
		m.mainPane.SetLogBundle("")

		return m, nil

	case ui.RequestLogStreamMsg:
		sel := m.sidebar.SelectedDevice()
		if sel == nil {
			return m, nil
		}
		bundleID := msg.App.BundleID
		if m.logStream != nil && m.logBundleID == bundleID && m.logDeviceID == sel.ID {
			return m, nil
		}
		if m.logStream != nil {
			m.logStream.Stop()
			m.logStream = nil
		}
		stream, err := m.coordinator.StreamLogs(context.Background(), *sel, msg.App)
		if err != nil {
			m.errs = append(m.errs, err)
			return m, m.setStatus("stream failed: "+errPreview(err), ui.StatusErr)
		}
		m.logStream = stream
		m.logBundleID = bundleID
		m.logDeviceID = sel.ID
		m.mainPane.SetLogBundle(bundleID)
		return m, tea.Batch(
			nextLogBatchCmd(stream, bundleID),
			m.setStatus("streaming "+bundleID, ui.StatusOk),
		)

	case logBatchMsg:
		if msg.bundleID != m.logBundleID || m.logStream == nil {
			return m, nil
		}
		for _, line := range msg.lines {
			m.mainPane.AppendLog(line)
		}
		return m, nextLogBatchCmd(m.logStream, m.logBundleID)

	case logEndedMsg:
		if msg.bundleID != m.logBundleID {
			return m, nil
		}
		m.logStream = nil
		m.logBundleID = ""
		m.logDeviceID = ""
		m.mainPane.SetLogBundle("")
		if msg.err != nil {
			m.errs = append(m.errs, msg.err)
			return m, m.setStatus("stream ended: "+errPreview(msg.err), ui.StatusWarn)
		}
		return m, m.setStatus("stream ended", ui.StatusInfo)

	case ui.ShowPlatformPickerMsg:
		p := ui.NewPlatformPickerModal()
		m.platformPicker = &p
		return m, nil

	case ui.ConfirmPlatformPickerMsg:
		m.platformPicker = nil
		if msg.Platform == device.PlatformIOS {
			modal, focusCmd := ui.NewCreateSimulatorModal()
			m.createIOSModal = &modal
			return m, tea.Batch(focusCmd, m.fetchDeviceTypesCmd(), m.fetchRuntimesCmd())
		}
		andModal, focusCmd := ui.NewCreateAndroidEmulatorModal()
		m.createAndModal = &andModal
		return m, tea.Batch(focusCmd, m.fetchAndroidSystemImagesCmd(), m.fetchAndroidDeviceProfilesCmd())

	case ui.ShowDeleteSimulatorMsg:
		alert := ui.NewDeleteSimulatorAlert(msg.Device)
		m.deleteAlert = &alert
		return m, nil

	case ui.ShowInstallAppMsg:
		modal, focusCmd := ui.NewInstallAppModal(msg.Device)
		m.installAppModal = &modal
		return m, focusCmd

	case ui.RequestXcodeSchemesMsg:
		return m, fetchXcodeSchemesCmd(msg.Path)

	case xcodeSchemesMsg:
		if m.installAppModal != nil {
			if msg.err != nil {
				m.installAppModal.SetSchemesError(msg.err.Error())
			} else {
				m.installAppModal.SetSchemes(msg.schemes)
			}
		}
		return m, nil

	case ui.ConfirmInstallAppMsg:
		m.installAppModal = nil
		if m.buildStream != nil {
			m.buildStream.Stop()
			m.buildStream = nil
		}
		m.installing = true
		return m, tea.Batch(
			m.startInstallCmd(msg.Device, msg.Path, msg.Scheme),
			m.setStatus("Starting build…", ui.StatusInfo),
			tea.Cmd(m.installSpinner.Tick),
		)

	case buildStartedMsg:
		m.buildStream = msg.stream
		m.buildDeviceID = msg.device.ID
		return m, nextBuildEventCmd(msg.stream, msg.device.ID)

	case buildEventMsg:
		if m.buildStream == nil || msg.deviceID != m.buildDeviceID {
			return m, nil
		}
		return m, tea.Batch(
			m.setStatus(msg.text, ui.StatusInfo),
			nextBuildEventCmd(m.buildStream, msg.deviceID),
		)

	case buildDoneMsg:
		stream := m.buildStream
		m.buildStream = nil
		m.buildDeviceID = ""
		m.installing = false
		if stream != nil {
			stream.Stop()
		}
		if msg.err != nil {
			m.errs = append(m.errs, msg.err)
			return m, m.setStatus("install failed: "+errPreview(msg.err), ui.StatusErr)
		}
		sel := m.sidebar.SelectedDevice()
		var reloadCmd tea.Cmd
		if sel != nil && sel.ID == msg.deviceID {
			reloadCmd = m.loadAppsCmd(*sel)
		}
		return m, tea.Batch(reloadCmd, m.setStatus("Installed & launched", ui.StatusOk))

	case ui.ShowDeleteAppMsg:
		alert := ui.NewDeleteAppAlert(msg.Device, msg.App)
		m.deleteAppAlert = &alert
		return m, nil

	case ui.ShowSQLiteViewerMsg:
		modal := dbviewer.New(func() tea.Msg { return ui.CancelOverlayMsg{} })
		modal.SetSize(m.width, m.height)

		fileCmd := modal.SetFile(msg.Device, msg.PackageID, msg.DBPath, msg.DBName, m.logger)

		m.dbViewerModal = &modal
		m.sqliteModal = nil

		return m, fileCmd

	case ui.SQLiteResultMsg:
		if m.sqliteModal != nil {
			m.sqliteModal.SetResult(msg.Rows, msg.Err)
		}
		return m, nil

	case dbviewer.SQLiteVersionMsg, dbviewer.TablesLoadedMsg, dbviewer.ColumnsLoadedMsg, dbviewer.FileSavedMsg, dbviewer.QueryResultMsg, dbviewer.SettingsSavedMsg, dbviewer.SettingsCancelMsg, dbviewer.ScriptsLoadedMsg, dbviewer.ScriptLoadedMsg:
		if m.dbViewerModal != nil {
			updated, cmd := m.dbViewerModal.Update(msg)
			m.dbViewerModal = &updated

			return m, cmd
		}

		return m, nil

	case tea.MouseClickMsg, tea.MouseWheelMsg:
		if m.dbViewerModal != nil {
			updated, cmd := m.dbViewerModal.Update(msg)
			m.dbViewerModal = &updated
			return m, cmd
		}
		return m, nil

	case dbviewer.ShowMsg:
		modal := dbviewer.New(func() tea.Msg { return ui.CancelOverlayMsg{} })
		modal.SetSize(m.width, m.height)
		m.dbViewerModal = &modal
		return m, nil

	case ui.CancelOverlayMsg:
		m.platformPicker = nil
		m.createIOSModal = nil
		m.createAndModal = nil
		m.deleteAlert = nil
		m.deleteAppAlert = nil
		m.installAppModal = nil
		m.sqliteModal = nil
		m.dbViewerModal = nil
		return m, nil

	case ui.ConfirmCreateSimulatorMsg:
		m.createIOSModal = nil
		return m, m.createIOSSimulatorCmd(msg.Name, msg.DeviceTypeID, msg.RuntimeID)

	case ui.ConfirmCreateAndroidEmulatorMsg:
		m.createAndModal = nil
		return m, m.createAndroidEmulatorCmd(msg.Name, msg.SystemImagePkg, msg.DeviceProfileID)

	case ui.ConfirmDeleteSimulatorMsg:
		m.deleteAlert = nil
		return m, m.deleteDeviceCmd(msg.Device)

	case ui.ConfirmDeleteAppMsg:
		m.deleteAppAlert = nil
		return m, m.deleteAppCmd(msg.Device, msg.App)

	case deleteAppResultMsg:
		if msg.err != nil {
			m.errs = append(m.errs, msg.err)
			return m, m.setStatus("delete app failed: "+errPreview(msg.err), ui.StatusErr)
		}
		if m.logStream != nil && m.logDeviceID == msg.deviceID && m.logBundleID == msg.bundleID {
			m.logStream.Stop()
			m.logStream = nil
			m.logBundleID = ""
			m.logDeviceID = ""
			m.mainPane.SetLogBundle("")
		}
		sel := m.sidebar.SelectedDevice()
		var reloadCmd tea.Cmd
		if sel != nil && sel.ID == msg.deviceID {
			reloadCmd = m.loadAppsCmd(*sel)
		}
		return m, tea.Batch(
			reloadCmd,
			m.setStatus("deleted "+msg.appLabel, ui.StatusOk),
		)

	case deviceTypesMsg:
		if msg.err != nil {
			m.errs = append(m.errs, msg.err)
			if m.createIOSModal != nil {
				m.createIOSModal.SetDeviceTypes(nil)
			}
			return m, m.setStatus("device types: "+errPreview(msg.err), ui.StatusErr)
		}
		if m.createIOSModal != nil {
			m.createIOSModal.SetDeviceTypes(msg.types)
		}
		return m, nil

	case runtimesMsg:
		if msg.err != nil {
			m.errs = append(m.errs, msg.err)
			if m.createIOSModal != nil {
				m.createIOSModal.SetRuntimes(nil)
			}
			return m, m.setStatus("runtimes: "+errPreview(msg.err), ui.StatusErr)
		}
		if m.createIOSModal != nil {
			m.createIOSModal.SetRuntimes(msg.runtimes)
		}
		return m, nil

	case androidSystemImagesMsg:
		if msg.err != nil {
			m.errs = append(m.errs, msg.err)
			if m.createAndModal != nil {
				m.createAndModal.SetSystemImages(nil)
			}
			return m, m.setStatus("system images: "+errPreview(msg.err), ui.StatusErr)
		}
		if m.createAndModal != nil {
			m.createAndModal.SetSystemImages(msg.images)
		}
		return m, nil

	case androidDeviceProfilesMsg:
		if msg.err != nil {
			// Device profiles are optional; log but don't block
			if m.createAndModal != nil {
				m.createAndModal.SetDeviceProfiles(nil)
			}
			return m, nil
		}
		if m.createAndModal != nil {
			m.createAndModal.SetDeviceProfiles(msg.profiles)
		}
		return m, nil

	case createSimulatorResultMsg:
		if msg.err != nil {
			m.errs = append(m.errs, msg.err)
			return m, m.setStatus("create failed: "+errPreview(msg.err), ui.StatusErr)
		}
		m.loading = true
		return m, tea.Batch(
			m.fetchDevicesCmd(),
			m.setStatus("created "+msg.name, ui.StatusOk),
		)

	case createAndroidEmulatorResultMsg:
		if msg.err != nil {
			m.errs = append(m.errs, msg.err)
			return m, m.setStatus("create failed: "+errPreview(msg.err), ui.StatusErr)
		}
		m.loading = true
		return m, tea.Batch(
			m.fetchDevicesCmd(),
			m.setStatus("created "+msg.name, ui.StatusOk),
		)

	case deleteSimulatorResultMsg:
		if msg.err != nil {
			m.errs = append(m.errs, msg.err)
			return m, m.setStatus("delete failed: "+errPreview(msg.err), ui.StatusErr)
		}
		m.loading = true
		return m, tea.Batch(
			m.fetchDevicesCmd(),
			m.setStatus("deleted "+msg.name, ui.StatusOk),
		)

	default:
		// Forward unrecognised messages (e.g. private filepicker readDirMsg) to
		// whichever overlay is active so embedded components can process them.
		if m.installAppModal != nil {
			updated, cmd := m.installAppModal.Update(msg)
			m.installAppModal = &updated
			return m, cmd
		}
		if m.dbViewerModal != nil {
			updated, cmd := m.dbViewerModal.Update(msg)
			m.dbViewerModal = &updated
			return m, cmd
		}
	}

	return m, nil
}
