import AppKit

/*
 * The eID platform's design system, in AppKit.
 *
 * Ported from gerege-line/eid/eid-platform-mn — desktop/macos-app/Design — so
 * the two desktop clients read as one family. The values are theirs exactly:
 * the brand ramp, the theme-aware brushes, the type scale. What changed is the
 * layer they are expressed in. Their app is SwiftUI and this one is AppKit, and
 * a SwiftUI view cannot be dropped into an NSViewController — so the palette is
 * restated as NSColor and the type scale as NSFont, which is where their own
 * Colors.swift already bottoms out anyway: its dynamic colours are built on
 * NSColor(name:) and NSColor(hex:), and those two helpers arrived unchanged.
 *
 * Their file carries a note worth keeping: the ramp is sourced from that
 * platform's web globals.css and kept in lockstep with its Windows app. This
 * is now a third place holding the same numbers, so a change to the ramp is a
 * change in three repositories rather than one. Nothing here should drift on
 * its own.
 */

// MARK: - Brand ramp (Windows Colors.xaml Brand50-950)

enum Brand {
    static let b50  = NSColor(eidHex: "EEF4FF")
    static let b100 = NSColor(eidHex: "D8E6FF")
    static let b200 = NSColor(eidHex: "B3CCFF")
    static let b300 = NSColor(eidHex: "84A8FF")
    static let b400 = NSColor(eidHex: "5B8AFF")
    static let b500 = NSColor(eidHex: "3A6DFF") // primary brand
    static let b600 = NSColor(eidHex: "2A5BD7")
    static let b700 = NSColor(eidHex: "1D47B3")
    static let b800 = NSColor(eidHex: "14358A")
    static let b900 = NSColor(eidHex: "0A2466")
    static let b950 = NSColor(eidHex: "001F5E")
}

// MARK: - Theme-aware brushes (their ThemeDictionaries port)

enum EID {
    /// `EidAccentBrush` — Brand500 light / Brand300 dark
    static let accent = NSColor.eidDynamic(light: Brand.b500, dark: Brand.b300)
    /// `EidAccentStrongBrush` — Brand700 light / Brand200 dark
    static let accentStrong = NSColor.eidDynamic(light: Brand.b700, dark: Brand.b200)
    /// `EidAccentSubtleBrush` — Brand100 light / Brand900 dark
    static let accentSubtle = NSColor.eidDynamic(light: Brand.b100, dark: Brand.b900)
    /// `EidAccentMutedBrush` — Brand50 light / Brand950 dark
    static let accentMuted = NSColor.eidDynamic(light: Brand.b50, dark: Brand.b950)

    static let success = NSColor.eidDynamic(light: NSColor(eidHex: "107C10"), dark: NSColor(eidHex: "6FCF6F"))
    static let warning = NSColor.eidDynamic(light: NSColor(eidHex: "9D5D00"), dark: NSColor(eidHex: "E8B66A"))
    static let danger = NSColor.eidDynamic(light: NSColor(eidHex: "BA1A1A"), dark: NSColor(eidHex: "FFB4AB"))

    /// `EidCardBackground` — White / #1F1F23
    static let cardBackground = NSColor.eidDynamic(light: .white, dark: NSColor(eidHex: "1F1F23"))
    /// `EidCardStroke` — #E5E7EB / #34343A
    static let cardStroke = NSColor.eidDynamic(light: NSColor(eidHex: "E5E7EB"), dark: NSColor(eidHex: "34343A"))
    /// `EidSurfaceBrush` — page background
    static let surface = NSColor.eidDynamic(light: NSColor(eidHex: "F9F9FC"), dark: NSColor(eidHex: "0B0E12"))
    /// `EidMutedForeground`
    static let muted = NSColor.eidDynamic(light: NSColor(eidHex: "6B7280"), dark: NSColor(eidHex: "9CA3AF"))
    static let textPrimary = NSColor.eidDynamic(light: NSColor(eidHex: "0F172A"), dark: NSColor(eidHex: "F8FAFC"))

    /// Inline banner tints. Fixed rather than theme-aware in their app too.
    static let bannerErrorBG = NSColor(eidHex: "FEE2E2")
    static let bannerErrorBorder = NSColor(eidHex: "FCA5A5")
    static let bannerErrorText = NSColor(eidHex: "7F1D1D")

    /// The left panel's diagonal gradient, top-leading to bottom-trailing.
    static let heroGradient = [
        NSColor(eidHex: "091540"), NSColor(eidHex: "0E2060"), NSColor(eidHex: "1A3A8F"),
    ]
    /// The shell's own chrome — rail, ribbon, footer. Always dark in their app
    /// too (their Colors.swift keeps these outside the theme dictionaries),
    /// because it frames a work area rather than sitting on the card surface.
    static let chrome = NSColor(eidHex: "0F172A")
    static let chromeHover = NSColor(eidHex: "1E293B")
    static let chromeMuted = NSColor(eidHex: "94A3B8")
    /// The accent as it reads on that dark chrome: the light end of the ramp,
    /// because Brand500 on #0F172A is too close to the ground to see.
    static let chromeAccent = Brand.b300

    /// Copy on that panel, which is always dark and so does not follow the theme.
    static let heroSubtitle = NSColor(eidHex: "93C5FD")
    static let heroBody = NSColor(eidHex: "7CAEE8")
    static let heroBokehSecondary = NSColor(eidHex: "818CF8")
}

// MARK: - Radius scale (their Styles.swift)

enum EIDRadius {
    static let sm: CGFloat = 4
    static let md: CGFloat = 8
    static let lg: CGFloat = 12
    /// The login card's own corner, which their LoginView sets directly.
    static let card: CGFloat = 18
}

// MARK: - Type scale (their Typography.swift port)

enum EIDFont {
    /// `HeroTitleTextBlock` — 28pt SemiBold
    static let heroTitle = NSFont.systemFont(ofSize: 28, weight: .semibold)
    /// `PageTitleTextBlock` — 20pt SemiBold
    static let pageTitle = NSFont.systemFont(ofSize: 20, weight: .semibold)
    /// `SectionTitleTextBlock` — 17pt SemiBold
    static let sectionTitle = NSFont.systemFont(ofSize: 17, weight: .semibold)
    /// `SubtleSubtitleTextBlock` — body copy
    static let body = NSFont.systemFont(ofSize: 14)
    /// `LabelTextBlock` — caption
    static let label = NSFont.systemFont(ofSize: 12)
    /// The card's own heading, 24pt Bold in their LoginView.
    static let cardTitle = NSFont.systemFont(ofSize: 24, weight: .bold)
    /// The identifier field: 18pt Medium monospaced, so digits line up as typed.
    static let identifier = NSFont.monospacedSystemFont(ofSize: 18, weight: .medium)
    /// Verification-code digits.
    static let codeDigit = NSFont.monospacedSystemFont(ofSize: 28, weight: .bold)
    static let tab = NSFont.systemFont(ofSize: 13, weight: .semibold)
    static let primaryButton = NSFont.systemFont(ofSize: 16, weight: .semibold)
}

// MARK: - Helpers, arriving unchanged from their Colors.swift

extension NSColor {
    /// Resolves light or dark at draw time, so a window following the system
    /// theme repaints without anything being told to.
    static func eidDynamic(light: NSColor, dark: NSColor) -> NSColor {
        NSColor(name: nil) { appearance in
            appearance.bestMatch(from: [.darkAqua, .vibrantDark]) != nil ? dark : light
        }
    }

    convenience init(eidHex hex: String) {
        let cleaned = hex.trimmingCharacters(in: CharacterSet.alphanumerics.inverted)
        var value: UInt64 = 0
        Scanner(string: cleaned).scanHexInt64(&value)
        let r, g, b: UInt64
        switch cleaned.count {
        case 6: (r, g, b) = ((value >> 16) & 0xFF, (value >> 8) & 0xFF, value & 0xFF)
        default: (r, g, b) = (0, 0, 0)
        }
        self.init(srgbRed: CGFloat(r) / 255, green: CGFloat(g) / 255, blue: CGFloat(b) / 255, alpha: 1)
    }

    /// A CGColor resolved against an appearance.
    ///
    /// A dynamic NSColor's cgColor is resolved once, against whatever appearance
    /// happened to be current — so a layer painted with it keeps the theme it
    /// was born in and never follows a switch. Every layer colour below goes
    /// through here, from a view that knows its own appearance.
    func eidCGColor(for view: NSView) -> CGColor {
        var resolved = cgColor
        view.effectiveAppearance.performAsCurrentDrawingAppearance {
            resolved = self.cgColor
        }
        return resolved
    }
}

// MARK: - Brand mark

/// The brand mark, loaded from logo.jpg beside the executable.
///
/// build.sh emits a bare binary rather than an .app, so there is no bundle to
/// carry a resource: logo.jpg travels next to it and is read from there. Nil
/// when it is not there, which every caller treats as "carry on without it"
/// rather than as a failure — a missing logo is not worth a crash.
func brandMark() -> NSImage? {
    let beside = Bundle.main.bundleURL.appendingPathComponent("logo.jpg")
    return NSImage(contentsOf: beside) ?? NSImage(contentsOfFile: "logo.jpg") ?? NSImage(contentsOfFile: "brand.png")
}

// MARK: - AppCard

/// Their `AppCard` — border, 1px stroke, 12 corner, 24 padding, card
/// background. It is the unit every settings screen is built from over there,
/// so it is a view here rather than a set of properties copied per screen.
final class EIDCard: NSView {
    private let content: NSView

    /// - Parameter padding: their Space.cardPadding, which every card uses
    ///   unless it is holding something that wants to reach the edge.
    init(content: NSView, padding: CGFloat = 24) {
        self.content = content
        super.init(frame: .zero)
        wantsLayer = true
        layer?.cornerRadius = EIDRadius.lg
        layer?.cornerCurve = .continuous
        layer?.borderWidth = 1

        content.translatesAutoresizingMaskIntoConstraints = false
        addSubview(content)
        NSLayoutConstraint.activate([
            content.leadingAnchor.constraint(equalTo: leadingAnchor, constant: padding),
            content.trailingAnchor.constraint(equalTo: trailingAnchor, constant: -padding),
            content.topAnchor.constraint(equalTo: topAnchor, constant: padding),
            content.bottomAnchor.constraint(equalTo: bottomAnchor, constant: -padding),
        ])
        applyColors()
    }

    required init?(coder: NSCoder) { fatalError() }

    override func viewDidChangeEffectiveAppearance() {
        super.viewDidChangeEffectiveAppearance()
        applyColors()
    }

    private func applyColors() {
        layer?.backgroundColor = EID.cardBackground.eidCGColor(for: self)
        layer?.borderColor = EID.cardStroke.eidCGColor(for: self)
    }
}

/// A view that paints their settings surface and keeps painting it after a
/// theme switch. Layer colours do not follow one on their own.
final class ThemeAwareSurface: NSView {
    override init(frame frameRect: NSRect) {
        super.init(frame: frameRect)
        wantsLayer = true
        applyColors()
    }

    required init?(coder: NSCoder) { fatalError() }

    override func viewDidChangeEffectiveAppearance() {
        super.viewDidChangeEffectiveAppearance()
        applyColors()
    }

    private func applyColors() { layer?.backgroundColor = EID.surface.eidCGColor(for: self) }
}
