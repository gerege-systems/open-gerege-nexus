import SwiftUI

/// Нэвтэрсэн үеийн бүрхүүл — ширээний sidebar-ын гар дээрх дүйцэл.
///
/// Хажуугийн цэс биш TabView байгаа нь гоо сайхны сонголт биш: гар дээр
/// эрхий хуруунд хүрэх зурвас доор байдаг. Ширээний нэр томьёо (`Nav_*` түлхүүр,
/// SF Symbol) хэвээр — хоёр клиент нэг зүйлийг нэг нэрээр дуудна.
struct MainTabView: View {
    @EnvironmentObject private var appState: AppState
    @ObservedObject private var loc = LocalizationService.shared

    var body: some View {
        TabView {
            MobileDashboardPage()
                .tabItem { Label(loc.t("Nav_Dashboard"), systemImage: "house") }
            MobileIdPage()
                .tabItem { Label(loc.t("Nav_MyId"), systemImage: "person.text.rectangle") }
            MobileLogsPage()
                .tabItem { Label(loc.t("Nav_Logs"), systemImage: "clock.arrow.circlepath") }
            MobileSettingsPage()
                .tabItem { Label(loc.t("Nav_Settings"), systemImage: "gearshape") }
        }
        // Сонгогдсон таб нь БРЭНДИЙН цэнхэр биш, дулаан улбар шар. Дотор
        // талын карт, товч, холбоос бүр брэндийн цэнхэр тул табыг мөн цэнхэр
        // болговол «аль нь идэвхтэй вэ» гэдэг ялгарахаа болино (Apple HIG).
        .tint(Theme.accent)
    }
}

/// Дэлгэц бүрийн нийтлэг хүрээ: гарчиг + гүйлгэх муж + ижил дэвсгэр.
///
/// Системийн `navigationTitle` БИШ, гарчгаа өөрөө зурдаг нь санаатай. UIKit-ийн
/// navigation bar нь SwiftUI-аас фонт, өнгө авдаггүй — `UINavigationBarAppearance`
/// гэсэн бүхэл давхаргыг апп даяар тааруулж байж л Montserrat орно. Android-ын
/// `EidScreen` нь гарчгаа ингэж зурдаг тул ийнхүү хоёр платформ ЯГ ижил болж,
/// нэг платформын хачирхалтай зан урсгалаас хасагдана.
struct MobilePage<Content: View>: View {
    let title: String
    let subtitle: String?
    @ViewBuilder var content: () -> Content

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: Theme.Space.md) {
                Text(title)
                    .font(Theme.TypeScale.title)
                    .foregroundStyle(Theme.fg1)
                if let subtitle, !subtitle.isEmpty {
                    Text(subtitle)
                        .font(Theme.TypeScale.footnote)
                        .foregroundStyle(Theme.fg3)
                        .padding(.bottom, Theme.Space.xxs)
                }
                content()
            }
            .padding(.horizontal, Theme.Space.lg)
            .padding(.vertical, Theme.Space.lg)
            .frame(maxWidth: .infinity, alignment: .leading)
        }
        .background(Theme.bg.ignoresSafeArea())
    }
}
