import SwiftUI

/// Самбар — ширээний `DashboardPageView`-ийн гар дээрх хувилбар.
///
/// Агуулга нь ЯГ ижил эх сурвалжаас: identity + локал үйл ажиллагааны лог
/// (`AppState.dashboardData`). Ялгаа нь зохион байгуулалт — 4 баганат тор
/// биш 2 баганат, hero нь босоо.
///
/// Hero нь брэндийн градиент карт: wallet дээр тэр байрыг үлдэгдэл эзэлдэг,
/// энд иргэний өөрийнх нь мөр эзэлнэ — аль ч аппыг нээхэд эхний зүйл нь
/// «би хэн бэ / надад юу байна» гэдэг нэг ижил хэлбэрээр угтана.
struct MobileDashboardPage: View {
    @EnvironmentObject private var appState: AppState
    @ObservedObject private var loc = LocalizationService.shared

    var body: some View {
        MobilePage(title: loc.t("Nav_Dashboard"), subtitle: nil) {
            hero
            stats
            activity
        }
    }

    private var hero: some View {
        VStack(alignment: .leading, spacing: Theme.Space.lg) {
            HStack(spacing: Theme.Space.md) {
                UserAvatar(photo: appState.dashboardData?.user.photo,
                           initials: initials, size: 56,
                           tint: Theme.Brand.onBrand,
                           tintBackground: Color.white.opacity(0.22))
                VStack(alignment: .leading, spacing: Theme.Space.xs) {
                    Text(appState.fullName.isEmpty ? loc.t("Dashboard_Greeting") : appState.fullName)
                        .font(Theme.TypeScale.title3)
                        .foregroundStyle(Theme.Brand.onBrand)
                        .lineLimit(2)
                    if !appState.civilID.isEmpty || !appState.nationalID.isEmpty {
                        Text(appState.civilID.isEmpty ? appState.nationalID : appState.civilID)
                            .font(Theme.TypeScale.monoSm)
                            .foregroundStyle(Theme.Brand.onBrand.opacity(0.75))
                    }
                }
                Spacer(minLength: 0)
            }
            HStack(spacing: 6) {
                Image(systemName: "checkmark.seal.fill").font(.system(size: 11))
                Text(loc.t("Dashboard_StatusBadge")).font(Theme.TypeScale.caption2Bold)
            }
            .foregroundStyle(Theme.Brand.onBrand)
            .padding(.horizontal, 10).padding(.vertical, 5)
            .background(Color.white.opacity(0.18), in: Capsule())
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(Theme.Space.lg)
        .background(Theme.Brand.gradient)
        .clipShape(RoundedRectangle(cornerRadius: Theme.Radius.lg, style: .continuous))
        .elevation(3)
    }

    private var stats: some View {
        LazyVGrid(columns: [GridItem(.flexible()), GridItem(.flexible())],
                  spacing: Theme.Space.md) {
            stat(loc.t("Dashboard_Stats_Certificates"), "\(appState.dashboardData?.certificates ?? 0)", "checkmark.seal")
            stat(loc.t("Dashboard_Stats_Logins"), "\(appState.dashboardData?.totalLogins ?? 0)", "arrow.right.circle")
            stat(loc.t("Nav_MyOrganizations"), "\(appState.organizations.count)", "building.2")
            stat(loc.t("Nav_Children"), "\(appState.children.count)", "figure.2.and.child.holdinghands")
        }
    }

    private func stat(_ label: String, _ value: String, _ icon: String) -> some View {
        BrandCard(padding: Theme.Space.md, spacing: Theme.Space.sm) {
            Image(systemName: icon)
                .font(.system(size: 14, weight: .semibold))
                .foregroundStyle(Theme.Brand.primary)
                .frame(width: 32, height: 32)
                .background(Theme.Brand.soft,
                            in: RoundedRectangle(cornerRadius: Theme.Radius.sm, style: .continuous))
            Text(value)
                .font(Theme.TypeScale.statValue)
                .foregroundStyle(Theme.fg1)
            Text(label)
                .font(Theme.TypeScale.caption2)
                .foregroundStyle(Theme.fg3)
                .lineLimit(2)
        }
    }

    private var activity: some View {
        BrandCard {
            Text(loc.t("Dashboard_Activity_Section"))
                .font(Theme.TypeScale.calloutBold)
                .foregroundStyle(Theme.fg1)
            let sessions = appState.dashboardData?.sessions.prefix(5) ?? []
            if sessions.isEmpty {
                Text(loc.t("Dashboard_Activity_Empty"))
                    .font(Theme.TypeScale.footnote)
                    .foregroundStyle(Theme.fg3)
            } else {
                ForEach(Array(sessions), id: \.id) { session in
                    HStack(spacing: Theme.Space.md) {
                        Image(systemName: session.sessionType == "AUTH" ? "arrow.right.circle" : "signature")
                            .font(.system(size: 13))
                            .foregroundStyle(Theme.Brand.primary)
                            .frame(width: 30, height: 30)
                            .background(Theme.Brand.soft, in: Circle())
                        VStack(alignment: .leading, spacing: 2) {
                            Text(session.rpName)
                                .font(Theme.TypeScale.footnoteBold)
                                .foregroundStyle(Theme.fg1)
                            Text(Self.shortDate(session.createdAt))
                                .font(Theme.TypeScale.monoSm)
                                .foregroundStyle(Theme.fg3)
                        }
                        Spacer(minLength: 0)
                        BrandPill(text: session.result, tone: session.result == "OK" ? .ok : .warn)
                    }
                }
            }
        }
    }

    private var initials: String {
        let parts = appState.fullName.split(separator: " ").prefix(2)
        return parts.compactMap { $0.first }.map(String.init).joined().uppercased()
    }

    static func shortDate(_ iso: String) -> String {
        guard let date = ISO8601DateFormatter().date(from: iso) else { return iso }
        let f = DateFormatter()
        f.dateFormat = "yyyy.MM.dd HH:mm"
        return f.string(from: date)
    }
}
