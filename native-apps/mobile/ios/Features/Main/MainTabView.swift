import SwiftUI

/// Нэвтэрсэн үеийн бүрхүүл — ширээний sidebar-ын гар дээрх дүйцэл.
///
/// Хажуугийн цэс биш TabView байгаа нь гоо сайхны сонголт биш: гар дээр
/// эрхий хуруунд хүрэх зурвас доор байдаг. Ширээний нэр томьёо (`Nav_*` түлхүүр,
/// SF Symbol) хэвээр — хоёр клиент нэг зүйлийг нэг нэрээр дуудна.
struct MainTabView: View {
    @EnvironmentObject private var appState: AppState
    @ObservedObject private var loc = LocalizationService.shared

    /// Аль таб нээлттэй байх.
    ///
    /// `DashboardTab` БИШ: тэр нь ширээний sidebar-ын жагсаалт бөгөөд түүнд
    /// `settings` гэсэн бичлэг байдаггүй (Мак дээр тохиргоо нь sheet). Гар
    /// дээрх зурвас өөрийн таван нэртэй.
    ///
    /// `EID_DEBUG_TAB` нь ЗӨВХӨН Debug build дээр эхлэх табыг сонгоно —
    /// дэлгэцийг зурган дээр шалгах цорын ганц арга. Release-д энэ код
    /// байхгүй тул тойрох зам ч байхгүй.
    @State private var tab: String = {
        #if DEBUG
        if let raw = ProcessInfo.processInfo.environment["EID_DEBUG_TAB"],
           let first = raw.split(separator: ">").first { return String(first) }
        #endif
        return "dashboard"
    }()

    var body: some View {
        TabView(selection: $tab) {
            MobileDashboardPage()
                .tabItem { Label(loc.t("Nav_Dashboard"), systemImage: "house") }
                .tag("dashboard")
            MobileIdPage()
                .tabItem { Label(loc.t("Nav_MyId"), systemImage: "person.text.rectangle") }
                .tag("id")
            MobileLogsPage()
                .tabItem { Label(loc.t("Nav_Logs"), systemImage: "clock.arrow.circlepath") }
                .tag("logs")
            MobileSettingsPage()
                .tabItem { Label(loc.t("Nav_Settings"), systemImage: "gearshape") }
                .tag("settings")
            // Платформын ажлын муж нь ЭНЭ аппын өөрийн дэлгэц биш тул native
            // дэлгэцүүдийн ДАРАА — ширээний sidebar-т ч мөн сүүлд байдаг
            // (`DashboardTab.mainNav`). Байрлал нь хүнд аль нь аль болохыг хэлнэ.
            MobilePlatformPage()
                .tabItem { Label(loc.t("Nav_Platform"), systemImage: "square.grid.2x2") }
                .tag("platform")
        }
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
