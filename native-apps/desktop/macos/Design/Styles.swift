import SwiftUI

// MARK: - Spacing tokens (Windows Spacing.xaml port)
enum Space {
    static let xs:  CGFloat = 4
    static let sm:  CGFloat = 8
    static let md:  CGFloat = 12
    static let lg:  CGFloat = 16
    static let xl:  CGFloat = 24
    static let xxl: CGFloat = 32
    static let xxxl: CGFloat = 48

    /// `PagePadding` Thickness 32,24,32,32
    static let pageTop:    CGFloat = 24
    static let pageHoriz:  CGFloat = 32
    static let pageBottom: CGFloat = 32

    /// `CardPadding` 24 uniform
    static let cardPadding: CGFloat = 24
}

// MARK: - Corner radii (Windows RadiusSm/Md/Lg)
enum Radius {
    static let sm: CGFloat = 4
    static let md: CGFloat = 8
    static let lg: CGFloat = 12
}

// MARK: - AppCard — Windows Controls/AppCard.xaml port
// Border + 1px stroke + 12 corner + 24 padding + EidCardBackground
struct AppCard<Content: View>: View {
    let content: () -> Content
    var padding: CGFloat = Space.cardPadding
    var cornerRadius: CGFloat = Radius.lg

    init(padding: CGFloat = Space.cardPadding,
         cornerRadius: CGFloat = Radius.lg,
         @ViewBuilder content: @escaping () -> Content) {
        self.padding = padding
        self.cornerRadius = cornerRadius
        self.content = content
    }

    var body: some View {
        content()
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(padding)
            .background(Color.eidCardBackground)
            .clipShape(RoundedRectangle(cornerRadius: cornerRadius, style: .continuous))
            .overlay(
                RoundedRectangle(cornerRadius: cornerRadius, style: .continuous)
                    .strokeBorder(Color.eidCardStroke, lineWidth: 1)
            )
    }
}

// MARK: - Card view modifier (legacy `.dashboardCard()` API)
struct DashboardCardStyle: ViewModifier {
    var padding: CGFloat = Space.cardPadding
    var cornerRadius: CGFloat = Radius.lg

    func body(content: Content) -> some View {
        content
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(padding)
            .background(Color.eidCardBackground)
            .clipShape(RoundedRectangle(cornerRadius: cornerRadius, style: .continuous))
            .overlay(
                RoundedRectangle(cornerRadius: cornerRadius, style: .continuous)
                    .strokeBorder(Color.eidCardStroke, lineWidth: 1)
            )
    }
}

extension View {
    /// Windows AppCard equivalent — apply to any container view.
    func appCard(padding: CGFloat = Space.cardPadding,
                 cornerRadius: CGFloat = Radius.lg) -> some View {
        modifier(DashboardCardStyle(padding: padding, cornerRadius: cornerRadius))
    }

    /// Legacy alias — same as `appCard()`.
    func dashboardCard(padding: CGFloat = Space.cardPadding,
                       cornerRadius: CGFloat = Radius.lg) -> some View {
        modifier(DashboardCardStyle(padding: padding, cornerRadius: cornerRadius))
    }
}

// MARK: - Accent button (Windows AccentButtonStyle)
struct PrimaryButtonStyle: ButtonStyle {
    var fullWidth: Bool = false
    var height: CGFloat = 38

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .font(.system(size: 14, weight: .semibold))
            .foregroundStyle(.white)
            .padding(.horizontal, 18)
            .frame(height: height)
            .frame(maxWidth: fullWidth ? .infinity : nil)
            .background(
                configuration.isPressed ? Color.eidAccentStrong : Color.eidAccent
            )
            .clipShape(RoundedRectangle(cornerRadius: Radius.md, style: .continuous))
            .opacity(configuration.isPressed ? 0.92 : 1)
    }
}

extension ButtonStyle where Self == PrimaryButtonStyle {
    static var primary: PrimaryButtonStyle { PrimaryButtonStyle() }
    static func primary(fullWidth: Bool, height: CGFloat = 38) -> PrimaryButtonStyle {
        PrimaryButtonStyle(fullWidth: fullWidth, height: height)
    }
}

// MARK: - Secondary button (outline)
struct SecondaryButtonStyle: ButtonStyle {
    var fullWidth: Bool = false
    var height: CGFloat = 38

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .font(.system(size: 14, weight: .semibold))
            .foregroundStyle(Color.textPrimary)
            .padding(.horizontal, 18)
            .frame(height: height)
            .frame(maxWidth: fullWidth ? .infinity : nil)
            .background(
                configuration.isPressed
                ? Color.backgroundSecondary
                : Color.eidCardBackground
            )
            .clipShape(RoundedRectangle(cornerRadius: Radius.md, style: .continuous))
            .overlay(
                RoundedRectangle(cornerRadius: Radius.md, style: .continuous)
                    .strokeBorder(Color.eidCardStroke, lineWidth: 1)
            )
    }
}

extension ButtonStyle where Self == SecondaryButtonStyle {
    static var secondary: SecondaryButtonStyle { SecondaryButtonStyle() }
    static func secondary(fullWidth: Bool, height: CGFloat = 38) -> SecondaryButtonStyle {
        SecondaryButtonStyle(fullWidth: fullWidth, height: height)
    }
}

// MARK: - Status pill (Windows DashboardPage status pill)
// Solid bg + white text + 12 corner + 10,4 padding
struct StatusPill: View {
    enum Variant { case ok, warn, bad, neutral, accent }
    let text: String
    let variant: Variant

    init(_ text: String, variant: Variant = .ok) {
        self.text = text
        self.variant = variant
    }

    var body: some View {
        Text(text)
            .font(.system(size: 12, weight: .semibold))
            .foregroundStyle(.white)
            .padding(.horizontal, 10)
            .padding(.vertical, 4)
            .background(background)
            .clipShape(RoundedRectangle(cornerRadius: 12, style: .continuous))
    }

    private var background: Color {
        switch variant {
        case .ok:      return Color.eidSuccess
        case .warn:    return Color.eidWarning
        case .bad:     return Color.eidDanger
        case .accent:  return Color.eidAccent
        case .neutral: return Color.eidMuted
        }
    }
}

// MARK: - Accent badge (Windows TokenDetail state/backend badge)
// Background=EidAccentSubtle, foreground=EidAccent
struct AccentBadge: View {
    let text: String
    var fontSize: CGFloat = 11
    var horizontalPadding: CGFloat = 8
    var verticalPadding: CGFloat = 3

    var body: some View {
        Text(text)
            .font(.system(size: fontSize, weight: .semibold))
            .foregroundStyle(Color.eidAccent)
            .padding(.horizontal, horizontalPadding)
            .padding(.vertical, verticalPadding)
            .background(Color.eidAccentSubtle)
            .clipShape(RoundedRectangle(cornerRadius: 10, style: .continuous))
    }
}

// MARK: - CodeDigitBox (Windows LoginPage/OrgPage verification-code box)
// 56x64, 2px border, 12 corner, EidAccentSubtle bg, EidAccent text
struct CodeDigitBox: View {
    let digit: Character
    var size: CGSize = CGSize(width: 56, height: 64)

    var body: some View {
        Text(String(digit))
            .font(.eidCodeDigit)
            .foregroundStyle(Color.eidAccent)
            .frame(width: size.width, height: size.height)
            .background(Color.eidAccentSubtle)
            .overlay(
                RoundedRectangle(cornerRadius: 12, style: .continuous)
                    .strokeBorder(Color.eidAccent, lineWidth: 2)
            )
            .clipShape(RoundedRectangle(cornerRadius: 12, style: .continuous))
    }
}

// MARK: - Verification code row helper
struct VerificationCodeRow: View {
    let code: String
    var size: CGSize = CGSize(width: 56, height: 64)

    var body: some View {
        HStack(spacing: 12) {
            ForEach(Array(code.enumerated()), id: \.offset) { _, digit in
                CodeDigitBox(digit: digit, size: size)
            }
        }
    }
}

// MARK: - Inline banner (Windows error banner pattern)
struct InlineBanner: View {
    enum Variant { case error, success, info }
    let text: String
    let variant: Variant
    var icon: String?

    var body: some View {
        HStack(alignment: .top, spacing: 10) {
            Image(systemName: icon ?? defaultIcon)
                .font(.system(size: 14, weight: .semibold))
                .foregroundStyle(iconColor)
            Text(text)
                .font(.eidBody)
                .foregroundStyle(textColor)
                .fixedSize(horizontal: false, vertical: true)
            Spacer(minLength: 0)
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 10)
        .background(background)
        .overlay(
            RoundedRectangle(cornerRadius: Radius.md, style: .continuous)
                .strokeBorder(borderColor, lineWidth: 1)
        )
        .clipShape(RoundedRectangle(cornerRadius: Radius.md, style: .continuous))
    }

    private var defaultIcon: String {
        switch variant {
        case .error:   return "exclamationmark.triangle.fill"
        case .success: return "checkmark.circle.fill"
        case .info:    return "info.circle.fill"
        }
    }

    private var background: Color {
        switch variant {
        case .error:   return Color.bannerErrorBG
        case .success: return Color.bannerSuccessBG
        case .info:    return Color.eidAccentSubtle
        }
    }

    private var borderColor: Color {
        switch variant {
        case .error:   return Color.bannerErrorBorder
        case .success: return Color.eidSuccess.opacity(0.35)
        case .info:    return Color.eidAccent.opacity(0.35)
        }
    }

    private var iconColor: Color {
        switch variant {
        case .error:   return Color.bannerErrorIcon
        case .success: return Color.bannerSuccessIcon
        case .info:    return Color.eidAccent
        }
    }

    private var textColor: Color {
        switch variant {
        case .error:   return Color.bannerErrorText
        case .success: return Color.textPrimary
        case .info:    return Color.textPrimary
        }
    }
}

// MARK: - StatusBadge (legacy alias — color-tinted pill with leading dot)
//
// Older callers used a dot+text+tinted-bg badge. Map to a Capsule with the
// requested foreground and a 10%-opacity background for visual continuity.
struct StatusBadge: View {
    let text: String
    let color: Color

    init(_ text: String, color: Color) {
        self.text = text
        self.color = color
    }

    var body: some View {
        HStack(spacing: 4) {
            Circle().fill(color).frame(width: 5, height: 5)
            Text(text)
                .font(.system(size: 11, weight: .semibold))
        }
        .foregroundStyle(color)
        .padding(.horizontal, 8)
        .padding(.vertical, 3)
        .background(color.opacity(0.12))
        .clipShape(Capsule())
    }
}

// MARK: - Status dot (Windows DashboardPage ellipse + label)
struct StatusDot: View {
    let color: Color
    var size: CGFloat = 8

    var body: some View {
        Circle().fill(color).frame(width: size, height: size)
    }
}
