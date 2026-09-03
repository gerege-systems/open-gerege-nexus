import SwiftUI

// Web dashboard page parity (eid-web `src/app/dashboard/*`). Each view mirrors
// the corresponding Next.js page 1:1 — same data source (the bearer-scoped
// /web2app/v1/dashboard payload already on `appState.dashboardData`), same
// fields, same actions. Bilingual strings follow the web pages' own inline
// `locale === "mn" ? … : …` pattern via `loc.language`.

// MARK: - Shared helpers

/// ISO-8601 string → "MMM d, yyyy HH:mm" (web `toLocaleDateString`+`Time`).
private func eidFmtDateTime(_ iso: String?, _ lang: AppLanguage) -> String {
    guard let iso, let date = isoParse(iso) else { return "—" }
    let f = DateFormatter()
    f.locale = Locale(identifier: lang.localeIdentifier)
    f.dateFormat = "yyyy.MM.dd  HH:mm"
    return f.string(from: date)
}
/// ISO-8601 string → "d MMMM yyyy" (web `{year,month:'long',day}`).
private func eidFmtDateLong(_ iso: String?, _ lang: AppLanguage) -> String {
    guard let iso, let date = isoParse(iso) else { return "—" }
    let f = DateFormatter()
    f.locale = Locale(identifier: lang.localeIdentifier)
    f.dateStyle = .long
    return f.string(from: date)
}

private func isoParse(_ s: String) -> Date? {
    let iso = ISO8601DateFormatter()
    iso.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
    if let d = iso.date(from: s) { return d }
    iso.formatOptions = [.withInternetDateTime]
    return iso.date(from: s)
}

/// Two-line label/value cell used by the detail grids.
private struct EidField: View {
    let label: String
    let value: String
    var mono = false
    var body: some View {
        VStack(alignment: .leading, spacing: 3) {
            Text(label.uppercased())
                .font(.system(size: 10, weight: .medium))
                .foregroundStyle(Color.textSecondary)
            Text(value.isEmpty ? "—" : value)
                .font(.system(size: 14, weight: .semibold, design: mono ? .monospaced : .default))
                .foregroundStyle(Color.textPrimary)
                .textSelection(.enabled)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }
}

/// Brand-tinted icon + label/value card (web `DetailCard`).
private struct EidStatCard: View {
    let icon: String
    let label: String
    let value: String
    var accent: Color = .eidAccent
    var body: some View {
        AppCard {
            HStack(alignment: .top, spacing: 14) {
                Image(systemName: icon)
                    .font(.system(size: 18, weight: .semibold))
                    .foregroundStyle(accent)
                    .frame(width: 40, height: 40)
                    .background(accent.opacity(0.12))
                    .clipShape(RoundedRectangle(cornerRadius: 11, style: .continuous))
                VStack(alignment: .leading, spacing: 3) {
                    Text(label.uppercased())
                        .font(.system(size: 10, weight: .medium))
                        .foregroundStyle(Color.textSecondary)
                    Text(value)
                        .font(.system(size: 15, weight: .semibold))
                        .foregroundStyle(Color.textPrimary)
                }
                Spacer(minLength: 0)
            }
        }
    }
}

/// Page header — title + subtitle (web `<h1>` + `<p>`). Shared across the
/// web-parity pages and the Sign page.
struct EidPageHeader: View {
    let title: String
    let subtitle: String
    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(title).font(.system(size: 22, weight: .bold)).foregroundStyle(Color.textPrimary)
            if !subtitle.isEmpty {
                Text(subtitle).font(.system(size: 13)).foregroundStyle(Color.textSecondary)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }
}

struct EidScroll<Content: View>: View {
    @ViewBuilder let content: () -> Content
    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 20) { content() }
                .padding(28)
                .frame(maxWidth: 920, alignment: .leading)
                .frame(maxWidth: .infinity, alignment: .leading)
        }
    }
}

// MARK: - /dashboard/id — Миний ID

struct MyIdView: View {
    @EnvironmentObject private var appState: AppState
    @ObservedObject private var loc = LocalizationService.shared

    private var user: DashboardUser? { appState.dashboardData?.user }

    var body: some View {
        EidScroll {
            EidPageHeader(
                title: loc.t("Nav_MyId"),
                subtitle: loc.pick("Танай иргэний цахим үнэмлэхний нэгдсэн профайл", "Your unified e-ID profile", "Ваш единый профиль e-ID", "您的统一 e-ID 档案")
            )
            if let user {
                heroCard(user)
                nameCard(user)
                detailGrid(user)
            }
        }
    }

    private func heroCard(_ user: DashboardUser) -> some View {
        AppCard {
            HStack(alignment: .top, spacing: 18) {
                UserAvatar(photo: user.photo, initials: initials(user.displayName), size: 76)
                VStack(alignment: .leading, spacing: 6) {
                    HStack(spacing: 10) {
                        Text(user.displayName)
                            .font(.system(size: 22, weight: .heavy))
                            .foregroundStyle(Color.textPrimary)
                        StatusPill(user.status.lowercased() == "active"
                                   ? (loc.pick("Идэвхтэй", "Active", "Активен", "有效")) : user.status,
                                   variant: user.status.lowercased() == "active" ? .ok : .warn)
                    }
                    EidField(label: loc.pick("Регистрийн дугаар", "Registration number", "Регистрационный номер", "登记号"),
                             value: user.nationalId, mono: true)
                    if let civil = user.civilId, !civil.isEmpty {
                        EidField(label: loc.pick("Иргэний бүртгэлийн дугаар", "Civil ID", "Гражданский ID", "公民号"),
                                 value: civil, mono: true)
                    }
                }
                Spacer(minLength: 0)
            }
        }
    }

    private func nameCard(_ user: DashboardUser) -> some View {
        AppCard {
            VStack(alignment: .leading, spacing: 14) {
                Text((loc.pick("Нэр", "Name", "Имя", "姓名")).uppercased())
                    .font(.system(size: 11, weight: .medium))
                    .foregroundStyle(Color.textSecondary)
                // v3: нэр cert subject-оос ирнэ (given/surname тусдаа RDN
                // ирээгүй бол зөвхөн бүтэн нэр харуулна).
                LazyVGrid(columns: [GridItem(.flexible()), GridItem(.flexible())], alignment: .leading, spacing: 14) {
                    EidField(label: loc.pick("Бүтэн нэр", "Full name", "Полное имя", "全名"), value: user.fullName)
                        .gridCellColumns(2)
                    EidField(label: loc.pick("Овог", "Surname", "Фамилия", "姓"), value: user.surname ?? "—")
                    EidField(label: loc.pick("Нэр", "Given name", "Имя", "名"), value: user.givenName ?? "—")
                }
            }
        }
    }

    private func detailGrid(_ user: DashboardUser) -> some View {
        LazyVGrid(columns: [GridItem(.flexible()), GridItem(.flexible())], spacing: 14) {
            EidStatCard(icon: "faceid", label: loc.pick("KYC түвшин", "KYC level", "Уровень KYC", "KYC 等级"), value: kycLabel(user.kycLevel))
            EidStatCard(icon: "checkmark.shield", label: loc.pick("Баталгаажуулалт", "Verification", "Проверка", "核验"),
                        value: user.kycLevel.uppercased().isEmpty ? "—" : user.kycLevel.uppercased())
            EidStatCard(icon: "calendar", label: loc.pick("Бүртгэгдсэн", "Registered on", "Дата регистрации", "注册日期"),
                        value: eidFmtDateLong(user.createdAt, loc.language))
            EidStatCard(icon: "person.text.rectangle", label: loc.pick("Төлөв", "Status", "Статус", "状态"),
                        value: user.status.lowercased() == "active" ? (loc.pick("Идэвхтэй", "Active", "Активен", "有效")) : user.status,
                        accent: .eidSuccess)
            EidStatCard(icon: "key", label: loc.pick("Сертификат", "Certificate", "Сертификат", "证书"),
                        value: "\(appState.dashboardData?.certificates ?? 0) \(loc.pick("ширхэг", "pcs", "шт.", "个"))")
            EidStatCard(icon: "iphone", label: loc.pick("Төхөөрөмж", "Device", "Устройство", "设备"),
                        value: "\(appState.dashboardData?.devices.count ?? 0) \(loc.pick("ширхэг", "pcs", "шт.", "个"))")
        }
    }

    private func kycLabel(_ src: String) -> String {
        switch src.lowercased() {
        case "dan":    return loc.pick("Дижитал Архив Үндэсний (DAN)", "DAN gateway", "Шлюз DAN", "DAN 网关")
        case "gsign":  return loc.pick("G-Sign баталгаажуулалт", "G-Sign gateway", "Шлюз G-Sign", "G-Sign 网关")
        case "manual": return loc.pick("Гарын баталгаажуулалт", "Manual KYC", "Ручной KYC", "人工 KYC")
        case "":       return loc.pick("Тодорхойгүй", "Unknown", "Неизвестно", "未知")
        default:       return src
        }
    }

    private func initials(_ name: String) -> String {
        name.split(separator: " ").prefix(2).compactMap { $0.first }.map(String.init).joined().uppercased()
    }
}

// MARK: - /dashboard/logs — Лог түүх

struct LogsView: View {
    @EnvironmentObject private var appState: AppState
    @ObservedObject private var loc = LocalizationService.shared
    @State private var tab: LogFilter = .all

    enum LogFilter: String { case all, AUTH, SIGN }
    private var sessions: [DashboardSession] { appState.dashboardData?.sessions ?? [] }
    private var filtered: [DashboardSession] {
        tab == .all ? sessions : sessions.filter { $0.sessionType == tab.rawValue }
    }

    var body: some View {
        EidScroll {
            EidPageHeader(title: loc.t("Nav_Logs"),
                          subtitle: loc.pick("Нэвтрэлт ба гарын үсгийн түүх", "Login and signature history", "История входов и подписей", "登录与签名历史"))
            statsRow
            filterChips
            if filtered.isEmpty {
                AppCard { emptyState }
            } else {
                AppCard(padding: 0) {
                    VStack(spacing: 0) {
                        ForEach(Array(filtered.enumerated()), id: \.element.id) { idx, s in
                            sessionRow(s)
                            if idx < filtered.count - 1 { Divider() }
                        }
                    }
                }
            }
        }
    }

    private var total: Int { sessions.count }
    private var success: Int { sessions.filter { $0.result == "OK" }.count }
    private var authCount: Int { sessions.filter { $0.sessionType == "AUTH" }.count }
    private var signCount: Int { sessions.filter { $0.sessionType == "SIGN" }.count }

    private var statsRow: some View {
        LazyVGrid(columns: [GridItem(.flexible()), GridItem(.flexible()), GridItem(.flexible())], spacing: 14) {
            EidStatCard(icon: "list.bullet.clipboard", label: loc.pick("Нийт бүртгэл", "Total events", "Всего событий", "事件总数"), value: "\(total)")
            EidStatCard(icon: "chart.line.uptrend.xyaxis", label: loc.pick("Амжилтын хувь", "Success rate", "Доля успешных", "成功率"),
                        value: total == 0 ? "—" : "\(Int((Double(success) / Double(total) * 100).rounded()))%",
                        accent: .eidSuccess)
            EidStatCard(icon: "signature", label: loc.pick("Гарын үсэг / Нэвтрэлт", "Sign / Login", "Подпись / вход", "签名 / 登录"),
                        value: "\(signCount) / \(authCount)", accent: .eidWarning)
        }
    }

    private var filterChips: some View {
        HStack(spacing: 8) {
            chip(.all, loc.pick("Бүгд", "All", "Все", "全部"), total)
            chip(.AUTH, loc.pick("Нэвтрэлт", "Login", "Вход", "登录"), authCount)
            chip(.SIGN, loc.pick("Гарын үсэг", "Signature", "Подпись", "签名"), signCount)
            Spacer()
            Text("\(filtered.count) \(loc.pick("бичлэг", "records", "записей", "条记录"))")
                .font(.system(size: 12)).foregroundStyle(Color.textSecondary)
        }
    }

    private func chip(_ f: LogFilter, _ label: String, _ count: Int) -> some View {
        Button { tab = f } label: {
            HStack(spacing: 6) {
                Text(label).font(.system(size: 13, weight: .semibold))
                Text("\(count)").font(.system(size: 10, weight: .bold))
                    .padding(.horizontal, 6).padding(.vertical, 2)
                    .background((tab == f ? Color.white.opacity(0.25) : Color.eidMuted))
                    .clipShape(Capsule())
            }
            .padding(.horizontal, 12).padding(.vertical, 7)
            .foregroundStyle(tab == f ? .white : Color.textSecondary)
            .background(tab == f ? Color.brand700 : Color.eidMuted.opacity(0.4))
            .clipShape(Capsule())
        }
        .buttonStyle(.plain)
    }

    private func sessionRow(_ s: DashboardSession) -> some View {
        let isSign = s.sessionType == "SIGN"
        return HStack(spacing: 14) {
            Image(systemName: isSign ? "signature" : "arrow.right.square")
                .font(.system(size: 15))
                .foregroundStyle(isSign ? Color.eidWarning : Color.eidAccent)
                .frame(width: 34, height: 34)
                .background((isSign ? Color.eidWarning : Color.eidAccent).opacity(0.12))
                .clipShape(RoundedRectangle(cornerRadius: 9, style: .continuous))
            VStack(alignment: .leading, spacing: 2) {
                Text(isSign ? (loc.pick("Гарын үсэг", "Signature", "Подпись", "签名")) : (loc.pick("Нэвтрэлт", "Login", "Вход", "登录")))
                    .font(.system(size: 13, weight: .medium)).foregroundStyle(Color.textPrimary)
                Text(eidFmtDateTime(s.createdAt, loc.language))
                    .font(.system(size: 11, design: .monospaced)).foregroundStyle(Color.textSecondary)
            }
            VStack(alignment: .leading, spacing: 2) {
                Text(s.rpName.isEmpty || s.rpName == "—" ? "—" : s.rpName)
                    .font(.system(size: 13)).foregroundStyle(Color.textSecondary).lineLimit(1)
                if let push = s.pushText, !push.isEmpty {
                    Text(push).font(.system(size: 11)).foregroundStyle(Color.textSecondary).lineLimit(1)
                }
            }
            Spacer(minLength: 0)
            resultPill(s.result)
        }
        .padding(.horizontal, 18).padding(.vertical, 12)
    }

    @ViewBuilder
    private func resultPill(_ result: String) -> some View {
        switch result {
        case "OK": StatusPill(loc.pick("Амжилттай", "Success", "Успешно", "成功"), variant: .ok)
        case "USER_REFUSED": StatusPill(loc.pick("Татгалзсан", "Refused", "Отклонено", "已拒绝"), variant: .warn)
        default: StatusPill(loc.pick("Алдаа", "Error", "Ошибка", "错误"), variant: .bad)
        }
    }

    private var emptyState: some View {
        VStack(spacing: 8) {
            Image(systemName: "tray").font(.system(size: 28)).foregroundStyle(Color.textSecondary)
            Text(loc.pick("Бүртгэл олдсонгүй", "No records yet", "Записей пока нет", "暂无记录"))
                .font(.system(size: 14, weight: .semibold)).foregroundStyle(Color.textPrimary)
            Text(loc.pick("Нэвтрэлт болон гарын үсгийн үйлдлүүд энд харагдана.", "Your login and signature activity will appear here.", "Здесь появятся ваши входы и подписи.", "您的登录与签名活动将显示在此处。"))
                .font(.system(size: 12)).foregroundStyle(Color.textSecondary)
        }
        .frame(maxWidth: .infinity).padding(.vertical, 28)
    }
}

// MARK: - /dashboard/devices — Итгэмжит төхөөрөмж

struct DevicesView: View {
    @EnvironmentObject private var appState: AppState
    @ObservedObject private var loc = LocalizationService.shared
    private var devices: [DashboardDevice] { appState.dashboardData?.devices ?? [] }

    var body: some View {
        EidScroll {
            EidPageHeader(title: loc.t("Nav_Devices"),
                          subtitle: loc.pick("Таны бүртгэлд холбогдсон төхөөрөмжүүд", "Devices linked to your account", "Устройства, привязанные к вашему аккаунту", "已关联到您账户的设备"))
            if devices.isEmpty {
                AppCard {
                    VStack(spacing: 10) {
                        Image(systemName: "iphone.slash").font(.system(size: 28)).foregroundStyle(Color.textSecondary)
                        Text(loc.pick("Бүртгэлтэй төхөөрөмж алга", "No registered devices", "Нет зарегистрированных устройств", "没有已注册的设备"))
                            .font(.system(size: 14)).foregroundStyle(Color.textSecondary)
                    }.frame(maxWidth: .infinity).padding(.vertical, 22)
                }
            } else {
                ForEach(Array(devices.enumerated()), id: \.element.id) { idx, d in
                    deviceCard(d, isCurrent: idx == 0)
                }
            }
        }
    }

    private func deviceCard(_ d: DashboardDevice, isCurrent: Bool) -> some View {
        let isIOS = d.platform == "IOS"
        let isActive = d.status == "ACTIVE"
        return AppCard {
            VStack(alignment: .leading, spacing: 14) {
                HStack(alignment: .top) {
                    HStack(spacing: 14) {
                        Image(systemName: "iphone")
                            .font(.system(size: 22))
                            .foregroundStyle(isIOS ? Color.eidAccent : Color.textSecondary)
                            .frame(width: 46, height: 46)
                            .background((isIOS ? Color.eidAccent : Color.textSecondary).opacity(0.12))
                            .clipShape(RoundedRectangle(cornerRadius: 12, style: .continuous))
                        VStack(alignment: .leading, spacing: 3) {
                            HStack(spacing: 8) {
                                Text(deviceName(d.platform)).font(.system(size: 16, weight: .bold))
                                    .foregroundStyle(Color.textPrimary)
                                if isCurrent { StatusPill(loc.pick("Одоогийн", "Current", "Текущее", "当前设备"), variant: .ok) }
                            }
                            Text("\(d.platform) · \(String(d.deviceId.prefix(8)))…")
                                .font(.system(size: 12)).foregroundStyle(Color.textSecondary)
                        }
                    }
                    Spacer()
                    StatusPill(isActive ? (loc.pick("Идэвхтэй", "Active now", "Активно сейчас", "当前活跃")) : (loc.pick("Идэвхгүй", "Inactive", "Неактивно", "未在用")),
                               variant: isActive ? .ok : .warn)
                }
                Divider()
                LazyVGrid(columns: Array(repeating: GridItem(.flexible()), count: 2), spacing: 12) {
                    EidField(label: loc.pick("Цагийн бүс", "Timezone", "Часовой пояс", "时区"), value: "Asia/Ulaanbaatar")
                    EidField(label: loc.pick("Платформ", "Platform", "Платформа", "平台"), value: d.platform)
                    EidField(label: loc.pick("Бүртгэгдсэн", "Registered on", "Дата регистрации", "注册日期"), value: eidFmtDateTime(d.createdAt, loc.language))
                    EidField(label: loc.pick("Сүүлд идэвхтэй", "Last active", "Последняя активность", "最近活跃"), value: eidFmtDateTime(d.lastUsedAt, loc.language))
                }
                Divider()
                HStack(spacing: 18) {
                    capability("checkmark.circle.fill", loc.pick("PKI хүчинтэй", "PKI valid", "PKI действителен", "PKI 有效"))
                    capability("shield.fill", loc.pick("Биометр идэвхтэй", "Biometrics active", "Биометрия активна", "生物识别已启用"))
                    capability("key.fill", loc.pick("Хардвер түлхүүр", "Hardware key", "Аппаратный ключ", "硬件密钥"))
                }
            }
        }
    }

    private func capability(_ icon: String, _ text: String) -> some View {
        HStack(spacing: 6) {
            Image(systemName: icon).font(.system(size: 12)).foregroundStyle(Color.eidSuccess)
            Text(text).font(.system(size: 12)).foregroundStyle(Color.textSecondary)
        }
    }

    private func deviceName(_ platform: String) -> String {
        switch platform {
        case "IOS": return "iPhone"
        case "ANDROID": return "Samsung Galaxy"
        default: return platform
        }
    }
}

// MARK: - /dashboard/security — Аюулгүй байдал

struct SecurityView: View {
    @EnvironmentObject private var appState: AppState
    @ObservedObject private var loc = LocalizationService.shared
    private var data: DashboardData? { appState.dashboardData }

    var body: some View {
        EidScroll {
            EidPageHeader(title: loc.t("Nav_Security"),
                          subtitle: loc.pick("Аюулгүй байдлын тойм", "Security overview", "Обзор безопасности", "安全概览"))
            LazyVGrid(columns: [GridItem(.flexible()), GridItem(.flexible()), GridItem(.flexible())], spacing: 14) {
                EidStatCard(icon: "checkmark.shield", label: loc.pick("KYC төлөв", "KYC status", "Статус KYC", "KYC 状态"),
                            value: data?.user.kycLevel.uppercased() ?? "—", accent: .eidSuccess)
                EidStatCard(icon: "key", label: loc.pick("Идэвхтэй сертификат", "Active certificates", "Действующие сертификаты", "有效证书"),
                            value: "\(data?.certificates ?? 0)")
                EidStatCard(icon: "iphone", label: loc.pick("Бүртгэлтэй төхөөрөмж", "Registered devices", "Зарегистрированные устройства", "已注册设备"),
                            value: "\(data?.devices.count ?? 0)")
            }
            if let devices = data?.devices, !devices.isEmpty {
                AppCard {
                    VStack(alignment: .leading, spacing: 10) {
                        Text(loc.pick("Төхөөрөмжүүд", "Devices", "Устройства", "设备"))
                            .font(.system(size: 14, weight: .semibold)).foregroundStyle(Color.textPrimary)
                        ForEach(devices) { d in
                            HStack {
                                Text("\(d.platform) · \(String(d.deviceId.prefix(12)))…")
                                    .font(.system(size: 13)).foregroundStyle(Color.textSecondary)
                                Spacer()
                                StatusPill(d.status, variant: d.status == "ACTIVE" ? .ok : .warn)
                            }
                        }
                    }
                }
            }
            InlineBanner(text: loc.pick(
                "PIN код солих болон биометр тохиргоог e-ID Mongolia мобайл апп дотроос хийнэ.",
                "Change your PIN and biometrics from the e-ID Mongolia mobile app.",
                "PIN-код и биометрию можно изменить в мобильном приложении e-ID Mongolia.",
                "请在 e-ID Mongolia 移动应用中修改 PIN 码与生物识别设置。"),
                variant: .info)
        }
    }
}

// MARK: - /dashboard/organizations — Миний байгууллага (зөвхөн унших)

/// Мобайл апп-ын "Миний байгууллагууд"-ын зөвхөн-унших хувилбар. Холбох/салгах нь
/// улсын бүртгэлийн шалгалт + PIN2 шаарддаг тул ЗӨВХӨН утсан дээр (docs/ORG_LOGIN.md).
struct OrganizationsView: View {
    @EnvironmentObject private var appState: AppState
    @ObservedObject private var loc = LocalizationService.shared

    var body: some View {
        EidScroll {
            EidPageHeader(title: loc.t("Nav_MyOrganizations"),
                          subtitle: loc.pick("Таны төлөөлж чадах байгууллагууд",
                                             "Organizations you can represent",
                                             "Организации, которые вы можете представлять",
                                             "您可代表的组织"))
            if appState.organizations.isEmpty {
                EidReadOnlyEmpty(
                    icon: "building.2",
                    title: loc.pick("Холбосон байгууллага алга", "No linked organizations",
                                    "Нет связанных организаций", "没有已关联的组织"),
                    text: loc.pick(
                        "Улсын бүртгэлд захирал эсвэл үүсгэн байгуулагчаар бүртгэлтэй байгууллагаа мобайл апп-аас холбоно.",
                        "Link an organization where the state registry lists you as director or founder — from the mobile app.",
                        "Свяжите организацию, где госреестр указывает вас директором или учредителем — в мобильном приложении.",
                        "请在移动应用中关联国家登记中将您列为董事或创始人的组织。"))
            } else {
                ForEach(appState.organizations) { org in orgCard(org) }
            }
            EidReadOnlyNote()
        }
    }

    private func orgCard(_ org: Representation) -> some View {
        AppCard {
            VStack(alignment: .leading, spacing: 14) {
                HStack(alignment: .top) {
                    HStack(spacing: 14) {
                        Image(systemName: "building.2")
                            .font(.system(size: 22))
                            .foregroundStyle(Color.eidAccent)
                            .frame(width: 46, height: 46)
                            .background(Color.eidAccent.opacity(0.12))
                            .clipShape(RoundedRectangle(cornerRadius: 12, style: .continuous))
                        VStack(alignment: .leading, spacing: 3) {
                            Text(org.orgName).font(.system(size: 16, weight: .bold))
                                .foregroundStyle(Color.textPrimary)
                            Text(org.orgRegister).font(.system(size: 12))
                                .foregroundStyle(Color.textSecondary)
                        }
                    }
                    Spacer()
                    // ADMIN — зурагч нэмж/хасна; MANAGER — зөвхөн зурна.
                    StatusPill(org.rightType, variant: org.rightType == "ADMIN" ? .ok : .accent)
                }
                Divider()
                LazyVGrid(columns: Array(repeating: GridItem(.flexible()), count: 2), spacing: 12) {
                    EidField(label: loc.pick("Албан тушаал", "Position", "Должность", "职位"),
                             value: org.role ?? "")
                    EidField(label: loc.pick("Эх сурвалж", "Source", "Источник", "来源"), value: org.source)
                }
            }
        }
    }
}

// MARK: - /dashboard/children — Миний хүүхдүүд (зөвхөн унших)

/// Мобайл апп-ын "Миний хүүхдүүд"-ын зөвхөн-унших хувилбар. Хүүхэд бүртгэх/цуцлах нь
/// асран хамгаалагчийн PIN2 зөвшөөрөл шаарддаг тул ЗӨВХӨН утсан дээр (docs/CHILD_EID.md).
struct ChildrenView: View {
    @EnvironmentObject private var appState: AppState
    @ObservedObject private var loc = LocalizationService.shared

    var body: some View {
        EidScroll {
            EidPageHeader(title: loc.t("Nav_Children"),
                          subtitle: loc.pick("Таны зөвшөөрлөөр eID авсан хүүхдүүд",
                                             "Children who received an eID with your consent",
                                             "Дети, получившие eID с вашего согласия",
                                             "在您同意下获得 eID 的子女"))
            if appState.children.isEmpty {
                EidReadOnlyEmpty(
                    icon: "figure.2.and.child.holdinghands",
                    title: loc.pick("Бүртгэлтэй хүүхэд алга", "No registered children",
                                    "Нет зарегистрированных детей", "没有已登记的子女"),
                    text: loc.pick(
                        "Насанд хүрээгүй хүүхэд зөвхөн НЭВТРЭХ эрхтэй eID авна. Мобайл апп → Хүүхэд нэмэх.",
                        "A minor receives a sign-in only eID. Mobile app → Add a child.",
                        "Несовершеннолетний получает eID только для входа. Мобильное приложение → Добавить ребёнка.",
                        "未成年人仅可获得用于登录的 eID。移动应用 → 添加子女。"))
            } else {
                ForEach(appState.children) { child in childCard(child) }
            }
            EidReadOnlyNote()
        }
    }

    private func childCard(_ c: PersonChild) -> some View {
        AppCard {
            VStack(alignment: .leading, spacing: 14) {
                HStack(alignment: .top) {
                    HStack(spacing: 14) {
                        Image(systemName: "person.crop.circle")
                            .font(.system(size: 22))
                            .foregroundStyle(Color.eidAccent)
                            .frame(width: 46, height: 46)
                            .background(Color.eidAccent.opacity(0.12))
                            .clipShape(RoundedRectangle(cornerRadius: 12, style: .continuous))
                        VStack(alignment: .leading, spacing: 3) {
                            Text(c.name.isEmpty ? c.regNo : c.name)
                                .font(.system(size: 16, weight: .bold))
                                .foregroundStyle(Color.textPrimary)
                            Text(c.regNo).font(.system(size: 12))
                                .foregroundStyle(Color.textSecondary)
                        }
                    }
                    Spacer()
                    // Зөвшөөрөл өгсөн ч хүүхэд утсан дээрээ бүртгэлээ дуусгаагүй байж болно.
                    StatusPill(c.registered ? loc.pick("Бүртгэлтэй", "Registered", "Зарегистрирован", "已登记")
                                            : loc.pick("Хүлээгдэж буй", "Pending", "Ожидает", "待处理"),
                               variant: c.registered ? .ok : .warn)
                }
                Divider()
                LazyVGrid(columns: Array(repeating: GridItem(.flexible()), count: 2), spacing: 12) {
                    EidField(label: loc.pick("Төрсөн огноо", "Date of birth", "Дата рождения", "出生日期"),
                             value: c.birthDate ?? "")
                    EidField(label: loc.pick("Гэрчилгээ дуусах", "Certificate expires",
                                             "Сертификат истекает", "证书到期"),
                             value: eidFmtDateLong(c.certNotAfter, loc.language))
                }
            }
        }
    }
}

/// Зөвхөн-унших хуудсуудын хоосон төлөв.
private struct EidReadOnlyEmpty: View {
    let icon: String
    let title: String
    let text: String
    var body: some View {
        AppCard {
            VStack(spacing: 10) {
                Image(systemName: icon).font(.system(size: 30)).foregroundStyle(Color.textSecondary)
                Text(title).font(.system(size: 16, weight: .semibold)).foregroundStyle(Color.textPrimary)
                Text(text).font(.system(size: 13)).foregroundStyle(Color.textSecondary)
                    .multilineTextAlignment(.center).frame(maxWidth: 460)
            }
            .frame(maxWidth: .infinity).padding(.vertical, 18)
        }
    }
}

/// Desktop дээр эдгээр жагсаалт нь нэвтрэх агшны snapshot — өөрчлөлт нь утсаар л хийгдэнэ.
private struct EidReadOnlyNote: View {
    @ObservedObject private var loc = LocalizationService.shared
    var body: some View {
        InlineBanner(text: loc.pick(
            "Энэ хуудас зөвхөн харуулна. Нэмэх, салгах, цуцлах үйлдлийг e-ID Mongolia мобайл апп дотроос (PIN2) хийнэ. Жагсаалт нэвтэрсэн үедээ шинэчлэгддэг.",
            "This page is read-only. Add, unlink and revoke happen in the e-ID Mongolia mobile app (PIN2). The list refreshes when you sign in.",
            "Эта страница только для чтения. Добавление, отвязка и отзыв выполняются в мобильном приложении e-ID Mongolia (PIN2). Список обновляется при входе.",
            "此页面仅供查看。添加、解绑与吊销请在 e-ID Mongolia 移动应用中完成（PIN2）。列表在登录时刷新。"),
            variant: .info)
    }
}
