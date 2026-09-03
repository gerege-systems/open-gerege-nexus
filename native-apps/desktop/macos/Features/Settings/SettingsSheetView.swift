import SwiftUI

/// Windows `SettingsPage` port — Тохиргооны цонх (sidebar footer-ээс нээгдэнэ).
/// Theme + Language picker. macOS-ийн системийн `Settings { ... }` Scene-ээс
/// тусдаа sheet — Windows-тэй ижил sidebar route-аас нээгдэх боломжтой.
struct SettingsSheetView: View {
    @Environment(\.dismiss) private var dismiss
    @ObservedObject private var loc = LocalizationService.shared
    @AppStorage("ui.theme") private var theme: String = "system"
    @AppStorage(AppConfig.baseURLKey) private var apiURL: String = ""
    @ObservedObject private var loginItem = LoginItemManager.shared
    @AppStorage(AppActivation.backgroundModeKey) private var backgroundMode: Bool = false
    @AppStorage(AppState.esignUseTokenKey) private var esignUseToken: Bool = false

    var body: some View {
        VStack(spacing: 0) {
            header
            ScrollView {
                VStack(alignment: .leading, spacing: 20) {
                    themeCard
                    languageCard
                    startupCard
                    esignCard
                    apiCard
                    aboutCard
                }
                .padding(.horizontal, Space.pageHoriz)
                .padding(.top, Space.pageTop)
                .padding(.bottom, Space.pageBottom)
            }
            .background(Color.eidSurface)
        }
        .frame(width: 540, height: 580)
        .onChange(of: theme) { _, newValue in applyTheme(newValue) }
        .onAppear { applyTheme(theme) }
    }

    // MARK: - Header bar

    private var header: some View {
        HStack {
            Text(loc.t("Settings_Title"))
                .font(.eidPageTitle)
                .foregroundStyle(Color.textPrimary)
            Spacer()
            Button(loc.t("Settings_Close")) { dismiss() }
                .keyboardShortcut(.cancelAction)
                .buttonStyle(.secondary)
        }
        .padding(.horizontal, 20)
        .padding(.vertical, 14)
        .background(Color.eidCardBackground)
        .overlay(alignment: .bottom) {
            Rectangle().fill(Color.eidCardStroke).frame(height: 1)
        }
    }

    // MARK: - Theme

    private var themeCard: some View {
        AppCard {
            VStack(alignment: .leading, spacing: 10) {
                Text(loc.t("Settings_Theme"))
                    .font(.eidSectionTitle)
                    .foregroundStyle(Color.textPrimary)
                Picker("", selection: $theme) {
                    Text(loc.t("Settings_Theme_System")).tag("system")
                    Text(loc.t("Settings_Theme_Light")).tag("light")
                    Text(loc.t("Settings_Theme_Dark")).tag("dark")
                }
                .pickerStyle(.segmented)
                .labelsHidden()
            }
        }
    }

    // MARK: - Language

    private var languageCard: some View {
        AppCard {
            VStack(alignment: .leading, spacing: 10) {
                Text(loc.t("Settings_Language"))
                    .font(.eidSectionTitle)
                    .foregroundStyle(Color.textPrimary)
                Picker("", selection: Binding(
                    get: { loc.language },
                    set: { loc.setLanguage($0) }
                )) {
                    ForEach(AppLanguage.allCases) { language in
                        Text(language.displayName).tag(language)
                    }
                }
                .pickerStyle(.segmented)
                .labelsHidden()
                Text(loc.t("Settings_Language_Hint"))
                    .labelStyle()
            }
        }
    }

    // MARK: - Startup (login item + menu-bar background mode)

    private var startupCard: some View {
        AppCard {
            VStack(alignment: .leading, spacing: 10) {
                Text(loc.t("Settings_Startup_Title"))
                    .font(.eidSectionTitle)
                    .foregroundStyle(Color.textPrimary)
                Toggle(loc.t("Settings_Startup_LaunchAtLogin"), isOn: Binding(
                    get: { loginItem.launchAtLogin },
                    set: { loginItem.setLaunchAtLogin($0) }
                ))
                Toggle(loc.t("Settings_Startup_BackgroundMode"), isOn: $backgroundMode)
                    .onChange(of: backgroundMode) { _, on in AppActivation.apply(background: on) }
                Text(loc.t("Settings_Startup_Hint"))
                    .labelStyle()
            }
        }
    }

    // MARK: - ESIGN (программ токен / USB токен)

    private var esignCard: some View {
        AppCard {
            VStack(alignment: .leading, spacing: 10) {
                Text(loc.pick("Программ токен (ESIGN)", "Software token (ESIGN)",
                              "Программный токен (ESIGN)", "软件令牌 (ESIGN)"))
                    .font(.eidSectionTitle)
                    .foregroundStyle(Color.textPrimary)
                Toggle(loc.pick("USB токеноор зурах", "Sign with USB token",
                                "Подписывать USB-токеном", "使用 USB 令牌签名"), isOn: $esignUseToken)
                Text(loc.pick(
                    "Асаалттай ба токен залгаастай бол ДАН-ий гарын үсгийг физик USB токеноор (токены гэрчилгээ + локал PIN) зурна; эс бөгөөс утас/PIN2-оор.",
                    "When on and a token is connected, DAN signatures are made with the physical USB token (token certificate + local PIN); otherwise via phone/PIN2.",
                    "Если включено и токен подключён, подпись DAN выполняется физическим USB-токеном (сертификат токена + локальный PIN); иначе через телефон/PIN2.",
                    "开启且已连接令牌时，DAN 签名将使用物理 USB 令牌（令牌证书 + 本地 PIN）；否则通过手机/PIN2。"))
                    .labelStyle()
            }
        }
    }

    // MARK: - API base override (dev convenience)

    private var apiCard: some View {
        AppCard {
            VStack(alignment: .leading, spacing: 10) {
                Text(loc.t("Settings_Api_Title"))
                    .font(.eidSectionTitle)
                    .foregroundStyle(Color.textPrimary)
                Text(loc.t("Settings_Api_Hint"))
                    .labelStyle()
                TextField("http://localhost:3000", text: $apiURL)
                    .textFieldStyle(.roundedBorder)
                    .font(.eidMonoSmall)
            }
        }
    }

    // MARK: - About

    private var aboutCard: some View {
        AppCard {
            VStack(alignment: .leading, spacing: 8) {
                Text(loc.t("Settings_About_Title"))
                    .font(.eidSectionTitle)
                    .foregroundStyle(Color.textPrimary)
                aboutRow(loc.t("Settings_Version"),
                         Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "1.0.0")
                aboutRow(loc.t("Settings_About_Build"),
                         Bundle.main.infoDictionary?["CFBundleVersion"] as? String ?? "—")
                aboutRow(loc.t("Settings_About_Publisher"), "Gerege Systems LLC")
                aboutRow(loc.t("Settings_About_Platform"), "macOS")
            }
        }
    }

    private func aboutRow(_ label: String, _ value: String) -> some View {
        HStack {
            Text(label)
                .font(.eidLabel)
                .foregroundStyle(Color.eidMuted)
                .frame(width: 96, alignment: .leading)
            Text(value)
                .font(.eidMonoSmall)
                .foregroundStyle(Color.textPrimary)
                .textSelection(.enabled)
            Spacer()
        }
    }

    // MARK: - Theme application

    private func applyTheme(_ value: String) {
        let appearance: NSAppearance? = {
            switch value {
            case "light": return NSAppearance(named: .aqua)
            case "dark":  return NSAppearance(named: .darkAqua)
            default:      return nil
            }
        }()
        NSApp.appearance = appearance
    }
}
