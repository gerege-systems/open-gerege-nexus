import AppKit
import CoreImage

/*
 * The sign-in screen, in the eID platform's design.
 *
 * Layout ported from gerege-line/eid/eid-platform-mn — desktop/macos-app,
 * Features/Login/LoginView.swift: the split with the dark diagonal panel on the
 * left, the light surface and floating card on the right, the segmented tab
 * switcher, the monospaced identifier field, the full-width primary button.
 * Their measurements are kept — 5/11 split, 820pt threshold, 450pt card, 28pt
 * padding, 18pt corner — because a design copied at nine tenths is a design
 * that looks almost right and reads as a mistake.
 *
 * What is not theirs is the flow underneath. This client signs in against Gerege
 * Nexus, so the phases, the long-poll and the password path are the ones that
 * were already here; only what is drawn around them changed.
 */

@MainActor
protocol NativeLoginDelegate: AnyObject {
    func nativeLoginDidSucceed(cookies: [HTTPCookie], profile: NativeUserProfile)
}

@MainActor
final class NativeLoginViewController: NSViewController {
    weak var delegate: NativeLoginDelegate?
    private let auth: NativeAuth

    private enum Mode { case nationalID, qr }
    private var mode: Mode = .nationalID

    // Chrome
    private let heroPanel = HeroPanel()
    private let card = NSView()
    private let surface = ThemeAwareView()
    private var heroWidth: NSLayoutConstraint?

    // Card contents
    private let idTab = NSButton(title: "РД / Иргэний бүртгэл", target: nil, action: nil)
    private let qrTab = NSButton(title: "QR код", target: nil, action: nil)
    private let tabTrack = NSView()
    private let hint = NSTextField(labelWithString: "")
    private let nationalID = NSTextField()
    private let primaryButton = NSButton(title: "Нэвтрэх", target: nil, action: nil)
    private let qrImage = NSImageView()
    private let banner = InlineBanner()
    private let status = NSTextField(labelWithString: "")
    private let cancelButton = NSButton(title: "Цуцлах", target: nil, action: nil)

    // The administrator's password path, behind a disclosure the way the web
    // sign-in screen keeps it: it is a fallback, not the way in.
    private let adminToggle = NSButton(title: "Системийн админ нэвтрэлт", target: nil, action: nil)
    private let email = NSTextField()
    private let password = NSSecureTextField()
    private let passwordButton = NSButton(title: "Админаар нэвтрэх", target: nil, action: nil)
    private var adminStack = NSStackView()

    private var idSection = NSStackView()
    private var qrSection = NSStackView()

    init(apiEndpoint: String) { auth = NativeAuth(apiEndpoint: apiEndpoint); super.init(nibName: nil, bundle: nil) }
    required init?(coder: NSCoder) { fatalError() }

    override func loadView() {
        let root = NSView()
        root.wantsLayer = true

        // The palette is theme-aware, so the appearance is no longer forced to
        // dark: a window pinned to darkAqua would resolve every dynamic colour
        // to its dark half and the light design would never be seen.
        surface.wantsLayer = true
        surface.translatesAutoresizingMaskIntoConstraints = false
        heroPanel.translatesAutoresizingMaskIntoConstraints = false
        root.addSubview(heroPanel)
        root.addSubview(surface)

        buildCard()
        card.translatesAutoresizingMaskIntoConstraints = false
        surface.addSubview(card)

        let heroConstraint = heroPanel.widthAnchor.constraint(equalTo: root.widthAnchor, multiplier: 5.0 / 11.0)
        heroWidth = heroConstraint
        NSLayoutConstraint.activate([
            heroPanel.leadingAnchor.constraint(equalTo: root.leadingAnchor),
            heroPanel.topAnchor.constraint(equalTo: root.topAnchor),
            heroPanel.bottomAnchor.constraint(equalTo: root.bottomAnchor),
            heroConstraint,

            surface.leadingAnchor.constraint(equalTo: heroPanel.trailingAnchor),
            surface.trailingAnchor.constraint(equalTo: root.trailingAnchor),
            surface.topAnchor.constraint(equalTo: root.topAnchor),
            surface.bottomAnchor.constraint(equalTo: root.bottomAnchor),

            card.centerXAnchor.constraint(equalTo: surface.centerXAnchor),
            card.centerYAnchor.constraint(equalTo: surface.centerYAnchor),
            card.widthAnchor.constraint(equalToConstant: 450),
            card.leadingAnchor.constraint(greaterThanOrEqualTo: surface.leadingAnchor, constant: 20),
        ])

        view = root
        // The theme hook is on NSView; a controller never hears about it.
        surface.onAppearanceChange = { [weak self] in self?.applyColors() }
        applyColors()
        auth.onPhase = { [weak self] in self?.render($0) }
        select(.nationalID)
        render(.idle)
    }

    override func viewDidLayout() {
        super.viewDidLayout()
        // Their 820pt threshold: below it the hero panel goes and the card has
        // the window to itself, which is what keeps a narrow window usable
        // rather than squeezing two panels into it.
        heroPanel.isHidden = view.bounds.width < 820
        heroWidth?.constant = 0
        heroWidth?.isActive = !heroPanel.isHidden
    }

    /// Layer-backed colours are resolved CGColors and do not follow a theme
    /// switch on their own. See NSColor.eidCGColor.
    private func applyColors() {
        heroPanel.needsDisplay = true
        surface.layer?.backgroundColor = EID.surface.eidCGColor(for: view)
        card.layer?.backgroundColor = EID.cardBackground.eidCGColor(for: view)
        card.layer?.borderColor = EID.cardStroke.eidCGColor(for: view)
        tabTrack.layer?.backgroundColor = EID.muted.withAlphaComponent(0.12).eidCGColor(for: view)
        styleTabs()
        [nationalID, email, password].forEach { field in
            field.layer?.borderColor = EID.cardStroke.eidCGColor(for: view)
            field.layer?.backgroundColor = EID.muted.withAlphaComponent(0.08).eidCGColor(for: view)
        }
        primaryButton.layer?.backgroundColor = EID.accent.eidCGColor(for: view)
        passwordButton.layer?.backgroundColor = EID.accent.eidCGColor(for: view)
    }

    // MARK: - Card

    private func buildCard() {
        card.wantsLayer = true
        card.layer?.cornerRadius = EIDRadius.card
        card.layer?.cornerCurve = .continuous
        card.layer?.borderWidth = 1
        card.shadow = NSShadow()
        card.layer?.shadowColor = NSColor.black.cgColor
        card.layer?.shadowOpacity = 0.12
        card.layer?.shadowRadius = 22
        card.layer?.shadowOffset = CGSize(width: 0, height: -10)

        let title = NSTextField(labelWithString: "Нэвтрэх")
        title.font = EIDFont.cardTitle
        title.textColor = EID.textPrimary
        let subtitle = NSTextField(labelWithString: "eID Mongolia апп-аар нэвтрэх")
        subtitle.font = EIDFont.label
        subtitle.textColor = EID.muted

        buildTabs()
        buildIDSection()
        buildQRSection()
        buildAdminSection()

        status.font = EIDFont.label
        status.textColor = EID.muted
        status.maximumNumberOfLines = 3
        styleSecondary(cancelButton)
        cancelButton.target = self
        cancelButton.action = #selector(cancel)

        let divider = NSBox()
        divider.boxType = .separator

        let stack = NSStackView(views: [
            title, subtitle, tabTrack, banner, idSection, qrSection,
            status, cancelButton, divider, adminToggle, adminStack,
        ])
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 16
        stack.setCustomSpacing(4, after: title)
        stack.setCustomSpacing(6, after: divider)
        stack.translatesAutoresizingMaskIntoConstraints = false
        card.addSubview(stack)

        NSLayoutConstraint.activate([
            stack.leadingAnchor.constraint(equalTo: card.leadingAnchor, constant: 28),
            stack.trailingAnchor.constraint(equalTo: card.trailingAnchor, constant: -28),
            stack.topAnchor.constraint(equalTo: card.topAnchor, constant: 28),
            stack.bottomAnchor.constraint(equalTo: card.bottomAnchor, constant: -28),
        ])
        [tabTrack, banner, idSection, qrSection, status, cancelButton, divider, adminToggle, adminStack].forEach {
            $0.translatesAutoresizingMaskIntoConstraints = false
            let width = $0.widthAnchor.constraint(equalTo: stack.widthAnchor)
            width.priority = .init(999)
            width.isActive = true
        }
    }

    private func buildTabs() {
        tabTrack.wantsLayer = true
        tabTrack.layer?.cornerRadius = 12
        tabTrack.layer?.cornerCurve = .continuous

        [idTab, qrTab].forEach { tab in
            tab.isBordered = false
            tab.wantsLayer = true
            tab.font = EIDFont.tab
            tab.layer?.cornerRadius = 9
            tab.layer?.cornerCurve = .continuous
            tab.target = self
            tab.translatesAutoresizingMaskIntoConstraints = false
            tab.heightAnchor.constraint(equalToConstant: 32).isActive = true
        }
        idTab.action = #selector(selectIDTab)
        qrTab.action = #selector(selectQRTab)

        let row = NSStackView(views: [idTab, qrTab])
        row.distribution = .fillEqually
        row.spacing = 4
        row.translatesAutoresizingMaskIntoConstraints = false
        tabTrack.addSubview(row)
        NSLayoutConstraint.activate([
            row.leadingAnchor.constraint(equalTo: tabTrack.leadingAnchor, constant: 4),
            row.trailingAnchor.constraint(equalTo: tabTrack.trailingAnchor, constant: -4),
            row.topAnchor.constraint(equalTo: tabTrack.topAnchor, constant: 4),
            row.bottomAnchor.constraint(equalTo: tabTrack.bottomAnchor, constant: -4),
        ])
    }

    private func buildIDSection() {
        hint.stringValue = "РД эсвэл иргэний бүртгэлийн дугаараа оруулна уу. Утсан дээрх eID Mongolia апп-д мэдэгдэл ирнэ."
        hint.font = EIDFont.label
        hint.textColor = EID.muted
        hint.alignment = .center
        hint.maximumNumberOfLines = 3

        let label = NSTextField(labelWithString: "РД / Иргэний бүртгэлийн дугаар")
        label.font = EIDFont.label
        label.textColor = EID.muted

        nationalID.alignment = .center
        styleField(nationalID, placeholder: "AA00000000", font: EIDFont.identifier)
        stylePrimary(primaryButton)
        primaryButton.target = self
        primaryButton.action = #selector(loginPush)

        idSection = NSStackView(views: [hint, label, nationalID, primaryButton])
        idSection.orientation = .vertical
        idSection.alignment = .leading
        idSection.spacing = 6
        idSection.setCustomSpacing(12, after: hint)
        idSection.setCustomSpacing(12, after: nationalID)
        [hint, label, nationalID, primaryButton].forEach {
            $0.translatesAutoresizingMaskIntoConstraints = false
            let width = $0.widthAnchor.constraint(equalTo: idSection.widthAnchor)
            width.priority = .init(999)
            width.isActive = true
        }
    }

    private func buildQRSection() {
        qrImage.imageScaling = .scaleProportionallyUpOrDown
        qrImage.wantsLayer = true
        qrImage.layer?.cornerRadius = 10
        qrImage.layer?.backgroundColor = NSColor.white.cgColor
        let caption = NSTextField(labelWithString: "eID Mongolia апп-аар QR кодыг уншуулна уу.")
        caption.font = EIDFont.label
        caption.textColor = EID.muted
        caption.alignment = .center
        caption.maximumNumberOfLines = 2

        qrSection = NSStackView(views: [qrImage, caption])
        qrSection.orientation = .vertical
        qrSection.alignment = .centerX
        qrSection.spacing = 14
        qrImage.translatesAutoresizingMaskIntoConstraints = false
        NSLayoutConstraint.activate([
            qrImage.widthAnchor.constraint(equalToConstant: 190),
            qrImage.heightAnchor.constraint(equalToConstant: 190),
        ])
        caption.translatesAutoresizingMaskIntoConstraints = false
        caption.widthAnchor.constraint(equalTo: qrSection.widthAnchor).isActive = true
    }

    private func buildAdminSection() {
        adminToggle.isBordered = false
        adminToggle.font = EIDFont.label
        adminToggle.contentTintColor = EID.accent
        adminToggle.target = self
        adminToggle.action = #selector(toggleAdmin)

        styleField(email, placeholder: "И-мэйл", font: NSFont.systemFont(ofSize: 14))
        styleField(password, placeholder: "Нууц үг", font: NSFont.systemFont(ofSize: 14))
        stylePrimary(passwordButton)
        passwordButton.target = self
        passwordButton.action = #selector(loginPassword)

        adminStack = NSStackView(views: [email, password, passwordButton])
        adminStack.orientation = .vertical
        adminStack.alignment = .leading
        adminStack.spacing = 8
        adminStack.isHidden = true
        [email, password, passwordButton].forEach {
            $0.translatesAutoresizingMaskIntoConstraints = false
            let width = $0.widthAnchor.constraint(equalTo: adminStack.widthAnchor)
            width.priority = .init(999)
            width.isActive = true
        }
    }

    // MARK: - Styling

    private func styleField(_ field: NSTextField, placeholder: String, font: NSFont) {
        field.font = font
        field.textColor = EID.textPrimary
        field.isBezeled = false
        field.drawsBackground = false
        field.wantsLayer = true
        field.layer?.cornerRadius = 10
        field.layer?.cornerCurve = .continuous
        field.layer?.borderWidth = 1
        field.focusRingType = .default
        // A placeholder does not inherit the field's alignment — it carries its
        // own paragraph style or it sits left while the typed text is centred.
        let centred = NSMutableParagraphStyle()
        centred.alignment = field.alignment
        field.placeholderAttributedString = NSAttributedString(
            string: placeholder,
            attributes: [.foregroundColor: EID.muted, .font: font, .paragraphStyle: centred]
        )
        let height = field.heightAnchor.constraint(equalToConstant: 46)
        height.priority = .init(999)
        height.isActive = true
    }

    private func stylePrimary(_ button: NSButton) {
        button.isBordered = false
        button.wantsLayer = true
        // Their PrimaryButtonStyle rounds by Radius.md, not by the field radius.
        button.layer?.cornerRadius = EIDRadius.md
        button.layer?.cornerCurve = .continuous
        button.font = EIDFont.primaryButton
        button.contentTintColor = .white
        button.attributedTitle = NSAttributedString(
            string: button.title,
            attributes: [.foregroundColor: NSColor.white, .font: EIDFont.primaryButton]
        )
        let height = button.heightAnchor.constraint(equalToConstant: 50)
        height.priority = .init(999)
        height.isActive = true
    }

    private func styleSecondary(_ button: NSButton) {
        button.isBordered = false
        button.font = EIDFont.label
        button.contentTintColor = EID.muted
    }

    private func styleTabs() {
        for (tab, active) in [(idTab, mode == .nationalID), (qrTab, mode == .qr)] {
            tab.layer?.backgroundColor = active ? EID.cardBackground.eidCGColor(for: view) : NSColor.clear.cgColor
            tab.layer?.shadowOpacity = active ? 0.10 : 0
            tab.layer?.shadowRadius = 3
            tab.layer?.shadowOffset = CGSize(width: 0, height: -1)
            tab.layer?.shadowColor = NSColor.black.cgColor
            tab.attributedTitle = NSAttributedString(
                string: tab.title,
                attributes: [
                    .foregroundColor: active ? EID.textPrimary : EID.muted,
                    .font: EIDFont.tab,
                ]
            )
        }
    }

    // MARK: - Actions

    @objc private func selectIDTab() { select(.nationalID) }
    @objc private func selectQRTab() { select(.qr) }

    private func select(_ next: Mode) {
        mode = next
        styleTabs()
        idSection.isHidden = next != .nationalID
        qrSection.isHidden = next != .qr
        if next == .qr { auth.qr() } else { auth.cancel() }
    }

    @objc private func toggleAdmin() { adminStack.isHidden.toggle() }
    @objc private func loginPassword() { auth.password(email: email.stringValue, password: password.stringValue) }
    @objc private func loginPush() { auth.push(nationalID: nationalID.stringValue) }
    @objc private func cancel() { auth.cancel() }

    private func render(_ phase: LoginPhase) {
        let pending: Bool
        banner.isHidden = true
        switch phase {
        case .idle:
            status.stringValue = ""; pending = false
        case .starting:
            status.stringValue = "Хүсэлт эхлүүлж байна…"; pending = true
        case .waiting(let code, let link):
            status.stringValue = "eID апп дээр зөвшөөрнө үү. Баталгаажуулах код: \(code)"
            pending = true
            renderQR(link)
        case .success:
            status.stringValue = "Амжилттай нэвтэрлээ"; pending = false
            delegate?.nativeLoginDidSucceed(cookies: auth.sessionCookies(), profile: auth.profile ?? .eidUser)
        case .expired:
            show(error: "Хүсэлтийн хугацаа дууслаа. Дахин оролдоно уу."); pending = false
        case .refused:
            show(error: "eID хүсэлтээс татгалзлаа."); pending = false
        case .error(let message):
            show(error: message); pending = false
        }
        primaryButton.isEnabled = !pending
        passwordButton.isEnabled = !pending
        cancelButton.isHidden = !pending
    }

    private func show(error message: String) {
        status.stringValue = ""
        banner.text = message
        banner.isHidden = false
    }

    private func renderQR(_ link: URL?) {
        guard let link, let filter = CIFilter(name: "CIQRCodeGenerator") else { return }
        filter.setValue(Data(link.absoluteString.utf8), forKey: "inputMessage")
        filter.setValue("Q", forKey: "inputCorrectionLevel")
        guard let output = filter.outputImage?.transformed(by: CGAffineTransform(scaleX: 8, y: 8)) else { return }
        let rep = NSCIImageRep(ciImage: output)
        let image = NSImage(size: rep.size)
        image.addRepresentation(rep)
        qrImage.image = image
    }
}

/// A view that says when its theme changed. AppKit puts that hook on NSView and
/// nowhere else, so a controller that paints layers needs one of these to hear
/// about it.
private final class ThemeAwareView: NSView {
    var onAppearanceChange: (() -> Void)?

    override func viewDidChangeEffectiveAppearance() {
        super.viewDidChangeEffectiveAppearance()
        onAppearanceChange?()
    }
}

// MARK: - Hero panel

/// The dark diagonal panel. Drawn rather than composed from subviews: it is a
/// gradient and two soft radial washes, which is three draw calls and no layout.
private final class HeroPanel: NSView {
    override var isFlipped: Bool { true }

    override func draw(_ dirtyRect: NSRect) {
        guard let context = NSGraphicsContext.current?.cgContext else { return }

        // Their stops: 0.0, 0.45, 1.0 — top-leading to bottom-trailing.
        let colors = EID.heroGradient.map { $0.cgColor } as CFArray
        if let gradient = CGGradient(colorsSpace: CGColorSpaceCreateDeviceRGB(),
                                     colors: colors, locations: [0.0, 0.45, 1.0]) {
            context.drawLinearGradient(
                gradient,
                start: CGPoint(x: bounds.minX, y: bounds.minY),
                end: CGPoint(x: bounds.maxX, y: bounds.maxY),
                options: []
            )
        }

        wash(context, color: Brand.b400.withAlphaComponent(0.16),
             centre: CGPoint(x: bounds.maxX - 40, y: bounds.minY + 40), radius: 260)
        wash(context, color: EID.heroBokehSecondary.withAlphaComponent(0.13),
             centre: CGPoint(x: bounds.minX + 50, y: bounds.maxY - 50), radius: 200)
    }

    private func wash(_ context: CGContext, color: NSColor, centre: CGPoint, radius: CGFloat) {
        guard let gradient = CGGradient(
            colorsSpace: CGColorSpaceCreateDeviceRGB(),
            colors: [color.cgColor, color.withAlphaComponent(0).cgColor] as CFArray,
            locations: [0, 1]
        ) else { return }
        context.drawRadialGradient(gradient, startCenter: centre, startRadius: 0,
                                   endCenter: centre, endRadius: radius, options: [])
    }

    override func viewDidMoveToWindow() {
        super.viewDidMoveToWindow()
        guard subviews.isEmpty else { return }

        let mark = NSImageView()
        mark.image = brandMark()
        mark.imageScaling = .scaleProportionallyUpOrDown
        mark.wantsLayer = true
        mark.layer?.cornerRadius = 22
        mark.layer?.cornerCurve = .continuous
        mark.layer?.masksToBounds = true
        mark.translatesAutoresizingMaskIntoConstraints = false
        mark.widthAnchor.constraint(equalToConstant: 88).isActive = true
        mark.heightAnchor.constraint(equalToConstant: 88).isActive = true
        mark.isHidden = mark.image == nil

        let title = NSTextField(labelWithString: "Gerege Nexus")
        title.font = .systemFont(ofSize: 34, weight: .bold)
        title.textColor = .white
        let subtitle = NSTextField(labelWithString: "Төрийн болон хувийн хэвшлийн нэгдсэн ажлын орчин")
        subtitle.font = .systemFont(ofSize: 13.5)
        subtitle.textColor = EID.heroSubtitle
        subtitle.alignment = .center
        subtitle.maximumNumberOfLines = 2

        let centre = NSStackView(views: [mark, title, subtitle])
        centre.orientation = .vertical
        centre.alignment = .centerX
        centre.spacing = 6
        centre.setCustomSpacing(22, after: mark)
        centre.translatesAutoresizingMaskIntoConstraints = false
        addSubview(centre)

        let strap = NSTextField(labelWithString: "Найдвартай & Баталгаатай")
        strap.font = .systemFont(ofSize: 21, weight: .bold)
        strap.textColor = .white
        let body = NSTextField(labelWithString: "Нэг бүртгэлээр платформын бүх үйлчилгээ рүү — eID Mongolia-гийн баталгаажуулалттай.")
        body.font = .systemFont(ofSize: 12)
        body.textColor = EID.heroBody
        body.maximumNumberOfLines = 3

        let bottom = NSStackView(views: [strap, body])
        bottom.orientation = .vertical
        bottom.alignment = .leading
        bottom.spacing = 10
        bottom.translatesAutoresizingMaskIntoConstraints = false
        addSubview(bottom)

        NSLayoutConstraint.activate([
            centre.centerXAnchor.constraint(equalTo: centerXAnchor),
            centre.centerYAnchor.constraint(equalTo: centerYAnchor, constant: -40),
            centre.leadingAnchor.constraint(greaterThanOrEqualTo: leadingAnchor, constant: 36),
            centre.trailingAnchor.constraint(lessThanOrEqualTo: trailingAnchor, constant: -36),

            bottom.leadingAnchor.constraint(equalTo: leadingAnchor, constant: 36),
            bottom.trailingAnchor.constraint(equalTo: trailingAnchor, constant: -36),
            bottom.bottomAnchor.constraint(equalTo: bottomAnchor, constant: -44),
        ])
    }
}

// MARK: - Inline banner

/// Their error banner: a pastel row rather than a line of red text, so a failed
/// attempt is visible without the card changing shape around it.
private final class InlineBanner: NSView {
    private let label = NSTextField(labelWithString: "")

    var text: String {
        get { label.stringValue }
        set { label.stringValue = newValue }
    }

    init() {
        super.init(frame: .zero)
        wantsLayer = true
        layer?.cornerRadius = 10
        layer?.cornerCurve = .continuous
        layer?.borderWidth = 1
        layer?.backgroundColor = EID.bannerErrorBG.cgColor
        layer?.borderColor = EID.bannerErrorBorder.cgColor

        label.font = EIDFont.label
        label.textColor = EID.bannerErrorText
        label.maximumNumberOfLines = 4
        label.translatesAutoresizingMaskIntoConstraints = false
        addSubview(label)
        NSLayoutConstraint.activate([
            label.leadingAnchor.constraint(equalTo: leadingAnchor, constant: 12),
            label.trailingAnchor.constraint(equalTo: trailingAnchor, constant: -12),
            label.topAnchor.constraint(equalTo: topAnchor, constant: 10),
            label.bottomAnchor.constraint(equalTo: bottomAnchor, constant: -10),
        ])
    }

    required init?(coder: NSCoder) { fatalError() }
}
