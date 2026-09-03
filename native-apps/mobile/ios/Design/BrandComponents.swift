import SwiftUI

// MARK: - Утасны дүрсийн сан
//
// Gerege Wallet-ийн `BrandComponents` бүлгийн порт: дэлгэцийн суурь, карт,
// оролтын сав, баталгаажуулалтын товч, мэдээллийн самбар, аюулгүйн хөл,
// ачаалалтай CTA. Геометр (52pt оролт, 56pt товч, 14pt радиус, 26pt
// брэндийн дөрвөлжин) нь эх дизайнаас яг тэр чигээрээ.
//
// Ширээний `Styles.swift` дэх AppCard / .primary / StatusPill / InlineBanner /
// VerificationCodeRow нь macOS апптай ХУВААЛЦСАН тул тэднийг өнгөөр нь
// сольж болохгүй — утасны дүйцлүүд нь энд, `Theme.` токен дээр амьдарна.
// Ширээнийхийг форк хийгээгүй: тэдгээр нь ширээн дээрээ хэвээр ажиллана.

// MARK: - Дэлгэцийн суурь

/// Бүх дэлгэцийн гадна давхарга — токены `bg`-ээр дүүргэсэн зотон.
struct BrandScreen<Content: View>: View {
    @ViewBuilder var content: () -> Content
    var body: some View {
        ZStack {
            Theme.bg.ignoresSafeArea()
            content()
        }
    }
}

/// Цагаан (харанхуйд өргөгдсөн) бүлэг карт — 1pt хүрээ, зөөлөн сүүдэр.
struct BrandCard<Content: View>: View {
    var padding: CGFloat = Theme.Space.lg
    var spacing: CGFloat = Theme.Space.md
    @ViewBuilder var content: () -> Content

    init(padding: CGFloat = Theme.Space.lg,
         spacing: CGFloat = Theme.Space.md,
         @ViewBuilder content: @escaping () -> Content) {
        self.padding = padding
        self.spacing = spacing
        self.content = content
    }

    var body: some View {
        VStack(alignment: .leading, spacing: spacing, content: content)
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(padding)
            .background(Theme.surface1)
            .clipShape(RoundedRectangle(cornerRadius: Theme.Radius.lg, style: .continuous))
            .overlay(
                RoundedRectangle(cornerRadius: Theme.Radius.lg, style: .continuous)
                    .strokeBorder(Theme.border, lineWidth: 1)
            )
            .elevation(1)
    }
}

// MARK: - Бичвэрийн жижиг дүрсүүд

/// Том үсгээр, зай нэмсэн хэсгийн шошго.
struct BrandSectionLabel: View {
    let text: String
    var body: some View {
        Text(text.uppercased())
            .font(Theme.TypeScale.sectionLabel)
            .tracking(0.9)
            .foregroundStyle(Theme.fg3)
    }
}

/// Шошго + утга. Гар дээрх талбарын үндсэн мөр.
struct MobileField: View {
    let label: String
    let value: String
    var mono = false

    var body: some View {
        VStack(alignment: .leading, spacing: Theme.Space.xs) {
            BrandSectionLabel(text: label)
            Text(value.isEmpty ? "—" : value)
                .font(mono ? Theme.TypeScale.mono : Theme.TypeScale.bodyBold)
                .foregroundStyle(Theme.fg1)
                .textSelection(.enabled)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }
}

// MARK: - Төлөвийн жижиг капсул

struct BrandPill: View {
    enum Tone { case ok, warn, brand, neutral }
    let text: String
    var tone: Tone = .ok

    private var color: Color {
        switch tone {
        case .ok:      return Theme.credit
        case .warn:    return Theme.accent
        case .brand:   return Theme.Brand.primary
        case .neutral: return Theme.fg3
        }
    }
    private var soft: Color {
        switch tone {
        case .ok:      return Theme.creditSoft
        case .warn:    return Theme.accentSoft
        case .brand:   return Theme.Brand.soft
        case .neutral: return Theme.surface3
        }
    }

    var body: some View {
        Text(text)
            .font(Theme.TypeScale.caption2Bold)
            .tracking(0.4)
            .foregroundStyle(color)
            .padding(.horizontal, 9)
            .padding(.vertical, 4)
            .background(soft, in: Capsule())
    }
}

// MARK: - Оролт

struct BrandValidationBadge: View {
    let text: String
    let valid: Bool
    var body: some View {
        HStack(spacing: Theme.Space.xs) {
            Image(systemName: valid ? "checkmark" : "xmark")
                .font(.system(size: 10, weight: .bold))
            Text(text).font(Theme.TypeScale.caption2Bold)
        }
        .foregroundStyle(valid ? Theme.credit : Theme.debit)
        .padding(.horizontal, 9)
        .padding(.vertical, 4)
        .background(valid ? Theme.creditSoft : Theme.debitSoft, in: Capsule())
    }
}

/// Оролтын сав — 52pt өндөр, дүрс + талбар + баталгаажуулалтын капсул.
/// Фокус дээр хүрээ нь брэнд рүү, 1.5pt болж зузаарна.
struct BrandInputCard<Content: View>: View {
    let leadingIcon: String?
    let validation: BrandValidationBadge?
    let isFocused: Bool
    @ViewBuilder var content: () -> Content

    init(leadingIcon: String? = nil,
         validation: BrandValidationBadge? = nil,
         isFocused: Bool = false,
         @ViewBuilder content: @escaping () -> Content) {
        self.leadingIcon = leadingIcon
        self.validation = validation
        self.isFocused = isFocused
        self.content = content
    }

    var body: some View {
        HStack(spacing: 10) {
            if let leadingIcon {
                Image(systemName: leadingIcon)
                    .font(.system(size: 16))
                    .foregroundStyle(Theme.fg3)
                    .frame(width: 20)
            }
            content()
                .frame(maxWidth: .infinity, alignment: .leading)
            validation
        }
        .padding(.horizontal, 14)
        .frame(minHeight: 52)
        .background(Theme.surface1)
        .clipShape(RoundedRectangle(cornerRadius: 14, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: 14, style: .continuous)
                .strokeBorder(isFocused ? Theme.Brand.primary : Theme.border,
                              lineWidth: isFocused ? 1.5 : 1)
        )
        .animation(.easeOut(duration: 0.15), value: isFocused)
    }
}

// MARK: - Самбарууд

/// Мэдээллийн самбар — 26pt брэндийн дөрвөлжин дотор цагаан дүрс.
struct BrandInfoBanner: View {
    var icon: String = "bell.fill"
    let text: String

    var body: some View {
        HStack(alignment: .top, spacing: 10) {
            Image(systemName: icon)
                .font(.system(size: 12, weight: .semibold))
                .foregroundStyle(Color.white)
                .frame(width: 26, height: 26)
                .background(Theme.Brand.primary,
                            in: RoundedRectangle(cornerRadius: 8, style: .continuous))
            Text(text)
                .font(Theme.TypeScale.caption)
                .lineSpacing(4)
                .foregroundStyle(Theme.fg2)
                .frame(maxWidth: .infinity, alignment: .leading)
        }
        .padding(Theme.Space.md)
        .background(Theme.Brand.soft)
        .clipShape(RoundedRectangle(cornerRadius: Theme.Radius.md, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: Theme.Radius.md, style: .continuous)
                .strokeBorder(Theme.Brand.line, lineWidth: 1)
        )
    }
}

/// Алдаа/амжилтын мөр — ширээний `InlineBanner`-ийн утасны дүйцэл.
struct BrandBanner: View {
    enum Tone { case error, success }
    let text: String
    var tone: Tone = .error

    private var tint: Color { tone == .error ? Theme.debit : Theme.credit }
    private var soft: Color { tone == .error ? Theme.debitSoft : Theme.creditSoft }

    var body: some View {
        HStack(alignment: .top, spacing: Theme.Space.sm) {
            Image(systemName: tone == .error ? "exclamationmark.triangle.fill" : "checkmark.circle.fill")
                .font(.system(size: 13))
                .foregroundStyle(tint)
            Text(text)
                .font(Theme.TypeScale.caption)
                .lineSpacing(3)
                .foregroundStyle(Theme.fg2)
                .frame(maxWidth: .infinity, alignment: .leading)
        }
        .padding(Theme.Space.md)
        .background(soft)
        .clipShape(RoundedRectangle(cornerRadius: Theme.Radius.md, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: Theme.Radius.md, style: .continuous)
                .strokeBorder(tint.opacity(0.35), lineWidth: 1)
        )
    }
}

/// Дэлгэцийн доод аюулгүйн мөр.
struct BrandSecurityFooter: View {
    let text: String
    var body: some View {
        HStack(spacing: 6) {
            Image(systemName: "lock.shield")
                .font(.system(size: 11))
            Text(text).font(Theme.TypeScale.caption2)
        }
        .foregroundStyle(Theme.fg3)
    }
}

// MARK: - Баталгаажуулах код
//
// Хоёр аппын хооронд ТУЛГАХ ёстой тоо тул тоо бүр өөрийн нүдэнд, tabular.

struct BrandCodeRow: View {
    let code: String
    var body: some View {
        HStack(spacing: Theme.Space.sm) {
            ForEach(Array(code.enumerated()), id: \.offset) { _, digit in
                Text(String(digit))
                    .font(Theme.TypeScale.codeHero)
                    .foregroundStyle(Theme.Brand.primary)
                    .frame(width: 40, height: 52)
                    .background(Theme.Brand.soft)
                    .clipShape(RoundedRectangle(cornerRadius: Theme.Radius.md, style: .continuous))
                    .overlay(
                        RoundedRectangle(cornerRadius: Theme.Radius.md, style: .continuous)
                            .strokeBorder(Theme.Brand.line, lineWidth: 1)
                    )
            }
        }
    }
}

// MARK: - Товчнууд

/// Үндсэн CTA — 56pt өндөр брэндийн блок. Ачаалж байх үед шошгыг спиннер
/// СОЛИНО (нуухгүй): сүлжээний эргэлтийн дунд хоёр дахь даралт нь давхар
/// session үүсгэхгүй.
struct LoadingPrimaryButton: View {
    let title: String
    var isLoading: Bool = false
    var isEnabled: Bool = true
    var leadingSymbol: String? = "arrow.right"
    let action: () -> Void

    var body: some View {
        Button(action: { if !isLoading && isEnabled { action() } }) {
            ZStack {
                HStack(spacing: Theme.Space.sm) {
                    if let leadingSymbol {
                        Image(systemName: leadingSymbol)
                            .font(.system(size: 16, weight: .semibold))
                    }
                    Text(title).font(Theme.TypeScale.headline)
                }
                .opacity(isLoading ? 0 : 1)
                if isLoading {
                    ProgressView().progressViewStyle(.circular).tint(.white)
                }
            }
            .frame(maxWidth: .infinity, minHeight: 56)
            .foregroundStyle(Theme.Brand.onBrand)
            .background(
                RoundedRectangle(cornerRadius: 14, style: .continuous)
                    .fill(isEnabled ? Theme.Brand.primary : Theme.Brand.primary.opacity(0.4))
            )
        }
        .buttonStyle(.plain)
        .disabled(!isEnabled || isLoading)
    }
}

/// Хоёрдогч товч — хүрээтэй, дэвсгэргүй.
struct BrandSecondaryButton: View {
    let title: String
    var systemImage: String? = nil
    var tone: Color = Theme.fg1
    var isEnabled: Bool = true
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            HStack(spacing: Theme.Space.sm) {
                if let systemImage {
                    Image(systemName: systemImage).font(.system(size: 14, weight: .medium))
                }
                Text(title).font(Theme.TypeScale.calloutBold)
            }
            .frame(maxWidth: .infinity, minHeight: 48)
            .foregroundStyle(isEnabled ? tone : Theme.fg4)
            .background(
                RoundedRectangle(cornerRadius: 14, style: .continuous)
                    .strokeBorder(isEnabled ? Theme.borderStrong : Theme.border, lineWidth: 1)
            )
        }
        .buttonStyle(.plain)
        .disabled(!isEnabled)
    }
}

/// Тексттэй холбоос маягийн товч — «Өөр төхөөрөмж дээрээ зөвшөөрөх».
struct BrandLinkButton: View {
    let title: String
    let action: () -> Void
    var body: some View {
        Button(action: action) {
            Text(title)
                .font(Theme.TypeScale.footnoteBold)
                .foregroundStyle(Theme.Brand.primary)
        }
        .buttonStyle(.plain)
    }
}
