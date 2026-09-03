import SwiftUI
import AppKit

/// The status-bar (MenuBarExtra) menu. Mirrors the most-used actions so the app
/// stays usable while it lives in the menu bar / background: open the window,
/// log out, toggle launch-at-login and menu-bar mode, and quit.
struct MenuBarContent: View {
    @EnvironmentObject private var appState: AppState
    @ObservedObject private var loc = LocalizationService.shared
    @ObservedObject private var loginItem = LoginItemManager.shared
    @AppStorage(AppActivation.backgroundModeKey) private var backgroundMode = false
    @Environment(\.openWindow) private var openWindow

    var body: some View {
        statusLine

        Divider()

        Button(loc.t("MenuBar_Open")) { openMainWindow() }
            .keyboardShortcut("o")

        if appState.screen == .dashboard {
            Button(loc.t("MenuBar_Logout")) {
                appState.logout()
                openMainWindow()
            }
        }

        Divider()

        Toggle(loc.t("Settings_Startup_LaunchAtLogin"), isOn: Binding(
            get: { loginItem.launchAtLogin },
            set: { loginItem.setLaunchAtLogin($0) }
        ))
        Toggle(loc.t("Settings_Startup_BackgroundMode"), isOn: $backgroundMode)
            .onChange(of: backgroundMode) { _, on in AppActivation.apply(background: on) }

        Divider()

        Button(loc.t("MenuBar_Quit")) { NSApp.terminate(nil) }
            .keyboardShortcut("q")
    }

    @ViewBuilder private var statusLine: some View {
        if appState.screen == .dashboard {
            Text(appState.fullName.isEmpty ? loc.t("MenuBar_SignedIn") : appState.fullName)
        } else {
            Text(loc.t("MenuBar_SignedOut"))
        }
    }

    /// Surface the main window even when running as a menu-bar accessory app.
    private func openMainWindow() {
        NSApp.activate(ignoringOtherApps: true)
        openWindow(id: GeregeApp.mainWindowID)
    }
}
