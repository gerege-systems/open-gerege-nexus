import AppKit

/// Owns the AppKit-level lifecycle bits SwiftUI's `App` can't express:
/// applying the menu-bar (accessory) activation policy at launch, and bringing
/// the main window back when the Dock icon is clicked.
final class AppDelegate: NSObject, NSApplicationDelegate {

    /// Set the policy before the first window is created so a background launch
    /// (e.g. at login) never flashes a Dock icon.
    func applicationWillFinishLaunching(_ notification: Notification) {
        if UserDefaults.standard.bool(forKey: AppActivation.backgroundModeKey) {
            NSApp.setActivationPolicy(.accessory)
        }
    }

    /// In background mode, start hidden in the menu bar instead of popping the
    /// auto-opened WindowGroup window.
    func applicationDidFinishLaunching(_ notification: Notification) {
        guard UserDefaults.standard.bool(forKey: AppActivation.backgroundModeKey) else { return }
        DispatchQueue.main.async {
            NSApp.windows.filter { $0.canBecomeMain }.forEach { $0.close() }
        }
    }

    /// Dock-icon click with no visible window → let AppKit reopen the window.
    func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows flag: Bool) -> Bool {
        if !flag {
            NSApp.activate(ignoringOtherApps: true)
        }
        return true
    }
}
