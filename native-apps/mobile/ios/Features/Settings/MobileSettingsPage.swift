import SwiftUI

/// Тохиргоо — ширээний `SettingsSheetView`-ийн гар дээрх хувилбар.
///
/// Ширээнийхээс ХАСАГДСАН зүйлс нь утсан дээр утгагүй байдгаараа: нэвтрэхэд
/// автоматаар асах (login item), арын горим, ESIGN токен (ws гүүр нь ижил
/// машин дээрх хөтчид зориулсан). Үлдсэн нь ижил: төрх, хэл, сервер, тухай.
struct MobileSettingsPage: View {
    @EnvironmentObject private var appState: AppState
    @ObservedObject private var loc = LocalizationService.shared
    @AppStorage("ui.theme") private var theme: String = "system"
    @AppStorage(AppConfig.baseURLKey) private var apiURL: String = ""

    @State private var confirmLogout = false
    @FocusState private var serverFocused: Bool

    var body: some View {
        MobilePage(title: loc.t("Nav_Settings"), subtitle: nil) {
            themeCard
            languageCard
            serverCard
            aboutCard
            logoutButton
        }
        .preferredColorScheme(colorScheme)
    }

    private var themeCard: some View {
        BrandCard {
            Text(loc.t("Settings_Theme"))
                .font(Theme.TypeScale.calloutBold)
                .foregroundStyle(Theme.fg1)
            Picker("", selection: $theme) {
                Text(loc.t("Settings_Theme_System")).tag("system")
                Text(loc.t("Settings_Theme_Light")).tag("light")
                Text(loc.t("Settings_Theme_Dark")).tag("dark")
            }
            .pickerStyle(.segmented)
            .labelsHidden()
        }
    }

    private var languageCard: some View {
        BrandCard {
            Text(loc.t("Settings_Language"))
                .font(Theme.TypeScale.calloutBold)
                .foregroundStyle(Theme.fg1)
            ForEach(AppLanguage.allCases) { lang in
                Button {
                    loc.setLanguage(lang)
                } label: {
                    HStack {
                        Text(lang.displayName)
                            .font(Theme.TypeScale.body)
                            .foregroundStyle(Theme.fg1)
                        Spacer()
                        if loc.language == lang {
                            Image(systemName: "checkmark")
                                .font(.system(size: 12, weight: .bold))
                                .foregroundStyle(Theme.Brand.primary)
                        }
                    }
                    .contentShape(Rectangle())
                    .padding(.vertical, Theme.Space.sm)
                }
                .buttonStyle(.plain)
                if lang != AppLanguage.allCases.last {
                    Rectangle().fill(Theme.divider).frame(height: 1)
                }
            }
        }
    }

    /// Сервер солих нь ЗӨВХӨН туршилтад. Хоосон бол энэ байрлуулалтын
    /// гарын шугам (`mobile.nexus.gerege.mn`) — `AppConfig.baseURL`.
    private var serverCard: some View {
        BrandCard {
            Text(loc.pick("Сервер", "Server", "Сервер", "服务器"))
                .font(Theme.TypeScale.calloutBold)
                .foregroundStyle(Theme.fg1)
            BrandInputCard(leadingIcon: "network", isFocused: serverFocused) {
                TextField(AppConfig.baseURL, text: $apiURL)
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
                    .keyboardType(.URL)
                    .font(Theme.TypeScale.mono)
                    .foregroundStyle(Theme.fg1)
                    .focused($serverFocused)
            }
            Text(loc.pick("Хоосон бол анхдагч: \(AppConfig.baseURL)",
                          "Empty means the default: \(AppConfig.baseURL)",
                          "Пусто — по умолчанию: \(AppConfig.baseURL)",
                          "留空则使用默认值：\(AppConfig.baseURL)"))
                .font(Theme.TypeScale.caption2)
                .foregroundStyle(Theme.fg3)
        }
    }

    private var aboutCard: some View {
        BrandCard {
            Text(loc.pick("Тухай", "About", "О приложении", "关于"))
                .font(Theme.TypeScale.calloutBold)
                .foregroundStyle(Theme.fg1)
            MobileField(label: loc.pick("Хувилбар", "Version", "Версия", "版本"),
                        value: "\(AppConfig.brandName) v\(version)", mono: true)
            MobileField(label: loc.pick("Холбогдож буй хост", "Connected host",
                                        "Подключённый хост", "连接的主机"),
                        value: URL(string: AppConfig.baseURL)?.host ?? AppConfig.baseURL, mono: true)
            Link(destination: URL(string: "mailto:support@eidmongol.mn")!) {
                HStack(spacing: 6) {
                    Image(systemName: "headphones").font(.system(size: 12))
                    Text(loc.t("Nav_Support")).font(Theme.TypeScale.footnoteBold)
                }
                .foregroundStyle(Theme.Brand.primary)
            }
        }
    }

    private var logoutButton: some View {
        BrandSecondaryButton(title: loc.t("Nav_Logout"),
                             systemImage: "rectangle.portrait.and.arrow.right",
                             tone: Theme.debit) {
            confirmLogout = true
        }
        .confirmationDialog(loc.t("Nav_Logout"), isPresented: $confirmLogout, titleVisibility: .visible) {
            Button(loc.t("Nav_Logout"), role: .destructive) { appState.logout() }
            Button(loc.t("Login_Cancel"), role: .cancel) {}
        }
    }

    private var colorScheme: ColorScheme? {
        switch theme {
        case "light": return .light
        case "dark":  return .dark
        default:      return nil
        }
    }

    private var version: String {
        Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "1.0.0"
    }
}
