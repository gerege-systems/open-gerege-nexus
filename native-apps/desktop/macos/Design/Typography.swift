import SwiftUI

// MARK: - Windows Typography.xaml port
//
// Windows builds on WinUI's TitleLarge / Title / Subtitle / Body / Caption.
// SF Pro `largeTitle / title2 / title3 / body / caption` map roughly:
//   TitleLargeTextBlockStyle → 28pt SemiBold  (HeroTitle)
//   TitleTextBlockStyle      → 20pt SemiBold  (PageTitle)
//   SubtitleTextBlockStyle   → 17pt SemiBold  (SectionTitle)
//   BodyTextBlockStyle       → 14pt Regular   (Subtitle subtle)
//   CaptionTextBlockStyle    → 12pt Regular   (Label)
extension Font {
    /// `HeroTitleTextBlock` — 28pt SemiBold (LineHeight 44 — applied via lineSpacing in modifier)
    static let eidHeroTitle    = Font.system(size: 28, weight: .semibold)
    /// `PageTitleTextBlock` — 20pt SemiBold
    static let eidPageTitle    = Font.system(size: 20, weight: .semibold)
    /// `SectionTitleTextBlock` — 17pt SemiBold
    static let eidSectionTitle = Font.system(size: 17, weight: .semibold)
    /// `SubtleSubtitleTextBlock` body / general copy
    static let eidBody         = Font.system(size: 14)
    /// `LabelTextBlock` — small caption (12pt)
    static let eidLabel        = Font.system(size: 12)
    /// Stat value (DashboardPage StatValueText)
    static let eidStatValue    = Font.system(size: 28, weight: .semibold)
    /// Stat label (DashboardPage StatLabelText)
    static let eidStatLabel    = Font.system(size: 12, weight: .semibold)
    /// Verification-code digit (CodeDigitText)
    static let eidCodeDigit    = Font.system(size: 28, weight: .bold, design: .monospaced)
    /// Mono code (Cascadia Code, Consolas)
    static let eidMono         = Font.system(size: 13, design: .monospaced)
    /// Small mono (serial, hex)
    static let eidMonoSmall    = Font.system(size: 11, design: .monospaced)

    // MARK: - Legacy aliases
    static let smartTitle   = Font.eidPageTitle
    static let smartHeading = Font.eidSectionTitle
    static let smartSubhead = Font.system(size: 14, weight: .medium)
    static let smartBody    = Font.eidBody
    static let smartCaption = Font.eidLabel
    static let smartCode    = Font.eidCodeDigit
}

// MARK: - Typography modifiers — apply Windows line-height/color rules

extension View {
    /// `HeroTitleTextBlock` Windows-тэй ижил
    func heroTitleStyle() -> some View {
        self.font(.eidHeroTitle)
            .foregroundStyle(Color.textPrimary)
            .lineSpacing(6)
    }

    /// `PageTitleTextBlock` — 20pt SemiBold + 8-bottom
    func pageTitleStyle() -> some View {
        self.font(.eidPageTitle)
            .foregroundStyle(Color.textPrimary)
    }

    /// `SectionTitleTextBlock` — 17pt SemiBold
    func sectionTitleStyle() -> some View {
        self.font(.eidSectionTitle)
            .foregroundStyle(Color.textPrimary)
    }

    /// `SubtleSubtitleTextBlock` — body + muted + wrap
    func subtleSubtitleStyle() -> some View {
        self.font(.eidBody)
            .foregroundStyle(Color.eidMuted)
            .lineSpacing(4)
            .fixedSize(horizontal: false, vertical: true)
    }

    /// `LabelTextBlock` — caption + muted + wrap
    func labelStyle() -> some View {
        self.font(.eidLabel)
            .foregroundStyle(Color.eidMuted)
            .fixedSize(horizontal: false, vertical: true)
    }
}
