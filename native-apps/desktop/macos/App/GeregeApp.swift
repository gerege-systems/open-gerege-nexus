import SwiftUI
import Sparkle

@main
struct GeregeApp: App {

    /// Stable identifier for the main WindowGroup so the menu-bar extra can
    /// re-open it via `openWindow(id:)` even after it was closed.
    static let mainWindowID = "main"

    @NSApplicationDelegateAdaptor(AppDelegate.self) private var appDelegate
    @StateObject private var appState = AppState()
    @StateObject private var tokenManager = TokenManager()
    @StateObject private var localization = LocalizationService.shared
    @State private var showWhatsNew = false

    private let updaterController = SPUStandardUpdaterController(
        startingUpdater: true,
        updaterDelegate: nil,
        userDriverDelegate: nil
    )

    init() {
        // SEC-2: launch-time anti-tamper. Denies debugger attach and, in
        // Release, terminates if the binary is debugged / injected / modified /
        // run as root. No-op in DEBUG so the Xcode dev loop is unaffected.
        SecurityGuard.enforce()
    }

    var body: some Scene {
        WindowGroup(id: Self.mainWindowID) {
            ContentView()
                .environmentObject(appState)
                .environmentObject(tokenManager)
                .environmentObject(localization)
                // Араб хэл дээр бүх зохиомжийг баруунаас зүүн болгоно (docs/I18N.md — RTL).
                .environment(\.layoutDirection, localization.language.isRTL ? .rightToLeft : .leftToRight)
                .frame(minWidth: 560, minHeight: 520)
                .sheet(isPresented: $showWhatsNew) {
                    WhatsNewView()
                }
                .onAppear {
                    appState.restoreSessionFromKeychain()
                    if WhatsNewChecker.shouldShow {
                        showWhatsNew = true
                        WhatsNewChecker.markAsSeen()
                    }
                }
        }
        .windowStyle(.titleBar)
        .defaultSize(width: 1450, height: 800)   // дэлгэцийн эталон зураг 1436×782 → 50px сүлжээнд
        .commands {
            CommandGroup(replacing: .newItem) {}
            CommandGroup(after: .appInfo) {
                CheckForUpdatesView(updater: updaterController.updater)
                Button("Шинэчлэлтийн түүх...") {
                    showWhatsNew = true
                }
            }
        }

        Settings {
            SettingsView()
        }

        // Menu-bar (status bar) presence — keeps the app alive in the
        // background and lets the user re-open the window or quit from there.
        MenuBarExtra(AppConfig.brandName, systemImage: "checkmark.shield.fill") {
            MenuBarContent()
                .environmentObject(appState)
                .environmentObject(localization)
        }
    }
}

/// Sparkle "Шинэчлэлт шалгах" цэс.
struct CheckForUpdatesView: View {
    @ObservedObject private var checkForUpdatesViewModel: CheckForUpdatesViewModel
    let updater: SPUUpdater

    init(updater: SPUUpdater) {
        self.updater = updater
        self.checkForUpdatesViewModel = CheckForUpdatesViewModel(updater: updater)
    }

    var body: some View {
        Button("Шинэчлэлт шалгах...", action: updater.checkForUpdates)
            .disabled(!checkForUpdatesViewModel.canCheckForUpdates)
    }
}

/// Sparkle update button state.
final class CheckForUpdatesViewModel: ObservableObject {
    @Published var canCheckForUpdates = false

    init(updater: SPUUpdater) {
        updater.publisher(for: \.canCheckForUpdates)
            .assign(to: &$canCheckForUpdates)
    }
}
