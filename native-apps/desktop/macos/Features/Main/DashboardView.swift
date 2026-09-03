import SwiftUI
import Sparkle
import Combine

/// Windows `MainWindow.xaml` shell port —
/// — 264px dark navy sidebar (logo top, NavigationView menu, footer bottom)
/// — 60px header (page title left + profile dropdown right)
/// — `EidSurface` content area
///
/// Routes mirror Windows ShellViewModel: home / dashboard / organizations / tokens / verify
/// plus footer Settings + Logout (Windows `__logout__` sentinel).
struct DashboardView: View {
    @EnvironmentObject private var appState: AppState
    @EnvironmentObject private var tokenManager: TokenManager
    @ObservedObject private var loc = LocalizationService.shared

    /// nil = follow the window width; non-nil = user pinned the sidebar
    /// open/closed via the header toggle.
    @State private var sidebarCollapsedOverride: Bool? = nil
    /// Ажлын мужийн навигаци — толгой хэсгийн буцах товч үүнээс уншина.
    @ObservedObject private var workArea = WorkAreaNavigation.shared

    /// Below this content width the sidebar auto-collapses to an icon rail so
    /// the page content keeps a usable width on small windows.
    private let collapseBreakpoint: CGFloat = 820

    /// Key of the icon-rail item currently hovered (collapsed sidebar only) —
    /// drives the name popover. `DashboardTab.rawValue` for nav rows,
    /// `"footer:<label>"` for the settings/logout footer rows.
    @State private var hoveredItemKey: String? = nil

    var body: some View {
        GeometryReader { proxy in
            let collapsed = sidebarCollapsedOverride ?? (proxy.size.width < collapseBreakpoint)
            VStack(spacing: 0) {
                HStack(spacing: 0) {
                    sidebar(collapsed: collapsed)
                        .frame(width: collapsed ? 68 : 264)
                    VStack(spacing: 0) {
                        header(collapsed: collapsed)
                            .frame(height: 60)
                        detailView
                            .frame(maxWidth: .infinity, maxHeight: .infinity)
                            .background(Color.eidSurface)
                    }
                }
                // Цонхны бүтэн өргөнд — хажуугийн цэсний ДОТОР биш, ДООР.
                AppStatusFooter()
            }
            .animation(.easeInOut(duration: 0.2), value: collapsed)
        }
        .background(Color.eidSurface)
        .sheet(isPresented: $appState.settingsPresented) {
            SettingsSheetView()
        }
        // SEC-3: re-check integrity after login (scattered guard — a single
        // patched launch check should not be enough to defeat protection).
        .onAppear { SecurityGuard.enforce() }
    }

    // MARK: - Sidebar (dark navy #0F172A)

    private func sidebar(collapsed: Bool) -> some View {
        VStack(alignment: .leading, spacing: 0) {
            sidebarHeader(collapsed: collapsed)

            ScrollView(showsIndicators: false) {
                VStack(alignment: .leading, spacing: 2) {
                    ForEach(DashboardTab.mainNav, id: \.self) { tab in
                        sidebarItem(tab: tab, collapsed: collapsed)
                    }
                }
                .padding(.horizontal, collapsed ? 8 : 10)
            }

            Spacer(minLength: 8)

            sidebarFooter(collapsed: collapsed)
        }
        .frame(maxHeight: .infinity)
        .background(Color.sidebarBackground)
    }

    // Sidebar top — Logo + brand title (Windows TitleBar Title + Subtitle port).
    private func sidebarHeader(collapsed: Bool) -> some View {
        HStack(spacing: 10) {
            Image("Logo")
                .resizable()
                .aspectRatio(contentMode: .fit)
                .frame(width: 34, height: 34)
                .clipShape(RoundedRectangle(cornerRadius: 8, style: .continuous))
            if !collapsed {
                VStack(alignment: .leading, spacing: 2) {
                    Text(AppConfig.brandName)
                        .font(.system(size: 15, weight: .semibold))
                        .foregroundStyle(.white)
                        .fixedSize()
                    Text("Digital Sovereignty")
                        .font(.system(size: 10))
                        .foregroundStyle(Color.sidebarMutedText)
                        .fixedSize()
                }
                Spacer(minLength: 0)
            }
        }
        .frame(maxWidth: .infinity, alignment: collapsed ? .center : .leading)
        .padding(.horizontal, collapsed ? 0 : 16)
        .padding(.top, 20)
        .padding(.bottom, 24)
    }

    @ViewBuilder
    private func sidebarItem(tab: DashboardTab, collapsed: Bool) -> some View {
        let selected = appState.selectedTab == tab
        Button {
            appState.selectedTab = tab
        } label: {
            HStack(spacing: 12) {
                Image(systemName: tab.icon)
                    .font(.system(size: 15))
                    .frame(width: 20)
                    .overlay(alignment: .topTrailing) {
                        // Collapsed: float the token dot on the icon since the
                        // label (and its trailing dot) is hidden.
                        if collapsed && tab == .tokens {
                            Circle()
                                .fill(tokenManager.isTokenPresent ? Color.eidSuccess
                                      : Color.sidebarMutedText.opacity(0.4))
                                .frame(width: 6, height: 6)
                                .offset(x: 4, y: -2)
                        }
                    }
                if !collapsed {
                    Text(loc.t(tab.labelKey))
                        .font(.system(size: 15, weight: selected ? .semibold : .medium))
                    Spacer(minLength: 0)
                    // Live token indicator on the Tokens row (Windows hint)
                    if tab == .tokens {
                        Circle()
                            .fill(tokenManager.isTokenPresent
                                  ? Color.eidSuccess
                                  : Color.sidebarMutedText.opacity(0.4))
                            .frame(width: 7, height: 7)
                    }
                }
            }
            .foregroundStyle(selected ? .white : Color.sidebarMutedText)
            .padding(.horizontal, collapsed ? 0 : 12)
            .padding(.vertical, 9)
            .frame(maxWidth: .infinity, alignment: collapsed ? .center : .leading)
            .background(
                RoundedRectangle(cornerRadius: 8, style: .continuous)
                    .fill(selected ? Color.eidAccent : Color.clear)
            )
            // Make the WHOLE row (incl. the empty space after the label)
            // hit-testable, not just the text/icon glyphs.
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityLabel(loc.t(tab.labelKey))
        .onHover { hovering in
            guard collapsed else { return }
            if hovering { hoveredItemKey = tab.rawValue }
            else if hoveredItemKey == tab.rawValue { hoveredItemKey = nil }
        }
        .popover(isPresented: hoverBinding(for: tab.rawValue, collapsed: collapsed),
                 arrowEdge: .trailing) {
            hoverLabel(loc.t(tab.labelKey))
        }
    }

    /// Show the item's name immediately on hover while the sidebar is an icon
    /// rail (replaces the slow native tooltip).
    private func hoverBinding(for key: String, collapsed: Bool) -> Binding<Bool> {
        Binding(
            get: { collapsed && hoveredItemKey == key },
            set: { if !$0 { hoveredItemKey = nil } }
        )
    }

    private func hoverLabel(_ text: String) -> some View {
        Text(text)
            .font(.system(size: 13, weight: .medium))
            .foregroundStyle(Color.textPrimary)
            .padding(.horizontal, 12)
            .padding(.vertical, 7)
    }

    // Sidebar bottom — Тохиргоо, Гарах, доор нь аппын мэдээллийн зурвас.
    private func sidebarFooter(collapsed: Bool) -> some View {
        VStack(alignment: .leading, spacing: 6) {
            // Settings (Windows footer item "Тохиргоо")
            footerButton(icon: "gearshape", label: loc.t("Nav_Settings"), collapsed: collapsed) {
                appState.settingsPresented = true
            }
            // Logout (Windows footer __logout__ sentinel)
            footerButton(icon: "rectangle.portrait.and.arrow.right",
                         label: loc.t("Nav_Logout"), collapsed: collapsed) {
                appState.logout()
            }

        }
        .padding(.horizontal, collapsed ? 8 : 14)
        .padding(.bottom, 14)
    }

    private func footerButton(icon: String, label: String, collapsed: Bool,
                              action: @escaping () -> Void) -> some View {
        Button(action: action) {
            HStack(spacing: 12) {
                Image(systemName: icon)
                    .font(.system(size: 13))
                    .frame(width: 20)
                if !collapsed {
                    Text(label).font(.system(size: 13))
                    Spacer()
                }
            }
            .foregroundStyle(Color.sidebarMutedText)
            .padding(.horizontal, collapsed ? 0 : 12)
            .padding(.vertical, 8)
            .frame(maxWidth: .infinity, alignment: collapsed ? .center : .leading)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityLabel(label)
        .onHover { hovering in
            guard collapsed else { return }
            let key = "footer:\(label)"
            if hovering { hoveredItemKey = key }
            else if hoveredItemKey == key { hoveredItemKey = nil }
        }
        .popover(isPresented: hoverBinding(for: "footer:\(label)", collapsed: collapsed),
                 arrowEdge: .trailing) {
            hoverLabel(label)
        }
    }

    // MARK: - Header (page title left + profile dropdown right)

    private func header(collapsed: Bool) -> some View {
        HStack(spacing: 12) {
            // Sidebar collapse/expand toggle (pins the override).
            Button {
                sidebarCollapsedOverride = !collapsed
            } label: {
                Image(systemName: "sidebar.left")
                    .font(.system(size: 14, weight: .medium))
                    .foregroundStyle(Color.textSecondary)
                    .frame(width: 28, height: 28)
                    .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            .help(collapsed ? "Хажуугийн цэсийг дэлгэх" : "Хажуугийн цэсийг хураах")

            // Ажлын мужийн буцах товч. Зөвхөн платформын дэлгэц дээр, зөвхөн
            // буцах түүх байх үед — өөр дэлгэцүүд нь native тул тэдэнд
            // хамаагүй, эхний хуудсан дээр бол буцах газар байхгүй.
            if appState.selectedTab == .platform, workArea.canGoBack {
                Button {
                    workArea.goBack()
                } label: {
                    Image(systemName: "chevron.backward")
                        .font(.system(size: 14, weight: .semibold))
                        .foregroundStyle(Color.textSecondary)
                        .frame(width: 28, height: 28)
                        .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
                .help("Буцах")
                .keyboardShortcut("[", modifiers: .command)
            }

            Text(loc.t(appState.selectedTab.labelKey))
                .font(.system(size: 16, weight: .semibold))
                .foregroundStyle(Color.textPrimary)
                .lineLimit(1)

            Spacer()

            // Profile menu (Windows DashboardPage profile dropdown)
            Menu {
                Button {
                    appState.selectedTab = .dashboard
                } label: {
                    Label(loc.t("Nav_Dashboard"), systemImage: "rectangle.3.group")
                }
                Button {
                    appState.settingsPresented = true
                } label: {
                    Label(loc.t("Nav_Settings"), systemImage: "gearshape")
                }
                Divider()
                Button(role: .destructive) {
                    appState.logout()
                } label: {
                    Label(loc.t("Nav_Logout"), systemImage: "rectangle.portrait.and.arrow.right")
                }
            } label: {
                HStack(spacing: 10) {
                    VStack(alignment: .trailing, spacing: 2) {
                        Text(appState.fullName.isEmpty ? loc.t("Common_User") : appState.fullName)
                            .font(.system(size: 13, weight: .semibold))
                            .foregroundStyle(Color.textPrimary)
                        Text(appState.nationalID)
                            .font(.system(size: 11, design: .monospaced))
                            .foregroundStyle(Color.textSecondary)
                    }
                    Image(systemName: "chevron.down")
                        .font(.system(size: 10))
                        .foregroundStyle(Color.textSecondary)
                }
            }
            .menuStyle(.borderlessButton)
            .fixedSize()
        }
        .padding(.horizontal, 24)
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(Color.eidCardBackground)
        .overlay(alignment: .bottom) {
            Rectangle().fill(Color.eidCardStroke).frame(height: 1)
        }
    }

    // MARK: - Detail content (Windows Frame x:Name="ContentFrame")

    @ViewBuilder
    private var detailView: some View {
        switch appState.selectedTab {
        // mainNav
        case .dashboard:     DashboardPageView()
        case .id:            MyIdView()
        case .logs:          LogsView()
        case .devices:       DevicesView()
        case .security:      SecurityView()
        case .organizations: OrganizationsView()
        case .children:      ChildrenView()
        case .sign:          SignView()
        case .tokens:        TokensView()
        case .tokenReader:   TokenScanView()
        // Desktop-only tools (nav-аас нуусан)
        case .platform:      PlatformView()
        case .home:          HomeView()
        case .verify:        VerifyView()
        }
    }
}

// MARK: - App status footer

/// Цонхны ёроолын статус мөр — хажуугийн цэс, контент ХОЁУЛАНГИЙН доогуур.
///
/// Өмнө нь энд «Тусламж / Нууцлал / Бүртгэл түгжих» гэсэн гурван цэс байв.
/// Гурвуулаа хасагдсан: түгжих нь «Гарах»-ийн давхардал, нөгөө хоёр нь нэг
/// уншаад дахин нээхгүй хуудсууд байсан бөгөөд цэсний мөр бүр өдөр бүр
/// хэрэглэдэг зүйлсийг доош түлхэж байв.
///
/// Оронд нь хүн ҮНЭНДЭЭ хардаг зүйлс: хувилбар, шинэчлэлтийн төлөв, аль
/// сервертэй ярьж байгаа, тусламжийн хаяг. Хажуугийн цэсний дотор биш ЭНД
/// байгаа шалтгаан: эдгээр нь тухайн цэсийнх биш, АППЫНХ — цэс хураагдсан ч,
/// платформын webview дүүрэн дэлгэц эзэлсэн ч ижил хэвээр харагдана.
struct AppStatusFooter: View {
    @ObservedObject private var vm = UpdateViewModel.shared
    @ObservedObject private var loc = LocalizationService.shared

    private static let supportEmail = "support@eidmongol.mn"

    var body: some View {
        HStack(spacing: 10) {
            Text(AppConfig.brandName)
                .font(.system(size: 11, weight: .semibold))
                .foregroundStyle(Color.textSecondary)

            Text("v\(version)")
                .font(.system(size: 10, weight: .medium, design: .monospaced))
                .foregroundStyle(Color.textSecondary.opacity(0.9))
                .padding(.horizontal, 6).padding(.vertical, 2)
                .background(Color.eidCardStroke.opacity(0.45), in: Capsule())

            divider

            // Аль сервертэй ярьж байгаа нь ширээний клиентэд чухал: Тохиргооноос
            // хаяг сольсон хүн юу ч болоогүй мэт харагддаг байв.
            Label(host, systemImage: "network")
                .font(.system(size: 10, design: .monospaced))
                .foregroundStyle(Color.textSecondary.opacity(0.75))
                .lineLimit(1).truncationMode(.middle)
                .help(AppConfig.baseURL)

            Spacer(minLength: 8)

            updateStatus

            divider

            Link(destination: URL(string: "mailto:\(Self.supportEmail)")!) {
                Label(loc.t("Nav_Support"), systemImage: "headphones")
                    .font(.system(size: 10))
                    .foregroundStyle(Color.textSecondary.opacity(0.8))
            }
            .buttonStyle(.plain)
            .help(Self.supportEmail)
        }
        .padding(.horizontal, 14)
        .frame(height: 28)
        .frame(maxWidth: .infinity)
        .background(alignment: .top) {
            // Дээд талын үсэн зураас — контентын ирмэгийг заана.
            Color.eidCardStroke.frame(height: 1)
        }
        .background(Color.eidSurface)
    }

    private var divider: some View {
        Color.eidCardStroke.frame(width: 1, height: 12)
    }

    /// Шинэчлэлт байвал ТОВЧ, байхгүй бол сүүлд шалгасан хугацаа.
    @ViewBuilder private var updateStatus: some View {
        if vm.updateAvailable {
            Button { vm.checkForUpdates() } label: {
                Label(loc.t("Update_Available"), systemImage: "arrow.down.circle.fill")
                    .font(.system(size: 10, weight: .semibold))
                    .foregroundStyle(Color.eidAccent)
            }
            .buttonStyle(.plain)
        } else if let last = vm.lastCheckDate {
            Text("\(loc.t("Update_LastChecked")): \(last)")
                .font(.system(size: 10))
                .foregroundStyle(Color.textSecondary.opacity(0.6))
        }
    }

    private var version: String {
        let short = Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "1.0.0"
        let build = Bundle.main.infoDictionary?["CFBundleVersion"] as? String
        // Build дугаар нь short-оос ЯЛГААТАЙ үед л утгатай.
        return (build.map { $0 != short ? "\(short) (\($0))" : short }) ?? short
    }

    private var host: String {
        URL(string: AppConfig.baseURL)?.host ?? AppConfig.baseURL
    }
}

/// Sparkle update state tracking.
final class UpdateViewModel: ObservableObject {
    static let shared = UpdateViewModel()

    @Published var updateAvailable = false
    @Published var lastCheckDate: String?

    private var updater: SPUUpdater?
    private var cancellables = Set<AnyCancellable>()

    private init() { loadLastCheckDate() }

    func configure(updater: SPUUpdater) {
        self.updater = updater
        updater.publisher(for: \.canCheckForUpdates)
            .sink { [weak self] _ in self?.checkUpdateState() }
            .store(in: &cancellables)
    }

    func checkForUpdates() {
        updater?.checkForUpdates()
        saveCheckDate()
    }

    func checkUpdateState() {
        if let lastUpdate = updater?.lastUpdateCheckDate {
            let formatter = DateFormatter()
            formatter.dateFormat = "MM/dd HH:mm"
            lastCheckDate = formatter.string(from: lastUpdate)
        }
    }

    private func saveCheckDate() {
        let formatter = DateFormatter()
        formatter.dateFormat = "MM/dd HH:mm"
        lastCheckDate = formatter.string(from: Date())
        UserDefaults.standard.set(lastCheckDate, forKey: "lastUpdateCheck")
    }

    private func loadLastCheckDate() {
        lastCheckDate = UserDefaults.standard.string(forKey: "lastUpdateCheck")
    }
}
