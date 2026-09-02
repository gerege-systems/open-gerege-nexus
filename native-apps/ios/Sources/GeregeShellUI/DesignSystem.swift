#if os(iOS)
import SwiftUI
import UIKit

/*
 * The wallet's design system, for the mobile clients.
 *
 * Ported from gerege-line/gerege-core/wallet-gerege-mn — mobile/ios/
 * GeregeWallet/Shared/Theme.swift. Both apps are SwiftUI, so unlike the
 * desktop port this one is a straight copy: the tokens are theirs, expressed
 * the way they express them.
 *
 * Worth knowing that this is a *different* palette from the one the desktop
 * clients wear. Those took the eID platform's ramp (#3A6DFF, light-first);
 * this is the wallet's (#3178EC, dark-first). Two product lines, two
 * identities — deliberate, but it does mean "the brand blue" is an ambiguous
 * phrase across this repository, and a change to one is not a change to the
 * other.
 *
 * Their file organises by Refactoring UI's 60/30/10: surface dominates, brand
 * identifies, accent signals. The comments explaining each choice are theirs
 * and are kept, because a token whose reason travels with it is one that
 * survives the next redesign.
 */

enum WalletTheme {

    // MARK: 60 — Surface (dominant neutral)

    enum Surface {
        /// Page background: warm off-white with a faint blue undertone in
        /// light mode; deep ink with a brand-blue undertone in dark.
        static let page = adaptive(light: (0.96, 0.97, 1.00), dark: (0.04, 0.05, 0.09))
        /// Raised card — pure white over `page` in light, lifted charcoal in
        /// dark. The container colour for action cards and activity rows.
        static let card = adaptive(light: (1.00, 1.00, 1.00), dark: (0.10, 0.11, 0.18))
        /// A subtle wash for grouped content that steps down from `card`
        /// without going all the way to `page`.
        static let subtle = adaptive(light: (0.94, 0.96, 1.00), dark: (0.13, 0.15, 0.22))
        /// Hairline divider.
        static let divider = Color(uiColor: UIColor.separator).opacity(0.6)
        /// Input field outline.
        static let border = adaptive(light: (0xE2 / 255.0, 0xE5 / 255.0, 0xEC / 255.0),
                                     dark: (0.22, 0.24, 0.30))
    }

    // MARK: 30 — Brand (identity)

    enum Brand {
        /// Refined brand blue (#3178EC), replacing a pure #0000FF that
        /// rendered as muddy purple on dark translucent material and competed
        /// with everything around it.
        static let primary = Color(red: 0x31 / 255.0, green: 0x78 / 255.0, blue: 0xEC / 255.0)
        /// Deeper variant — pressed state, bottom of the hero gradient. About
        /// 12% darker: noticeable without being a jump.
        static let lo = Color(red: 0x25 / 255.0, green: 0x63 / 255.0, blue: 0xD1 / 255.0)
        /// Text and control tint. The same blue in light mode; lifted in dark
        /// so links and the selected tab read against the material.
        static let hi = adaptive(light: (0x31 / 255.0, 0x78 / 255.0, 0xEC / 255.0),
                                 dark: (0x80 / 255.0, 0xAE / 255.0, 0xFF / 255.0))
        /// Foreground on brand surfaces — always white, because the hero is
        /// brand blue in both modes.
        static let onBrand = Color.white
        /// A subtle wash for info banners and validation pills.
        static let tint = Color(uiColor: UIColor { trait in
            trait.userInterfaceStyle == .dark
                ? UIColor(red: 0x80 / 255.0, green: 0xAE / 255.0, blue: 0xFF / 255.0, alpha: 0.18)
                : UIColor(red: 0x31 / 255.0, green: 0x78 / 255.0, blue: 0xEC / 255.0, alpha: 0.10)
        })
        /// Hero gradient — lifted brand to deep brand, both inside the family
        /// so it shades without breaking the colour story.
        static let gradient = LinearGradient(
            colors: [
                Color(red: 0x4A / 255.0, green: 0x90 / 255.0, blue: 0xF0 / 255.0),
                Color(red: 0x25 / 255.0, green: 0x63 / 255.0, blue: 0xD1 / 255.0),
            ],
            startPoint: .topLeading,
            endPoint: .bottomTrailing
        )
    }

    // MARK: 10 — Accent (success)

    enum Accent {
        /// Emerald — confirmations and anything that means "went through".
        static let primary = adaptive(light: (0x10 / 255.0, 0xB9 / 255.0, 0x81 / 255.0),
                                      dark: (0x34 / 255.0, 0xD3 / 255.0, 0x99 / 255.0))
    }

    enum Danger {
        /// Coral red — a cooler red that sits beside the emerald without
        /// clashing with it.
        static let primary = adaptive(light: (0xEF / 255.0, 0x44 / 255.0, 0x44 / 255.0),
                                      dark: (0xF8 / 255.0, 0x71 / 255.0, 0x71 / 255.0))
    }

    // MARK: Foreground ramp

    enum Text {
        static let primary = adaptive(light: (0x0E / 255.0, 0x15 / 255.0, 0x25 / 255.0),
                                      dark: (0xEA / 255.0, 0xEC / 255.0, 0xEF / 255.0))
        static let secondary = adaptive(light: (0x4A / 255.0, 0x54 / 255.0, 0x68 / 255.0),
                                        dark: (0xB7 / 255.0, 0xBD / 255.0, 0xC6 / 255.0))
        static let tertiary = adaptive(light: (0x88 / 255.0, 0x92 / 255.0, 0xA6 / 255.0),
                                       dark: (0x70 / 255.0, 0x7A / 255.0, 0x8A / 255.0))
    }

    /// Their `adaptiveColor`: one token, resolved per appearance at draw time.
    static func adaptive(light: (Double, Double, Double), dark: (Double, Double, Double)) -> Color {
        Color(uiColor: UIColor { trait in
            let component = trait.userInterfaceStyle == .dark ? dark : light
            return UIColor(red: component.0, green: component.1, blue: component.2, alpha: 1)
        })
    }
}

// MARK: - Scale system
//
// Their note: pick from a menu, don't invent. A 4pt base with roughly 25%
// steps, shared with the Android app so a measurement means the same thing on
// both.

extension WalletTheme {
    enum Space {
        static let xxs: CGFloat = 2
        static let xs: CGFloat = 4
        static let sm: CGFloat = 8
        static let md: CGFloat = 12
        static let lg: CGFloat = 16
        static let xl: CGFloat = 24
        static let xxl: CGFloat = 32
        static let xxxl: CGFloat = 48
        static let huge: CGFloat = 64
    }

    enum Radius {
        static let sm: CGFloat = 8
        static let md: CGFloat = 12
        static let lg: CGFloat = 16
        static let xl: CGFloat = 20
        static let xxl: CGFloat = 24
    }
}
#endif
