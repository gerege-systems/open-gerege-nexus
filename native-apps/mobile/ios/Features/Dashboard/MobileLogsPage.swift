import SwiftUI

/// Лог түүх — ширээний `LogsView`-ийн гар дээрх хувилбар.
///
/// Эх сурвалж нь ижил: v3 RP-API нь иргэний бүх session-ий түүхийг өгдөггүй тул
/// энэ жагсаалт бол ЭНЭ ТӨХӨӨРӨМЖ дээрх үйлдлийн локал бүртгэл
/// (`UserDefaults["activity.sessions"]`). Тиймээс утас, мак хоёр өөр өөр
/// жагсаалт харуулах нь ХЭВИЙН — тэдгээр нь өөр өөр төхөөрөмжийн түүх.
struct MobileLogsPage: View {
    @EnvironmentObject private var appState: AppState
    @ObservedObject private var loc = LocalizationService.shared

    var body: some View {
        MobilePage(title: loc.t("Nav_Logs"),
                   subtitle: loc.pick("Энэ төхөөрөмж дээрх нэвтрэлт, гарын үсгийн бүртгэл",
                                      "Sign-in and signature activity on this device",
                                      "Входы и подписи на этом устройстве",
                                      "本设备上的登录与签名记录")) {
            let sessions = appState.dashboardData?.sessions ?? []
            if sessions.isEmpty {
                BrandCard {
                    Text(loc.t("Dashboard_Activity_Empty"))
                        .font(Theme.TypeScale.footnote)
                        .foregroundStyle(Theme.fg3)
                }
            } else {
                ForEach(sessions) { session in
                    BrandCard(padding: Theme.Space.md) {
                        HStack(spacing: Theme.Space.md) {
                            Image(systemName: session.sessionType == "AUTH" ? "arrow.right.circle" : "signature")
                                .font(.system(size: 15))
                                .foregroundStyle(Theme.Brand.primary)
                                .frame(width: 36, height: 36)
                                .background(Theme.Brand.soft,
                                            in: RoundedRectangle(cornerRadius: Theme.Radius.md, style: .continuous))
                            VStack(alignment: .leading, spacing: 3) {
                                Text(session.sessionType == "AUTH"
                                     ? loc.t("Dashboard_Activity_Auth") : loc.t("Dashboard_Activity_Sign"))
                                    .font(Theme.TypeScale.calloutBold)
                                    .foregroundStyle(Theme.fg1)
                                Text(session.rpName)
                                    .font(Theme.TypeScale.caption)
                                    .foregroundStyle(Theme.fg2)
                                Text(MobileDashboardPage.shortDate(session.createdAt))
                                    .font(Theme.TypeScale.monoSm)
                                    .foregroundStyle(Theme.fg3)
                            }
                            Spacer(minLength: 0)
                            BrandPill(text: session.result == "OK"
                                      ? loc.t("Dashboard_Activity_Success") : loc.t("Dashboard_Activity_Failure"),
                                      tone: session.result == "OK" ? .ok : .warn)
                        }
                    }
                }
            }
        }
    }
}
