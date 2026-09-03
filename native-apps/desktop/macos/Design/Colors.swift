import SwiftUI
#if canImport(AppKit)
import AppKit
#endif
#if canImport(UIKit)
import UIKit
#endif

// MARK: - Brand palette (Windows Colors.xaml Brand50-950)
// Sourced from web/src/app/globals.css. Keep in lockstep with Windows app.
extension Color {
    static let brand50  = Color(hex: "EEF4FF")
    static let brand100 = Color(hex: "D8E6FF")
    static let brand200 = Color(hex: "B3CCFF")
    static let brand300 = Color(hex: "84A8FF")
    static let brand400 = Color(hex: "5B8AFF")
    static let brand500 = Color(hex: "3A6DFF") // primary brand
    static let brand600 = Color(hex: "2A5BD7")
    static let brand700 = Color(hex: "1D47B3")
    static let brand800 = Color(hex: "14358A")
    static let brand900 = Color(hex: "0A2466")
    static let brand950 = Color(hex: "001F5E")
}

// MARK: - Theme-aware brushes (Windows ThemeDictionaries port)
extension Color {

    /// `EidAccentBrush` — Brand500 light / Brand300 dark
    static let eidAccent = Color.dynamic(light: "3A6DFF", dark: "84A8FF")

    /// `EidAccentStrongBrush` — Brand700 light / Brand200 dark
    static let eidAccentStrong = Color.dynamic(light: "1D47B3", dark: "B3CCFF")

    /// `EidAccentSubtleBrush` — Brand100 light / Brand900 dark
    static let eidAccentSubtle = Color.dynamic(light: "D8E6FF", dark: "0A2466")

    /// `EidAccentMutedBrush` — Brand50 light / Brand950 dark
    static let eidAccentMuted = Color.dynamic(light: "EEF4FF", dark: "001F5E")

    /// `EidSuccessBrush`
    static let eidSuccess = Color.dynamic(light: "107C10", dark: "6FCF6F")

    /// `EidWarningBrush`
    static let eidWarning = Color.dynamic(light: "9D5D00", dark: "E8B66A")

    /// `EidDangerBrush`
    static let eidDanger = Color.dynamic(light: "BA1A1A", dark: "FFB4AB")

    /// `EidCardBackground` — White / #1F1F23
    static let eidCardBackground = Color.dynamic(light: "FFFFFF", dark: "1F1F23")  // light: .white

    /// `EidCardStroke` — #E5E7EB / #34343A
    static let eidCardStroke = Color.dynamic(light: "E5E7EB", dark: "34343A")

    /// `EidSurfaceBrush` — page background
    static let eidSurface = Color.dynamic(light: "F9F9FC", dark: "0B0E12")

    /// `EidMutedForeground`
    static let eidMuted = Color.dynamic(light: "6B7280", dark: "9CA3AF")

    /// Pastel light backgrounds for inline banners (Windows hard-coded #FEE2E2 etc.)
    static let bannerErrorBG     = Color(hex: "FEE2E2")
    static let bannerErrorBorder = Color(hex: "FCA5A5")
    static let bannerErrorText   = Color(hex: "7F1D1D")
    static let bannerErrorIcon   = Color(hex: "B91C1C")
    static let bannerSuccessBG   = Color(hex: "DCFCE7")
    static let bannerSuccessIcon = Color(hex: "16A34A")
}

// MARK: - Legacy aliases (existing Views still reference these)
extension Color {
    /// alias → `eidAccent` (was hard-coded #2563EB)
    static let primaryBlue       = Color.eidAccent
    /// alias → `eidAccentStrong`
    static let primaryBlueHover  = Color.eidAccentStrong
    /// alias → Brand400
    static let primaryBlueLight  = Color.brand400
    /// alias → light brand badge accent
    static let accentGold        = Color(hex: "D4A017")

    /// alias → `eidSuccess`
    static let success           = Color.eidSuccess
    /// alias → light success tint
    static let successBG         = Color.bannerSuccessBG
    /// alias → `eidDanger`
    static let destructive       = Color.eidDanger
    /// alias → light danger tint
    static let destructiveBG     = Color.bannerErrorBG
    /// alias → `eidWarning`
    static let warning           = Color.eidWarning
    /// alias → light warning tint
    static let warningBG         = Color(hex: "FEF3C7")

    // Sidebar (dark navy, always dark — matches Windows brand chrome)
    static let sidebarBackground = Color(hex: "0F172A")
    static let sidebarHover      = Color(hex: "1E293B")
    static let sidebarMutedText  = Color(hex: "94A3B8")

    /// Windows-аас өөр: page background — Windows EidSurface маяг (F9F9FC light).
    static let backgroundPrimary = Color.eidSurface

    /// Secondary surface (slightly darker for nested rows)
    static let backgroundSecondary = Color.dynamic(light: "F1F5F9", dark: "111827")

    /// Card background — alias for eidCardBackground
    static let surfaceCard = Color.eidCardBackground
    /// Card border — alias for eidCardStroke
    static let cardBorder  = Color.eidCardStroke

    /// Primary text
    static let textPrimary = Color.dynamic(light: "0F172A", dark: "F8FAFC")

    /// Secondary text — alias for eidMuted (Windows uses #6B7280)
    static let textSecondary = Color.eidMuted
}

// MARK: - Hex init
extension Color {
    init(hex: String) {
        let hex = hex.trimmingCharacters(in: CharacterSet.alphanumerics.inverted)
        var int: UInt64 = 0
        Scanner(string: hex).scanHexInt64(&int)
        let a, r, g, b: UInt64
        switch hex.count {
        case 6:
            (a, r, g, b) = (255, (int >> 16) & 0xFF, (int >> 8) & 0xFF, int & 0xFF)
        case 8:
            (a, r, g, b) = ((int >> 24) & 0xFF, (int >> 16) & 0xFF, (int >> 8) & 0xFF, int & 0xFF)
        default:
            (a, r, g, b) = (255, 0, 0, 0)
        }
        self.init(.sRGB,
                  red: Double(r) / 255,
                  green: Double(g) / 255,
                  blue: Double(b) / 255,
                  opacity: Double(a) / 255)
    }
}

// MARK: - Хоёр платформын dynamic өнгө
//
// Windows-ийн ThemeDictionaries-ийн порт нь macOS дээр `NSColor(name:)` dynamic
// provider дээр суурилдаг байв. Тэр API нь AppKit-ийнх тул iOS дээр байхгүй —
// энэ файлыг утасны апп ХУВААЛЦАЖ ашигладаг болсон учир өнгөний тодорхойлолтууд
// (дээрх бүх мөр) хэвээр үлдэж, зөвхөн ЭНЭ доод давхарга платформоор салаална.
extension Color {
    /// Гэрэл/харанхуйд өөр өөр hex. Утга нь харагдах агшинд шийдэгдэнэ — хэрэглэгч
    /// системийн төрхөө солиход апп дахин асаахгүйгээр дагана.
    static func dynamic(light: String, dark: String) -> Color {
        #if canImport(AppKit) && !targetEnvironment(macCatalyst)
        return Color(nsColor: NSColor(name: nil) { appearance in
            appearance.bestMatch(from: [.darkAqua, .vibrantDark]) != nil
                ? NSColor(hex: dark) : NSColor(hex: light)
        })
        #elseif canImport(UIKit)
        return Color(uiColor: UIColor { traits in
            traits.userInterfaceStyle == .dark ? UIColor(hex: dark) : UIColor(hex: light)
        })
        #else
        return Color(hex: light)
        #endif
    }
}

private func rgbComponents(_ hex: String) -> (CGFloat, CGFloat, CGFloat) {
    let cleaned = hex.trimmingCharacters(in: CharacterSet.alphanumerics.inverted)
    var int: UInt64 = 0
    Scanner(string: cleaned).scanHexInt64(&int)
    guard cleaned.count == 6 else { return (0, 0, 0) }
    return (CGFloat((int >> 16) & 0xFF) / 255,
            CGFloat((int >> 8) & 0xFF) / 255,
            CGFloat(int & 0xFF) / 255)
}

#if canImport(AppKit) && !targetEnvironment(macCatalyst)
extension NSColor {
    convenience init(hex: String) {
        let (r, g, b) = rgbComponents(hex)
        self.init(srgbRed: r, green: g, blue: b, alpha: 1)
    }
}
#endif

#if canImport(UIKit)
extension UIColor {
    convenience init(hex: String) {
        let (r, g, b) = rgbComponents(hex)
        self.init(red: r, green: g, blue: b, alpha: 1)
    }
}
#endif
