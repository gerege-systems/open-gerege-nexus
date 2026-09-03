import SwiftUI
import UIKit

// MARK: - Утасны дизайны токенууд
//
// Эх сурвалж: Gerege Wallet-ийн «Real App Mirror» токен багц
// (`wallet-gerege-mn/android/.../ui/theme/Color.kt`). Тэр репод iOS ба
// Android хоёр бага зэрэг зөрсөн (accent emerald vs монгол тугны ногоон,
// tab улбар шар vs …) — энд ЗӨВХӨН Android талын багцыг авав: тэр нь
// прототипээс шууд гаралтай, гурван гэр бүл (neutral / brand / semantic)
// бүрэн дүүрэн. Хоёр утас нэг палитраас уншина.
//
// **Яагаад энэ файл ширээнийхээс ТУСДАА вэ.** `desktop/macos/Design/` нь
// macOS апптай ХУВААЛЦСАН (`project.yml` шууд эх файлаар оруулна) бөгөөд
// Android-ын `EidColors.kt` нь түүнээс ҮҮСГЭГДДЭГ. Тэнд гар хүрвэл ширээний
// апп болон Windows-той нийцлийн CI шалгалт хоёулаа хөдөлнө. Утасны
// харагдацыг солих ажил ширээний апп руу давлагаа явуулах ёсгүй тул
// токенууд энд, `Theme.` нэрийн зайд амьдарна.
//
// Ширээнийх глобал `Space`, `Radius` нэрийг аль хэдийн эзэмшсэн (Styles.swift)
// тул энд бүх зүйл `Theme` дотор нэрлэгдэнэ — мөргөлдөхгүй.

enum Theme {

    // MARK: Neutral — суурь давхарга (--bg, --surface-1..3, --fg-1..4)

    /// Хуудасны дэвсгэр.
    static let bg = dyn(Color(hex: "F4F6FB"), Color(hex: "0B0E11"))
    /// Картын гадаргуу — гол сав.
    static let surface1 = dyn(Color(hex: "FFFFFF"), Color(hex: "12161C"))
    /// Карт доторх үүрлэсэн блок.
    static let surface2 = dyn(Color(hex: "F4F6FB"), Color(hex: "1A1F27"))
    /// Хамгийн гүн үүр — оролтын талбар, дүрсний тойрог.
    static let surface3 = dyn(Color(hex: "E9EDF5"), Color(hex: "232A34"))

    /// Үндсэн бичвэр.
    static let fg1 = dyn(Color(hex: "0E1525"), Color(hex: "EAECEF"))
    /// Хоёрдогч бичвэр.
    static let fg2 = dyn(Color(hex: "4A5468"), Color(hex: "B7BDC6"))
    /// Шошго, тайлбар.
    static let fg3 = dyn(Color(hex: "8892A6"), Color(hex: "707A8A"))
    /// Идэвхгүй.
    static let fg4 = dyn(Color(hex: "C2CADA"), Color(hex: "4B5563"))

    static let border       = dyn(Color(hex: "0E1525").opacity(0.08), Color.white.opacity(0.06))
    static let borderStrong = dyn(Color(hex: "0E1525").opacity(0.16), Color.white.opacity(0.12))
    static let divider      = dyn(Color(hex: "0E1525").opacity(0.06), Color.white.opacity(0.04))

    // MARK: Brand — горимоос үл хамаарна

    enum Brand {
        static let primary = Color(hex: "3178EC")
        static let deep    = Color(hex: "1F5FD0")
        /// Мэдээллийн самбар, идэвхтэй таб — гэрэлд 10%, харанхуйд 14%.
        static let soft = dyn(Color(hex: "3178EC").opacity(0.10),
                              Color(hex: "3178EC").opacity(0.14))
        static let line = Color(hex: "3178EC").opacity(0.40)
        static let glow = Color(hex: "3178EC").opacity(0.35)
        static let onBrand = Color.white
        static let gradient = LinearGradient(
            colors: [Color(hex: "3178EC"), Color(hex: "1F5FD0")],
            startPoint: .topLeading, endPoint: .bottomTrailing
        )
    }

    // MARK: Semantic — амжилт / алдаа / анхаарал / ховор

    /// Монгол тугны ногоон — баталгаажсан, «болсон».
    static let credit     = Color(hex: "119D52")
    static let creditSoft = dyn(Color(hex: "119D52").opacity(0.10), Color(hex: "119D52").opacity(0.14))
    static let debit      = dyn(Color(hex: "E03A50"), Color(hex: "F6465D"))
    static let debitSoft  = dyn(Color(hex: "E03A50").opacity(0.10), Color(hex: "F6465D").opacity(0.12))
    /// Дулаан улбар шар — сонгогдсон таб, анхааруулга.
    static let accent     = Color(hex: "FE6401")
    static let accentSoft = dyn(Color(hex: "FE6401").opacity(0.10), Color(hex: "FE6401").opacity(0.14))
    static let gold       = Color(hex: "E0A82E")
    static let goldSoft   = dyn(Color(hex: "E0A82E").opacity(0.10), Color(hex: "E0A82E").opacity(0.14))

    // MARK: Зай, радиус
    //
    // 4pt суурь, ~25% алхам (Refactoring UI). Цэснээс сонгоно, шинийг
    // зохиохгүй — тэр нь 8/10/12/14/16 хольсон одоогийн байдлыг давтна.

    enum Space {
        static let xxs: CGFloat = 2
        static let xs:  CGFloat = 4
        static let sm:  CGFloat = 8
        static let md:  CGFloat = 12
        static let lg:  CGFloat = 16
        static let xl:  CGFloat = 24
        static let xxl: CGFloat = 32
        static let xxxl: CGFloat = 48
    }

    enum Radius {
        static let sm:  CGFloat = 8
        static let md:  CGFloat = 12
        static let lg:  CGFloat = 16
        static let xl:  CGFloat = 20
        static let xxl: CGFloat = 24
    }

    // MARK: Фонт
    //
    // Montserrat нь `Resources/Fonts/`-оос багцлагдаж `project.yml`-ийн
    // `UIAppFonts`-д бүртгэгдсэн. `Font.custom` нь ФАЙЛЫН нэрээр биш
    // PostScript нэрээр хайдаг тул тэр буулгалт энд ганц газар байна.
    //
    // Тоо (регистр, баримтын дугаар, код) нь системийн `.monospaced`
    // хэвээр — Montserrat-д tabular figure байхгүй тул баганаар
    // эгнэхгүй, харьцуулах ёстой 6 оронтой код бүдгэрнэ.

    enum TypeScale {
        static let display     = Font.custom("Montserrat-Bold",     size: 34, relativeTo: .largeTitle)
        static let title       = Font.custom("Montserrat-Bold",     size: 28, relativeTo: .title)
        static let title2      = Font.custom("Montserrat-Bold",     size: 22, relativeTo: .title2)
        static let title3      = Font.custom("Montserrat-SemiBold", size: 20, relativeTo: .title3)

        static let headline    = Font.custom("Montserrat-SemiBold", size: 17, relativeTo: .headline)
        static let body        = Font.custom("Montserrat-Regular",  size: 15, relativeTo: .body)
        static let bodyBold    = Font.custom("Montserrat-SemiBold", size: 15, relativeTo: .body)
        static let callout     = Font.custom("Montserrat-Medium",   size: 14, relativeTo: .callout)
        static let calloutBold = Font.custom("Montserrat-SemiBold", size: 14, relativeTo: .callout)

        static let footnote     = Font.custom("Montserrat-Regular",  size: 13, relativeTo: .footnote)
        static let footnoteBold = Font.custom("Montserrat-SemiBold", size: 13, relativeTo: .footnote)
        static let caption      = Font.custom("Montserrat-Regular",  size: 12, relativeTo: .caption)
        static let captionBold  = Font.custom("Montserrat-SemiBold", size: 12, relativeTo: .caption)
        static let caption2     = Font.custom("Montserrat-Regular",  size: 11, relativeTo: .caption2)
        static let caption2Bold = Font.custom("Montserrat-SemiBold", size: 11, relativeTo: .caption2)

        /// Том үсгээр, зай нэмсэн хэсгийн шошго («РЕГИСТРИЙН ДУГААР»).
        /// `tracking(1.2)`-ыг дуудлагын талд тавина.
        static let sectionLabel = Font.custom("Montserrat-SemiBold", size: 11, relativeTo: .caption2)

        static let statValue = Font.custom("Montserrat-Bold", size: 26, relativeTo: .title)
        static let mono      = Font.system(.footnote, design: .monospaced)
        static let monoSm    = Font.system(.caption2, design: .monospaced)
        /// Баталгаажуулах код — хоёр аппын хооронд тулгах ёстой тул
        /// системийн rounded, tabular.
        static let codeHero  = Font.system(size: 30, design: .rounded).weight(.bold).monospacedDigit()
    }

    // MARK: Дотоод

    /// Гэрэл/харанхуйд өөр өөр Color. Ширээний `Color.dynamic(light:dark:)`
    /// нь ЗӨВХӨН hex мөр авдаг тул alpha-тай токенуудад хүрэлцэхгүй —
    /// энэ нь бэлэн `Color` хоёрыг авна.
    fileprivate static func dyn(_ light: Color, _ dark: Color) -> Color {
        Color(uiColor: UIColor { trait in
            UIColor(trait.userInterfaceStyle == .dark ? dark : light)
        })
    }
}

// MARK: - Өндөрлөг
//
// Гэрэл дээрээс: нэг зөөлөн том сүүдэр, нэг чанга жижиг. Дөрвөн нэртэй
// түвшин — карт бүр `.shadow(radius: 4)` гэж өөрийгөө зохиохгүй.

extension View {
    @ViewBuilder
    func elevation(_ level: Int) -> some View {
        switch level {
        case 1:
            self.shadow(color: .black.opacity(0.05), radius: 1, y: 1)
                .shadow(color: .black.opacity(0.03), radius: 3, y: 2)
        case 2:
            self.shadow(color: .black.opacity(0.06), radius: 2, y: 2)
                .shadow(color: .black.opacity(0.05), radius: 8, y: 4)
        case 3:
            self.shadow(color: .black.opacity(0.08), radius: 4, y: 3)
                .shadow(color: .black.opacity(0.06), radius: 16, y: 8)
        default:
            self
        }
    }
}
