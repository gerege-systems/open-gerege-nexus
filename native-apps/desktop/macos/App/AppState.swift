import SwiftUI
import LocalAuthentication

/// App-ийн 2 төлөв:
/// 1. `.login` — Утсаар нэвтрэх (v3: QR device-link / РД push notification)
/// 2. `.dashboard` — Dashboard
enum AppScreen: Equatable {
    case login
    case dashboard
}

/// Dashboard sidebar route-ууд. v3 RP-API-д backend-тэй нь: sign.
/// dashboard/id/logs нь login үр дүнгээс (cert subject + local activity log)
/// рендэрлэнэ. tokens нь macOS-only USB hardware token (локал PKCS#11).
enum DashboardTab: String, CaseIterable {
    case dashboard         // Хяналтын самбар — identity + activity
    case id                // Миний ID — cert subject
    case logs              // Лог түүх — локал session бүртгэл
    case devices           // (nav-аас нуусан — v3 RP-д device жагсаалт байхгүй)
    case security          // (nav-аас нуусан)
    case organizations     // Миний байгууллага — зөвхөн унших (мобайл апп-ын парити)
    case children          // Миний хүүхдүүд — зөвхөн унших (мобайл апп-ын парити)
    case sign              // Гарын үсэг — v3 /signature/notification + stamp
    case tokens            // macOS-only extra — USB hardware token
    case tokenReader       // Токен уншигч — Windows TokensPage-ийн парити (backend + гэрчилгээ)
    case platform          // Gerege Nexus-ийн ажлын муж — webview (SHELL_CONTRACT §1)
    case home              // desktop-only: backend health monitor (нуусан)
    case verify            // desktop-only: X.509 cert checker (нуусан)

    /// Sidebar-ын үндсэн цэс — v3-д бодит утгатай хуудсууд.
    /// Sidebar-ын үндсэн цэс. `platform` нь сүүлд: тэр нь энэ аппын өөрийн
    /// дэлгэц биш, платформын ажлын муж — native дэлгэцүүдийн ДАРАА байх нь
    /// хүнд аль нь аль болохыг хэлнэ.
    static let mainNav: [DashboardTab] = [.dashboard, .id, .organizations, .children, .logs, .sign, .tokens, .tokenReader, .platform]

    /// Mongolian fallback label (when localization is unavailable).
    var label: String {
        switch self {
        case .dashboard:     return "Хяналтын самбар"
        case .id:            return "Миний ID"
        case .logs:          return "Лог түүх"
        case .devices:       return "Итгэмжит төхөөрөмж"
        case .security:      return "Аюулгүй байдал"
        case .organizations: return "Миний байгууллага"
        case .children:      return "Миний хүүхдүүд"
        case .sign:          return "Гарын үсэг"
        case .tokens:        return "Токенууд"
        case .tokenReader:   return "Токен уншигч"
        case .platform:      return "Платформ"
        case .home:          return "Эхлэл"
        case .verify:        return "Гэрчилгээ шалгах"
        }
    }

    /// Localization key — mirrors the web i18n `nav.*` keys.
    var labelKey: String {
        switch self {
        case .dashboard:     return "Nav_Dashboard"
        case .id:            return "Nav_MyId"
        case .logs:          return "Nav_Logs"
        case .devices:       return "Nav_Devices"
        case .security:      return "Nav_Security"
        case .organizations: return "Nav_MyOrganizations"
        case .children:      return "Nav_Children"
        case .sign:          return "Nav_Sign"
        case .tokens:        return "Nav_Tokens"
        case .tokenReader:   return "Nav_TokenReader"
        case .platform:      return "Nav_Platform"
        case .home:          return "Nav_Home"
        case .verify:        return "Nav_Verify"
        }
    }

    /// SF Symbol mirroring the web lucide icon.
    var icon: String {
        switch self {
        case .dashboard:     return "house"
        case .id:            return "person.text.rectangle"
        case .logs:          return "clock.arrow.circlepath"
        case .devices:       return "iphone"
        case .security:      return "checkmark.shield"
        case .organizations: return "building.2"
        case .children:      return "figure.2.and.child.holdinghands"
        case .sign:          return "signature"
        case .tokens:        return "externaldrive"
        case .tokenReader:   return "doc.text.magnifyingglass"
        case .platform:      return "square.grid.2x2"
        case .home:          return "waveform.path.ecg"
        case .verify:        return "checkmark.seal"
        }
    }
}

/// v3 auth ceremony-ийн үр дүнгээс хадгалагдах identity snapshot.
/// v3-д bearer session байхгүй — `documentNumber` нь дараагийн гарын үсгийн
/// хүсэлтүүдийн identity handle.
struct StoredIdentity: Codable {
    let documentNumber: String
    /// Монгол нэр (XYP-ийн хураангуйгаас). Гэрчилгээн дээрх латин галиг БИШ.
    let fullName: String
    let civilID: String
    /// Push нэвтрэлтэд бичсэн РД (QR-д хоосон байж болно).
    let nationalID: String
    /// cert.certificateLevel (жишээ нь QUALIFIED).
    let certificateLevel: String
    /// ISO8601 — нэвтэрсэн огноо.
    let loginAt: String
    /// ESIGN "программ токен"-д зориулсан гэрчилгээний кэш (base64 DER). ДАН-д илгээх
    /// payload-д гэрчилгээг ЗУРАХААС ӨМНӨ хавсаргах ёстой бол `/api/certificates` нь
    /// session-bound — иймд нэвтэрсэн даруйд татаж энд хадгална. Optional: хуучин
    /// хадгалсан identity-г (эдгээр талбаргүй) задлахад алдаа гарахгүй.
    var signingCertB64: String?
    var authCertB64: String?
}

@MainActor
final class AppState: ObservableObject {

    /// Нэвтрэлгүйгээр аль нэг дэлгэцийг нээх — ЗӨВХӨН DEBUG.
    ///
    /// Нэвтрэлт нь иргэний утсанд push илгээдэг тул дэлгэцийн алдааг хөөж
    /// байгаа хүн бүр жинхэнэ eID-ээр нэвтрэх шаардлагатай болдог. Тэр нь
    /// «Платформ» товч дарахад унадаг алдааг олох цорын ганц зам нь тэр
    /// товчийг дарж чадах хүн байх явдал гэсэн үг байв.
    ///
    ///   EID_DEBUG_TAB=platform         эхнээсээ тэр дэлгэц дээр нээгдэнэ
    ///   EID_DEBUG_TAB=tokens>platform  эхнийх дээр нээгдээд гурван секундын
    ///                                  дараа хоёр дахь руу ШИЛЖИНЭ — шилжилтэд
    ///                                  гардаг алдааг барихад (эхнээсээ нээхэд
    ///                                  гардаггүй нь бий)
    ///
    /// Release build-д ЭНЭ КОД БАЙХГҮЙ — тиймээс нэвтрэлтийг тойрох зам ч
    /// байхгүй.
    @Published var screen: AppScreen = {
        #if DEBUG
        if ProcessInfo.processInfo.environment["EID_DEBUG_TAB"] != nil { return .dashboard }
        #endif
        return .login
    }()

    @Published var selectedTab: DashboardTab = {
        #if DEBUG
        if let raw = ProcessInfo.processInfo.environment["EID_DEBUG_TAB"],
           let first = raw.split(separator: ">").first,
           let tab = DashboardTab(rawValue: String(first)) { return tab }
        #endif
        return .dashboard
    }()

    #if DEBUG
    /// `EID_DEBUG_TAB=a>b` үед `b` рүү гурван секундын дараа шилжинэ.
    func startDebugTabSwitchIfRequested() {
        guard let raw = ProcessInfo.processInfo.environment["EID_DEBUG_TAB"] else { return }
        let parts = raw.split(separator: ">")
        guard parts.count == 2, let target = DashboardTab(rawValue: String(parts[1])) else { return }
        DispatchQueue.main.asyncAfter(deadline: .now() + 3) { [weak self] in
            print("EID_DEBUG_TAB: шилжиж байна ->", target.rawValue)
            self?.selectedTab = target
        }
    }
    #endif
    @Published var nationalID: String = ""
    @Published var fullName: String = ""
    @Published var civilID: String = ""
    @Published var kycLevel: String = ""
    @Published var sessionID: String = ""
    /// v3 identity handle — гарын үсэг зурахад ашиглана.
    @Published var documentNumber: String = ""
    /// ESIGN-ийн гэрчилгээний кэш (base64 DER) — signing давуу, эс бөгөөс auth.
    @Published var esignSigningCertB64: String = ""
    @Published var esignAuthCertB64: String = ""

    /// ДАН-д гарын үсэг зурахдаа физик USB токен ашиглах эсэх. Асаалттай + токен залгаастай
    /// бол EsignBridge токен замаар (ESPK cert + локал PIN), эс бөгөөс утас/PIN2-оор зурна.
    static let esignUseTokenKey = "esign.useToken"
    @Published var esignUseToken: Bool = UserDefaults.standard.bool(forKey: AppState.esignUseTokenKey) {
        didSet { UserDefaults.standard.set(esignUseToken, forKey: Self.esignUseTokenKey) }
    }

    /// Dashboard-ын локал өгөгдөл (identity + local activity log-оос угсарна).
    @Published var dashboardData: DashboardData?

    /// Тохиргооны хуудас нээлттэй эсэх. `@State` биш ЭНД байгаа шалтгаан нь
    /// платформын бүрхүүл `shell.openPane("settings")`-ээр үүнийг нээдэг —
    /// дэлгэцийн дотоод төлөв байсан бол гаднаас хүрэх зам байхгүй.
    @Published var settingsPresented = false

    /// Зөвхөн унших самбарууд — нэвтрэх агшинд НЭГ удаа татсан snapshot.
    /// pollToken 10 минут хүчинтэй тул дараа нь дахин татахгүй; keychain-аас сэргээсэн
    /// session-д хоосон байна (дахин нэвтэрснээр шинэчлэгдэнэ) — PII-г дискэнд бичихгүй.
    @Published var organizations: [Representation] = []
    @Published var children: [PersonChild] = []
    @Published var extrasLoaded = false

    /// User-toggleable Touch ID gate on session restore.
    static let biometricGateKey = "auth.biometricGateEnabled"
    private static let activityLogKey = "activity.sessions"

    // MARK: - Session restore

    /// Keychain-д хадгалсан identity snapshot-оор login төлөвийг сэргээнэ.
    /// v3-д сервер талын session байхгүй тул энэ нь зөвхөн локал "сануулах"
    /// төлөв — дараагийн гарын үсэг бүр утсан дээр PIN2-оор баталгаажна.
    func restoreSessionFromKeychain() {
        Task {
            guard let data = try? KeychainManager.shared.load(.identity),
                  let identity = try? JSONDecoder().decode(StoredIdentity.self, from: data) else {
                return
            }
            if !(await passBiometricGate()) { return }
            apply(identity: identity)
            startEsignBridge()   // кэшлэсэн гэрчилгээтэй бол апп сэргэхэд гүүр шууд асна
            screen = .dashboard
        }
    }

    private func passBiometricGate() async -> Bool {
        let defaults = UserDefaults.standard
        let enabledRaw = defaults.object(forKey: Self.biometricGateKey) as? Bool
        let enabled = enabledRaw ?? true
        guard enabled else { return true }
        let ctx = LAContext()
        var err: NSError?
        guard ctx.canEvaluatePolicy(.deviceOwnerAuthenticationWithBiometrics, error: &err) else {
            return true
        }
        return await withCheckedContinuation { cont in
            ctx.evaluatePolicy(.deviceOwnerAuthenticationWithBiometrics,
                               localizedReason: "Гэрэгэ нэвтрэлтийг сэргээх") { ok, _ in
                cont.resume(returning: ok)
            }
        }
    }

    // MARK: - Login / logout

    /// v3 auth ceremony (`/v3/session/{id}` → COMPLETE + OK) дууссаны дараа
    /// дуудагдана. Cert subject-оос нэр/civil ID-г аль хэдийн задалсан байна.
    func didLogin(identity: StoredIdentity) {
        if let data = try? JSONEncoder().encode(identity) {
            try? KeychainManager.shared.save(data, key: .identity)
        }
        apply(identity: identity)
        recordSession(type: "AUTH", state: "COMPLETE", result: "OK")
        screen = .dashboard
    }

    func logout() {
        #if os(macOS)
        EsignBridge.shared.clearIdentity()
        #endif
        try? KeychainManager.shared.delete(.identity)
        UserDefaults.standard.removeObject(forKey: Self.activityLogKey)
        nationalID = ""
        fullName = ""
        civilID = ""
        kycLevel = ""
        sessionID = ""
        documentNumber = ""
        esignSigningCertB64 = ""
        esignAuthCertB64 = ""
        dashboardData = nil
        organizations = []
        children = []
        extrasLoaded = false
        screen = .login
    }

    private func apply(identity: StoredIdentity) {
        nationalID = identity.nationalID.isEmpty ? identity.civilID : identity.nationalID
        fullName = identity.fullName
        civilID = identity.civilID
        kycLevel = identity.certificateLevel
        documentNumber = identity.documentNumber
        esignSigningCertB64 = identity.signingCertB64 ?? ""
        esignAuthCertB64 = identity.authCertB64 ?? ""
        rebuildDashboardData(loginAt: identity.loginAt)
    }

    /// Нэвтэрсэн session-ий pollToken-оор байгууллага/хүүхдийн жагсаалтыг татна.
    /// Аль нэг нь уначихвал (эрх/хугацаа) нөгөө нь хэвээр — хуудас хоосон төлөвөө өөрөө харуулна.
    func loadPersonExtras(sessionID: String, pollToken: String) {
        guard !sessionID.isEmpty, !pollToken.isEmpty else { return }
        Task {
            async let orgs: RepresentationsResponse? = try? await APIClient.shared
                .request(.representations(sessionID: sessionID, pollToken: pollToken))
            async let kids: ChildrenResponse? = try? await APIClient.shared
                .request(.children(sessionID: sessionID, pollToken: pollToken))
            // ESIGN-ийн гэрчилгээ — pollToken хүчинтэй байх ЦОРЫН ГАНЦ агшин.
            async let certs: CertificatesResponse? = try? await APIClient.shared
                .request(.certificates(sessionID: sessionID, pollToken: pollToken))
            let (o, c, k) = await (orgs, kids, certs)
            organizations = o?.representations ?? []
            children = c?.children ?? []
            if let k { cacheEsignCerts(signing: k.signing ?? "", auth: k.auth ?? "") }
            extrasLoaded = true
        }
    }

    /// ESIGN-ийн гэрчилгээг санах ойд + Keychain-ийн identity snapshot-д хадгална.
    /// Хоосон утга ирвэл өмнөх кэшийг УСТГАХГҮЙ (сүлжээний түр алдаанаас болж ESIGN
    /// ажиллахаа болихоос сэргийлнэ) — зөвхөн шинэ утгаар дарж бичнэ.
    private func cacheEsignCerts(signing: String, auth: String) {
        if !signing.isEmpty { esignSigningCertB64 = signing }
        if !auth.isEmpty { esignAuthCertB64 = auth }
        startEsignBridge()
        guard !signing.isEmpty || !auth.isEmpty,
              let data = try? KeychainManager.shared.load(.identity),
              var identity = try? JSONDecoder().decode(StoredIdentity.self, from: data) else { return }
        if !signing.isEmpty { identity.signingCertB64 = signing }
        if !auth.isEmpty { identity.authCertB64 = auth }
        if let out = try? JSONEncoder().encode(identity) {
            try? KeychainManager.shared.save(out, key: .identity)
        }
    }

    /// ESIGN гүүрт нэвтэрсэн identity-г өгнө (гүүр өөрөө апп асахад аль хэдийн ажиллаж байгаа).
    ///
    /// ЗӨВХӨН ШИРЭЭНИЙ. Гүүр нь `ws://127.0.0.1:59001` дээр сонсдог — ДАН-ий вэб
    /// хуудас ижил машин дээрх хөтчөөс залгах загвар. Утсан дээр ийм хөрш процесс
    /// байхгүй тул энэ файлыг iOS target хуваалцахдаа гүүрийг хамт авчрахгүй.
    func startEsignBridge() {
        #if os(macOS)
        let cert = esignSigningCertB64.isEmpty ? esignAuthCertB64 : esignSigningCertB64
        let person = civilID.isEmpty ? nationalID : civilID
        guard !cert.isEmpty, !person.isEmpty else { return }
        EsignBridge.shared.setIdentity(personID: person, certificateB64: cert)
        #endif
    }

    // MARK: - Local activity log

    /// v3 RP-API нь иргэний бүх session-ий түүхийг өгдөггүй тул энэ app-аас
    /// хийсэн auth/sign session-үүдийг локалд бүртгэж dashboard-д харуулна.
    func recordSession(type: String, state: String, result: String, pushText: String? = nil) {
        var log = Self.loadActivityLog()
        log.insert(DashboardSession(
            id: UUID().uuidString,
            sessionType: type,
            state: state,
            result: result,
            rpName: AppConfig.serviceName,
            pushText: pushText,
            createdAt: Self.isoNow()
        ), at: 0)
        if log.count > 100 { log = Array(log.prefix(100)) }
        if let data = try? JSONEncoder().encode(log) {
            UserDefaults.standard.set(data, forKey: Self.activityLogKey)
        }
        rebuildDashboardData()
    }

    private static func loadActivityLog() -> [DashboardSession] {
        guard let data = UserDefaults.standard.data(forKey: activityLogKey),
              let log = try? JSONDecoder().decode([DashboardSession].self, from: data) else {
            return []
        }
        return log
    }

    private static func isoNow() -> String {
        ISO8601DateFormatter().string(from: Date())
    }

    /// Identity + локал activity log → DashboardData (view-үүд хэвээр ажиллана).
    private func rebuildDashboardData(loginAt: String? = nil) {
        let sessions = Self.loadActivityLog()
        let createdAt = loginAt ?? dashboardData?.user.createdAt ?? Self.isoNow()
        dashboardData = DashboardData(
            user: DashboardUser(
                nationalId: nationalID,
                fullName: fullName,
                kycLevel: kycLevel,
                status: "ACTIVE",
                createdAt: createdAt,
                photo: nil,
                civilId: civilID.isEmpty ? nil : civilID,
                givenName: nil,
                surname: nil,
                givenNameEn: nil,
                surnameEn: nil,
                fullNameLatin: nil
            ),
            devices: [],
            certificates: documentNumber.isEmpty ? 0 : 1,
            sessions: sessions,
            totalLogins: sessions.filter { $0.sessionType == "AUTH" }.count
        )
    }
}

// MARK: - Dashboard Data Models (локал угсралт — v3-д dashboard endpoint байхгүй)

struct DashboardData: Codable {
    let user: DashboardUser
    let devices: [DashboardDevice]
    let certificates: Int
    let sessions: [DashboardSession]
    let totalLogins: Int
}

struct DashboardUser: Codable {
    let nationalId: String
    let fullName: String
    let kycLevel: String
    let status: String
    let createdAt: String
    let photo: String?
    let civilId: String?
    let givenName: String?
    let surname: String?
    let givenNameEn: String?
    let surnameEn: String?
    let fullNameLatin: String?

    /// Дэлгэцэнд МОНГОЛ нэр. Өмнө нь энэ нь латин галигт давуу эрх өгдөг байсан тул
    /// нэвтэрсэн иргэн өөрийн нэрийг «ERDENEBAT TSENDDORJ» гэж хардаг байв.
    var displayName: String {
        let mongolian = fullName.trimmingCharacters(in: .whitespaces)
        if !mongolian.isEmpty { return mongolian }
        return (fullNameLatin ?? "").trimmingCharacters(in: .whitespaces)
    }
}

struct DashboardDevice: Codable, Identifiable {
    var id: String { deviceId }
    let deviceId: String
    let platform: String
    let status: String
    let lastUsedAt: String?
    let createdAt: String
}

struct DashboardSession: Codable, Identifiable {
    let id: String
    let sessionType: String
    let state: String
    let result: String
    let rpName: String
    let pushText: String?
    let createdAt: String
}
