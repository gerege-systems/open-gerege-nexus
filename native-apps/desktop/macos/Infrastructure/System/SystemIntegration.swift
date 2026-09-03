import Foundation
import ServiceManagement
import AppKit

/// Wraps `SMAppService.mainApp` (macOS 13+) so the app can register or
/// unregister itself as a Login Item — "Нэвтрэхэд автоматаар нээх". The
/// published `launchAtLogin` always reflects the real system status, even if a
/// call throws (e.g. the user revoked the item in System Settings > Login Items).
@MainActor
final class LoginItemManager: ObservableObject {
    static let shared = LoginItemManager()

    @Published private(set) var launchAtLogin: Bool

    private init() {
        launchAtLogin = SMAppService.mainApp.status == .enabled
    }

    /// Toggle the Login Item, then re-sync `launchAtLogin` to the real system
    /// state so the UI never drifts from what macOS actually recorded.
    func setLaunchAtLogin(_ enabled: Bool) {
        defer { launchAtLogin = SMAppService.mainApp.status == .enabled }
        do {
            if enabled {
                if SMAppService.mainApp.status != .enabled {
                    try SMAppService.mainApp.register()
                }
            } else if SMAppService.mainApp.status == .enabled {
                try SMAppService.mainApp.unregister()
            }
        } catch {
            // The `defer` re-sync keeps the toggle honest if this fails.
        }
    }

    /// Re-read the system status (the user may have changed it externally).
    func refresh() {
        launchAtLogin = SMAppService.mainApp.status == .enabled
    }
}

/// Menu-bar (background) vs. regular Dock app. `.accessory` removes the Dock
/// icon and the app menu so the app lives only in the status bar; `.regular`
/// is the normal windowed app. The MenuBarExtra keeps the process alive in
/// either mode, so closing the window never quits the app.
enum AppActivation {
    static let backgroundModeKey = "ui.backgroundMode"

    @MainActor
    static func apply(background: Bool) {
        NSApp.setActivationPolicy(background ? .accessory : .regular)
        if !background {
            NSApp.activate(ignoringOtherApps: true)
        }
    }
}
