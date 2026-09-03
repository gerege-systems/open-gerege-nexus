import SwiftUI
import UIKit

/// Утаснаас нэвтрэх — ширээний `LoginView`-ийн гар дээрх хувилбар.
///
/// **Яагаад ширээнийхээс өөр вэ.** Мак дээр QR/РД push-ыг ХӨРШ утас
/// зөвшөөрдөг. Утсан дээр тэр зөвшөөрөгч нь өөрөө байгаа тул QR харуулах нь
/// утгагүй — өөрийгөө скан хийх боломжгүй. Оронд нь ижил session-ийг
/// **app-to-app**-аар eID Mongolia апп руу шилжүүлнэ:
///
///   1. `POST /api/v1/auth/eid/start` → `{session_id, verification_code}`
///   2. `eidmongolia://approve?sessionId=<sid>` нээнэ — eID апп зөвшөөрөл асууна
///   3. Энэ апп нь `/api/v1/auth/eid/poll`-ыг цаанаа барьж, хүн буцаж ирэхэд бэлэн
///
/// Session нь ЭНЭ ПЛАТФОРМЫН RP-ээр үүснэ — хөтөч дээрх нэвтрэлт яг эдгээр
/// route-уудыг дууддаг. Өмнө нь `/api/start` рүү явдаг байсан бөгөөд nginx
/// түүнийг eidmongolia.mn руу proxy хийдэг тул иргэн утсан дээрээ ТЭДНИЙ demo
/// RP-ийн нэрийг («RP Demo Bank») уншдаг байв — өөр бүтээгдэхүүн зөвшөөрүүлж
/// байгаа мэт.
///
/// eID апп суугаагүй бол РД push руу шилжинэ — тэр үед хүн өөр төхөөрөмж
/// дээрээ зөвшөөрнө.
struct MobileLoginView: View {
    @EnvironmentObject private var appState: AppState
    @Environment(\.scenePhase) private var scenePhase
    @ObservedObject private var loc = LocalizationService.shared

    private enum Phase: Equatable { case idle, starting, waiting, success, error }

    @State private var phase: Phase = .idle
    @State private var register = ""
    @State private var verificationCode = ""
    @State private var errorMessage = ""
    @State private var signedInName = ""
    @State private var task: Task<Void, Never>?
    @State private var showRegisterField = false
    @State private var sessionTerminated = false
    #if os(iOS)
    @State private var backgroundTaskID: UIBackgroundTaskIdentifier = .invalid
    #endif
    @FocusState private var registerFocused: Bool

    /// eID Mongolia аппын deep link. Схемийг Info.plist-ийн
    /// `LSApplicationQueriesSchemes`-д мөн бүртгэсэн байх ёстой.
    private static let appSchemes = ["eidmongolia", "geregesmartid"]

    var body: some View {
        // Богино агуулгыг ГОЛЛУУЛЖ, гар гарч ирэхэд ГҮЙЛГЭНЭ. Зөвхөн ScrollView
        // байвал агуулга дээд ирмэг дээр наалдаж, доогуураа хагас дэлгэц хоосон
        // үлддэг; зөвхөн VStack байвал регистр бичих үед гар талбарыг дардаг.
        BrandScreen {
            GeometryReader { proxy in
                ScrollView {
                    VStack(spacing: Theme.Space.xl) {
                        brand
                        card
                        if !errorMessage.isEmpty {
                            BrandBanner(text: errorMessage)
                        }
                        footer
                    }
                    .padding(.horizontal, Theme.Space.xl)
                    .padding(.vertical, Theme.Space.xxl)
                    .frame(maxWidth: 520)
                    .frame(maxWidth: .infinity, minHeight: proxy.size.height)
                }
            }
        }
        .onDisappear {
            task?.cancel()
            #if os(iOS)
            endBackgroundTask()
            #endif
        }
        .onOpenURL { url in
            handleCallbackURL(url)
        }
        .onChange(of: appState.authCallbackURL) { url in
            if let url = url {
                handleCallbackURL(url)
            }
        }
        .onChange(of: scenePhase) { newPhase in
            if newPhase == .active {
                handleAppBecameActive()
            }
        }
    }

    // MARK: - Хэсгүүд

    private var brand: some View {
        VStack(spacing: Theme.Space.md) {
            Image(systemName: "person.text.rectangle.fill")
                .font(.system(size: 32, weight: .semibold))
                .foregroundStyle(Theme.Brand.onBrand)
                .frame(width: 76, height: 76)
                .background(Theme.Brand.gradient,
                            in: RoundedRectangle(cornerRadius: 22, style: .continuous))
                .elevation(2)
            Text(AppConfig.brandName)
                .font(Theme.TypeScale.title)
                .foregroundStyle(Theme.fg1)
            Text(loc.t("Login_Subtitle"))
                .font(Theme.TypeScale.footnote)
                .foregroundStyle(Theme.fg3)
                .multilineTextAlignment(.center)
        }
        .padding(.top, Theme.Space.md)
    }

    @ViewBuilder private var card: some View {
        BrandCard(spacing: Theme.Space.lg) {
            switch phase {
            case .idle, .error:   choices
            case .starting:       progress(loc.t("Login_Initiate_Loading"))
            case .waiting:        waitingBody
            case .success:        successBody
            }
        }
    }

    private var choices: some View {
        VStack(spacing: Theme.Space.lg) {
            BrandInfoBanner(text: loc.pick(
                "eID Mongolia апп нээгдэж, зөвшөөрөл асууна. Зөвшөөрсний дараа энэ апп руугаа буцна.",
                "The eID Mongolia app opens and asks for approval. You return here once you approve.",
                "Откроется приложение eID Mongolia и запросит подтверждение. После подтверждения вы вернётесь сюда.",
                "eID Mongolia 应用将打开并请求确认。确认后即返回本应用。"))

            LoadingPrimaryButton(
                title: loc.pick("eID аппаар нэвтрэх", "Sign in with the eID app",
                                "Войти через приложение eID", "使用 eID 应用登录"),
                leadingSymbol: "arrow.up.forward.app.fill"
            ) { startAppToApp() }

            if showRegisterField || !isEidAppInstalled {
                VStack(alignment: .leading, spacing: Theme.Space.sm) {
                    BrandSectionLabel(text: loc.t("Login_NationalId"))
                    BrandInputCard(leadingIcon: "person.text.rectangle",
                                   validation: registerBadge,
                                   isFocused: registerFocused) {
                        TextField(loc.t("Login_NationalId_Placeholder"), text: $register)
                            .textInputAutocapitalization(.characters)
                            .autocorrectionDisabled()
                            .font(Theme.TypeScale.mono)
                            .foregroundStyle(Theme.fg1)
                            .focused($registerFocused)
                    }
                    BrandSecondaryButton(
                        title: loc.pick("Регистрээр мэдэгдэл илгээх", "Send a push by registration number",
                                        "Отправить push по регистрационному номеру", "按登记号发送推送"),
                        systemImage: "bell.badge",
                        tone: Theme.Brand.primary,
                        isEnabled: isRegisterValid
                    ) { startPush() }
                }
            } else {
                BrandLinkButton(title: loc.pick("Өөр төхөөрөмж дээрээ зөвшөөрөх",
                                                "Approve on another device",
                                                "Подтвердить на другом устройстве",
                                                "在其他设备上确认")) {
                    showRegisterField = true
                }
            }
        }
    }

    /// Регистр 8+ тэмдэгт болмогц ✓, богино байхад юу ч харуулахгүй —
    /// хоосон талбар дээр улаан ✗ анивчуулах нь бичиж эхлээгүй хүнийг
    /// буруутгаж байгаа хэрэг.
    private var registerBadge: BrandValidationBadge? {
        let typed = register.trimmingCharacters(in: .whitespaces)
        guard !typed.isEmpty else { return nil }
        return BrandValidationBadge(
            text: isRegisterValid ? loc.pick("зөв", "ok", "верно", "有效")
                                  : loc.pick("богино", "short", "коротко", "太短"),
            valid: isRegisterValid)
    }

    private var isRegisterValid: Bool {
        register.trimmingCharacters(in: .whitespaces).count >= 8
    }

    private var waitingBody: some View {
        VStack(spacing: Theme.Space.lg) {
            progress(loc.t("Login_Waiting_Subtitle"))
            if !verificationCode.isEmpty {
                VStack(spacing: Theme.Space.sm) {
                    BrandSectionLabel(text: loc.t("Login_VerificationCode"))
                    BrandCodeRow(code: verificationCode)
                    Text(loc.pick("Аппд харагдах кодтой тулгана уу.",
                                  "Match this with the code shown in the app.",
                                  "Сравните с кодом в приложении.",
                                  "请与应用中显示的代码核对。"))
                        .font(Theme.TypeScale.caption2)
                        .foregroundStyle(Theme.fg3)
                        .multilineTextAlignment(.center)
                }
            }
            BrandLinkButton(title: loc.t("Login_Cancel")) {
                task?.cancel()
                #if os(iOS)
                endBackgroundTask()
                #endif
                sessionTerminated = true
                appState.sessionID = ""
                phase = .idle
            }
        }
    }

    private var successBody: some View {
        VStack(spacing: Theme.Space.md) {
            Image(systemName: "checkmark.seal.fill")
                .font(.system(size: 34))
                .foregroundStyle(Theme.credit)
            Text(loc.t("Login_Success_Title"))
                .font(Theme.TypeScale.title3)
                .foregroundStyle(Theme.fg1)
            if !signedInName.isEmpty {
                Text(signedInName)
                    .font(Theme.TypeScale.body)
                    .foregroundStyle(Theme.fg2)
            }
        }
        .frame(maxWidth: .infinity)
        .padding(.vertical, Theme.Space.sm)
    }

    private func progress(_ text: String) -> some View {
        VStack(spacing: Theme.Space.md) {
            ProgressView().tint(Theme.Brand.primary)
            Text(text)
                .font(Theme.TypeScale.footnote)
                .foregroundStyle(Theme.fg2)
                .multilineTextAlignment(.center)
        }
        .frame(maxWidth: .infinity)
        .padding(.vertical, Theme.Space.sm)
    }

    private var footer: some View {
        VStack(spacing: Theme.Space.sm) {
            BrandSecurityFooter(text: loc.pick("eID Mongolia · X-Road хамгаалалттай",
                                               "eID Mongolia · secured by X-Road",
                                               "eID Mongolia · защищено X-Road",
                                               "eID Mongolia · 由 X-Road 保护"))
            Text("\(host)  ·  \(AppConfig.brandName) v\(appVersion)")
                .font(Theme.TypeScale.monoSm)
                .foregroundStyle(Theme.fg4)
        }
    }

    // MARK: - Урсгалууд

    /// eID апп суусан эсэх. Схемүүд Info.plist-д бүртгэгдээгүй бол iOS үргэлж
    /// `false` буцаана — тэр тохиолдолд ч РД push зам нээлттэй тул апп гацахгүй.
    private var isEidAppInstalled: Bool {
        Self.appSchemes.contains { scheme in
            URL(string: "\(scheme)://approve").map { UIApplication.shared.canOpenURL($0) } ?? false
        }
    }

    #if os(iOS)
    private func beginBackgroundTask() {
        endBackgroundTask()
        backgroundTaskID = UIApplication.shared.beginBackgroundTask(withName: "mn.gerege.nexus.eid_poll") {
            DispatchQueue.main.async {
                self.endBackgroundTask()
            }
        }
    }

    private func endBackgroundTask() {
        if backgroundTaskID != .invalid {
            let current = backgroundTaskID
            backgroundTaskID = .invalid
            UIApplication.shared.endBackgroundTask(current)
        }
    }
    #endif

    private func startAppToApp() {
        sessionTerminated = false
        #if os(iOS)
        beginBackgroundTask()
        #endif
        run { () -> (String, String)? in
            let response: AuthStartResponse = try await APIClient.shared
                .request(.authStart(callbackURL: AppConfig.appToAppCallback))
            let sid = response.sessionID
            await MainActor.run {
                appState.sessionID = sid
                verificationCode = response.verificationCode ?? ""
                phase = .waiting
            }
            await openEidApp(sessionID: sid)
            return (sid, "")
        }
    }

    private func startPush() {
        sessionTerminated = false
        let typed = register.trimmingCharacters(in: .whitespaces).uppercased()
        run { () -> (String, String)? in
            let response: AuthStartResponse = try await APIClient.shared
                .request(.authStartByID(nationalID: typed, callbackURL: AppConfig.appToAppCallback))
            await MainActor.run {
                appState.sessionID = response.sessionID
                verificationCode = response.verificationCode ?? ""
                phase = .waiting
            }
            return (response.sessionID, typed)
        }
    }

    /// Session эхлүүлээд poll хийх нийтлэг бүрхүүл — хоёр зам ижил төгсгөлтэй.
    private func run(_ start: @escaping () async throws -> (String, String)?) {
        task?.cancel()
        errorMessage = ""
        phase = .starting
        task = Task {
            #if os(iOS)
            defer { endBackgroundTask() }
            #endif
            do {
                guard let (sessionID, typedID) = try await start() else { return }
                let poll = try await APIClient.shared.waitForPlatformAuth(sessionID: sessionID)
                await finish(poll, typedID: typedID)
            } catch is CancellationError {
                // Хүн цуцаллаа.
            } catch let urlErr as URLError where urlErr.code == .cancelled {
                // Цуцлагдсан.
            } catch {
                await MainActor.run {
                    errorMessage = error.localizedDescription
                    phase = .error
                    if let apiErr = error as? APIError, case .server = apiErr {
                        sessionTerminated = true
                    }
                }
            }
        }
    }

    /// Callback deep link эсвэл app active болоход poll-ыг сэргээж шалгана.
    private func resumeOrVerify(sessionID: String) {
        guard phase != .success else { return }
        task?.cancel()
        errorMessage = ""
        phase = .waiting
        let typed = register.trimmingCharacters(in: .whitespaces).uppercased()
        task = Task {
            #if os(iOS)
            beginBackgroundTask()
            defer { endBackgroundTask() }
            #endif
            do {
                let poll = try await APIClient.shared.waitForPlatformAuth(sessionID: sessionID)
                await finish(poll, typedID: typed)
            } catch is CancellationError {
                // Superseded
            } catch let urlErr as URLError where urlErr.code == .cancelled {
                // Цуцлагдсан
            } catch {
                await MainActor.run {
                    errorMessage = error.localizedDescription
                    phase = .error
                    if let apiErr = error as? APIError, case .server = apiErr {
                        sessionTerminated = true
                    }
                }
            }
        }
    }

    private func handleCallbackURL(_ url: URL) {
        guard url.scheme == "gerege-nexus" else { return }
        let urlComponents = URLComponents(url: url, resolvingAgainstBaseURL: false)
        let querySid = urlComponents?.queryItems?.first(where: { $0.name == "sessionId" || $0.name == "session_id" })?.value
        let sid = (querySid?.isEmpty == false) ? querySid! : appState.sessionID
        guard !sid.isEmpty else { return }
        sessionTerminated = false
        resumeOrVerify(sessionID: sid)
    }

    private func handleAppBecameActive() {
        // Хүн eID аппаас гараараа (дэлгэцийн зүүн дээд «‹ eID Mongolia» товчоор эсвэл app switcher-ээр)
        // буцаж ирэхэд deep link өдөөгдөхгүй байж болно. Хэрэв түр сүлжээ тасарсан (.error) эсвэл
        // хүлээгдэж буй (.waiting) төлөвтэй бөгөөд sessionID байгаа бол шалгалтыг сэргээнэ.
        guard phase != .success, !sessionTerminated, !appState.sessionID.isEmpty else { return }
        if phase == .waiting || phase == .error {
            resumeOrVerify(sessionID: appState.sessionID)
        }
    }

    /// eID Mongolia апп руу шилжүүлнэ. Аль схем нээгдэхийг iOS шийднэ —
    /// `geregesmartid` нь backend-ийн хуучин схем, `eidmongolia` нь rebrand.
    @MainActor
    private func openEidApp(sessionID: String) async {
        for scheme in Self.appSchemes {
            guard let url = URL(string: "\(scheme)://approve?sessionId=\(sessionID)"),
                  UIApplication.shared.canOpenURL(url) else { continue }
            await UIApplication.shared.open(url)
            return
        }
        // Апп олдсонгүй — хүн өөр төхөөрөмж дээрээ зөвшөөрөх зам руу.
        showRegisterField = true
        errorMessage = loc.pick(
            "eID Mongolia апп олдсонгүй. Регистрийн дугаараар мэдэгдэл илгээж, өөр төхөөрөмж дээрээ зөвшөөрнө үү.",
            "The eID Mongolia app was not found. Send a push by registration number and approve on another device.",
            "Приложение eID Mongolia не найдено. Отправьте push по регистрационному номеру и подтвердите на другом устройстве.",
            "未找到 eID Mongolia 应用。请按登记号发送推送，并在其他设备上确认。")
    }

    /// Нэвтрэлтийн төгсгөл: платформын буцаасан identity-г Keychain-д snapshot
    /// болгож хадгална.
    ///
    /// Нэр нь МОНГОЛООР ирдэг тул латин галигийг нөхөх нэмэлт дуудлага
    /// (`/api/dashboard`) хэрэггүй болов — тэр route нь eidmongolia.mn-ийх
    /// бөгөөд түүний session бидэнд байхгүй.
    private func finish(_ poll: AuthPollResponse, typedID: String) async {
        guard poll.isComplete, let identity = poll.identity, identity.verifiedStatus != false else {
            await MainActor.run {
                errorMessage = Self.stateMessage(poll.state, loc: loc)
                phase = .error
                sessionTerminated = true
            }
            return
        }
        let name = identity.mongolianName ?? ""
        let stored = StoredIdentity(
            // Платформ нь eID-ийн `documentNumber`-ыг дээшээ гаргадаггүй.
            // Гэрчилгээний сериал нь иргэн ЯМАР гэрчилгээгээр зөвшөөрснийг
            // заадаг цорын ганц бариул тул ID хуудсанд түүнийг харуулна.
            documentNumber: identity.certificateSerial ?? "",
            fullName: name,
            civilID: identity.civilID ?? "",
            nationalID: identity.regNumber ?? (typedID.isEmpty ? register : typedID),
            // Нэвтрэлтийн доод хязгаар нь ADVANCED (`EID_CERT_LEVEL`); QUALIFIED
            // гэж бичих нь баталгаагүй зүйлийг батласан болно.
            certificateLevel: "ADVANCED",
            loginAt: ISO8601DateFormatter().string(from: Date())
        )
        await MainActor.run {
            signedInName = name
            phase = .success
            sessionTerminated = true
            DispatchQueue.main.asyncAfter(deadline: .now() + 1.0) {
                appState.didLogin(identity: stored)
            }
        }
    }

    /// COMPLETE биш терминал төлөвүүд. eID-ийн үгсийг иргэний хэлээр.
    private static func stateMessage(_ state: String?, loc: LocalizationService) -> String {
        switch state {
        case "EXPIRED":
            return loc.pick("Хугацаа дууслаа. Дахин оролдоно уу.",
                            "The request expired. Please try again.",
                            "Срок запроса истёк. Попробуйте снова.",
                            "请求已过期，请重试。")
        case "REFUSED":
            return loc.pick("Утсан дээр татгалзсан байна.",
                            "The request was refused on the phone.",
                            "Запрос отклонён на телефоне.",
                            "已在手机上拒绝该请求。")
        default:
            return loc.pick("Баталгаажуулалт амжилтгүй.", "Verification failed.",
                            "Проверка не удалась.", "验证失败。")
        }
    }

    private var host: String { URL(string: AppConfig.baseURL)?.host ?? AppConfig.baseURL }
    private var appVersion: String {
        Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "1.0.0"
    }
}
