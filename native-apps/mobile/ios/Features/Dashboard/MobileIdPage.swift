import SwiftUI

/// Миний ID — ширээний `MyIdView`-ийн гар дээрх хувилбар.
///
/// Ижил эх сурвалж (`AppState.dashboardData?.user`), ижил талбарууд. Ялгаа нь
/// зохион байгуулалт: гар дээр хоёр баганат тор биш дараалсан мөр.
struct MobileIdPage: View {
    @EnvironmentObject private var appState: AppState
    @ObservedObject private var loc = LocalizationService.shared

    private var user: DashboardUser? { appState.dashboardData?.user }

    var body: some View {
        MobilePage(title: loc.t("Nav_MyId"),
                   subtitle: loc.pick("Танай иргэний цахим үнэмлэхний нэгдсэн профайл",
                                      "Your unified e-ID profile",
                                      "Ваш единый профиль e-ID",
                                      "您的统一 e-ID 档案")) {
            if let user {
                identityCard(user)
                certificateCard(user)
                if !appState.organizations.isEmpty { organizationsCard }
                if !appState.children.isEmpty { childrenCard }
            } else {
                BrandCard {
                    Text(loc.t("Dashboard_Activity_Empty"))
                        .font(Theme.TypeScale.footnote)
                        .foregroundStyle(Theme.fg3)
                }
            }
        }
    }

    private func identityCard(_ user: DashboardUser) -> some View {
        BrandCard(spacing: Theme.Space.lg) {
            HStack(spacing: Theme.Space.md) {
                UserAvatar(photo: user.photo, initials: initials(user.displayName), size: 64,
                           tint: Theme.Brand.primary,
                           tintBackground: Theme.Brand.soft)
                VStack(alignment: .leading, spacing: Theme.Space.sm) {
                    Text(user.displayName)
                        .font(Theme.TypeScale.title3)
                        .foregroundStyle(Theme.fg1)
                    BrandPill(text: user.status.lowercased() == "active"
                              ? loc.pick("Идэвхтэй", "Active", "Активен", "有效") : user.status,
                              tone: user.status.lowercased() == "active" ? .ok : .warn)
                }
                Spacer(minLength: 0)
            }
            Rectangle().fill(Theme.divider).frame(height: 1)
            MobileField(label: loc.pick("Регистрийн дугаар", "Registration number",
                                        "Регистрационный номер", "登记号"),
                        value: user.nationalId.isEmpty ? "—" : user.nationalId, mono: true)
            if let civil = user.civilId, !civil.isEmpty {
                MobileField(label: loc.pick("Иргэний бүртгэлийн дугаар", "Civil ID",
                                            "Гражданский ID", "公民号"),
                            value: civil, mono: true)
            }
        }
    }

    private func certificateCard(_ user: DashboardUser) -> some View {
        BrandCard {
            BrandSectionLabel(text: loc.pick("Гэрчилгээ", "Certificate", "Сертификат", "证书"))
            MobileField(label: loc.pick("Түвшин", "Level", "Уровень", "等级"),
                        value: user.kycLevel.isEmpty ? "—" : user.kycLevel)
            // Гэрчилгээний сериал: платформ нь eID-ийн `documentNumber`-ыг
            // дээшээ гаргадаггүй бөгөөд иргэн ЯМАР гэрчилгээгээр зөвшөөрснийг
            // заадаг нь энэ (`MobileLoginView.finish`).
            MobileField(label: loc.pick("Гэрчилгээний сериал", "Certificate serial",
                                        "Серийный номер сертификата", "证书序列号"),
                        value: appState.documentNumber.isEmpty ? "—" : appState.documentNumber, mono: true)
        }
    }

    private var organizationsCard: some View {
        BrandCard {
            Text(loc.t("Nav_MyOrganizations"))
                .font(Theme.TypeScale.calloutBold)
                .foregroundStyle(Theme.fg1)
            ForEach(appState.organizations) { org in
                listRow(icon: "building.2",
                        title: org.orgName,
                        subtitle: org.orgRegister) {
                    BrandPill(text: org.rightType, tone: .brand)
                }
            }
            // Ширээнийхтэй ижил дүрэм: энэ жагсаалт ЗӨВХӨН УНШИХ.
            EidReadOnlyHint()
        }
    }

    private var childrenCard: some View {
        BrandCard {
            Text(loc.t("Nav_Children"))
                .font(Theme.TypeScale.calloutBold)
                .foregroundStyle(Theme.fg1)
            ForEach(appState.children) { child in
                listRow(icon: "figure.child",
                        title: child.name,
                        subtitle: child.regNo) {
                    BrandPill(text: child.registered
                              ? loc.pick("Бүртгэлтэй", "Registered", "Зарегистрирован", "已注册")
                              : loc.pick("Хүлээгдэж буй", "Pending", "Ожидает", "待处理"),
                              tone: child.registered ? .ok : .warn)
                }
            }
            EidReadOnlyHint()
        }
    }

    /// Байгууллага ба хүүхдийн мөр нэг л хэлбэртэй — дүрс, нэр, дугаар, капсул.
    private func listRow<Trailing: View>(icon: String, title: String, subtitle: String,
                                         @ViewBuilder trailing: () -> Trailing) -> some View {
        HStack(spacing: Theme.Space.md) {
            Image(systemName: icon)
                .font(.system(size: 13))
                .foregroundStyle(Theme.Brand.primary)
                .frame(width: 30, height: 30)
                .background(Theme.Brand.soft, in: Circle())
            VStack(alignment: .leading, spacing: 2) {
                Text(title)
                    .font(Theme.TypeScale.footnoteBold)
                    .foregroundStyle(Theme.fg1)
                Text(subtitle)
                    .font(Theme.TypeScale.monoSm)
                    .foregroundStyle(Theme.fg3)
            }
            Spacer(minLength: 0)
            trailing()
        }
    }

    private func initials(_ name: String) -> String {
        name.split(separator: " ").prefix(2).compactMap { $0.first }.map(String.init).joined().uppercased()
    }
}

/// «Энэ жагсаалтыг зөвхөн утсан дээрх eID апп өөрчилнө» — ширээний
/// `EidReadOnlyNote`-ийн богино хувилбар. Гар дээр газар бага тул нэг мөр.
struct EidReadOnlyHint: View {
    @ObservedObject private var loc = LocalizationService.shared
    var body: some View {
        Text(loc.pick("Зөвхөн харуулна — өөрчлөлтийг eID Mongolia апп дотроос (PIN2) хийнэ.",
                      "Read-only — changes happen in the eID Mongolia app (PIN2).",
                      "Только просмотр — изменения выполняются в приложении eID Mongolia (PIN2).",
                      "仅供查看——更改请在 eID Mongolia 应用中完成（PIN2）。"))
            .font(Theme.TypeScale.caption2)
            .foregroundStyle(Theme.fg3)
    }
}
