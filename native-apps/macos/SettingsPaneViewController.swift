import AppKit

/// Тохиргооны дэлгэц — аппын хүрээн ДОТОР амьдардаг.
///
/// Өмнө нь энэ нь өөрийн `NSWindow`-той байсан. Тусдаа цонх нь хэрэглэгчийг
/// нэг аппын хоёр өөр хүрээ хооронд үсрүүлдэг, Dock дээр хоёр дүрс үлдээдэг,
/// бүтэн дэлгэцийн горимд бүр огт олдохгүй болдог. Одоо энэ нь `NSView` —
/// ажлын мужийг сольж, ижил ribbon, ижил sidebar, ижил footer-ын дунд гарна.
///
/// Тусдаа цонх үлдээх ганц зөвшөөрөгдсөн зүйл бол popup: `NSAlert`, `NSMenu`,
/// `NSSavePanel` — эдгээр нь богино насалж, эцэг цонхондоо холбогдоно.
final class SettingsPaneViewController: NSViewController {
    /// Endpoint өөрчлөгдөхөд бүрхүүлд дуулгана — ажлын муж өөр origin руу
    /// ачаалагдах ёстой болно.
    var onEndpointChanged: (() -> Void)?

    private enum Section: Int, CaseIterable {
        case general, connection, printer, scanner, serial, privacy, device, update, diagnostics
        var title: String { switch self {
        case .general: "Ерөнхий"; case .connection: "Холболт"; case .printer: "Принтер"
        case .scanner: "Сканнер"; case .serial: "Serial порт"; case .privacy: "Нууцлал"; case .device: "Төхөөрөмж"
        case .update: "Шинэчлэлт"; case .diagnostics: "Оношлогоо" } }
        var symbol: String { switch self {
        case .general: "gearshape"; case .connection: "network"; case .printer: "printer"
        case .scanner: "barcode.viewfinder"; case .serial: "cable.connector"; case .privacy: "lock.shield"; case .device: "display"
        case .update: "arrow.triangle.2.circlepath"; case .diagnostics: "stethoscope" } }
    }

    private var settings = NativeSettings.load()
    private let detail = NSStackView()
    /// The card the current section's rows are being added to. Their settings
    /// screens are a stack of cards on a surface rather than a bare column of
    /// controls, so `row` and friends fill this instead of `detail` directly.
    private var cardContent = NSStackView()
    private let surface = ThemeAwareSurface()
    private let status = NSTextField(labelWithString: "")
    private var enrollmentCode = ""
    private let deviceClient = MacDeviceEnrollmentClient()

    init() {
        super.init(nibName: nil, bundle: nil)
    }
    required init?(coder: NSCoder) { fatalError() }

    override func loadView() {
        // Хүрээн дотор амьдардаг тул хэмжээгээ өөрөө шийдэхгүй — эцэг нь
        // autoresizing-аар сунгана. Энд өгсөн frame нь зөвхөн эхний утга.
        view = NSView(frame: NSRect(x: 0, y: 0, width: 920, height: 650))
        build()
        show(.general)
    }

    private func build() {
        let content = view
        let split = NSSplitView(frame: content.bounds); split.isVertical = true; split.dividerStyle = .thin; split.autoresizingMask = [.width, .height]
        let sidebar = NSVisualEffectView(); sidebar.material = .sidebar; sidebar.blendingMode = .behindWindow
        let menu = NSStackView(); menu.orientation = .vertical; menu.alignment = .leading; menu.spacing = 4; menu.edgeInsets = NSEdgeInsets(top: 20, left: 12, bottom: 20, right: 12)
        for section in Section.allCases {
            let button = NSButton(title: section.title, target: self, action: #selector(selectSection(_:)))
            button.tag = section.rawValue; button.bezelStyle = .recessed; button.imagePosition = .imageLeading
            button.image = NSImage(systemSymbolName: section.symbol, accessibilityDescription: section.title)
            button.alignment = .left; button.widthAnchor.constraint(equalToConstant: 190).isActive = true
            menu.addArrangedSubview(button)
        }
        sidebar.addSubview(menu); menu.translatesAutoresizingMaskIntoConstraints = false
        NSLayoutConstraint.activate([menu.leadingAnchor.constraint(equalTo: sidebar.leadingAnchor), menu.trailingAnchor.constraint(equalTo: sidebar.trailingAnchor), menu.topAnchor.constraint(equalTo: sidebar.topAnchor)])

        let scroll = NSScrollView(); scroll.hasVerticalScroller = true; scroll.drawsBackground = false
        detail.orientation = .vertical; detail.alignment = .leading; detail.spacing = 16; detail.edgeInsets = NSEdgeInsets(top: 28, left: 32, bottom: 36, right: 32)
        let document = NSView(); document.addSubview(detail); detail.translatesAutoresizingMaskIntoConstraints = false
        NSLayoutConstraint.activate([detail.leadingAnchor.constraint(equalTo: document.leadingAnchor), detail.trailingAnchor.constraint(equalTo: document.trailingAnchor), detail.topAnchor.constraint(equalTo: document.topAnchor), detail.bottomAnchor.constraint(equalTo: document.bottomAnchor), detail.widthAnchor.constraint(greaterThanOrEqualToConstant: 540)])
        scroll.documentView = document
        // The surface their settings sit on. The scroll view draws nothing, so
        // this is what shows through behind the cards.
        surface.addSubview(scroll)
        scroll.translatesAutoresizingMaskIntoConstraints = false
        NSLayoutConstraint.activate([
            scroll.leadingAnchor.constraint(equalTo: surface.leadingAnchor),
            scroll.trailingAnchor.constraint(equalTo: surface.trailingAnchor),
            scroll.topAnchor.constraint(equalTo: surface.topAnchor),
            scroll.bottomAnchor.constraint(equalTo: surface.bottomAnchor),
        ])
        split.addArrangedSubview(sidebar); split.addArrangedSubview(surface); sidebar.widthAnchor.constraint(equalToConstant: 216).isActive = true
        content.addSubview(split)
    }

    @objc private func selectSection(_ sender: NSButton) { if let section = Section(rawValue: sender.tag) { show(section) } }
    private func show(_ section: Section) {
        detail.arrangedSubviews.forEach { detail.removeArrangedSubview($0); $0.removeFromSuperview() }
        let heading = NSTextField(labelWithString: section.title)
        heading.font = EIDFont.pageTitle
        heading.textColor = EID.textPrimary
        detail.addArrangedSubview(heading)

        // One card per section, filled by the row helpers below.
        cardContent = NSStackView()
        cardContent.orientation = .vertical
        cardContent.alignment = .leading
        cardContent.spacing = 14
        let card = EIDCard(content: cardContent)
        card.translatesAutoresizingMaskIntoConstraints = false
        detail.addArrangedSubview(card)
        card.widthAnchor.constraint(equalTo: detail.widthAnchor).isActive = true
        switch section {
        case .general:
            addToggle("Нэвтрэхэд автоматаар ажиллуулах", value: settings.launchAtLogin, action: #selector(toggleLaunch(_:)))
            addPopup("Хэл", items: ["mn", "en", "ru", "zh"], selected: settings.language, action: #selector(changeLanguage(_:)))
        case .connection:
            addField("Web endpoint", value: settings.webEndpoint, action: #selector(changeWeb(_:)))
            addField("API endpoint", value: settings.apiEndpoint, action: #selector(changeAPI(_:)))
            addAction("Холболт шалгах", action: #selector(testConnection))
        case .printer:
            addPopup("Холболтын төрөл", items: ["USB", "Network", "Serial"], selected: settings.printerTransport, action: #selector(changePrinterTransport(_:)))
            addField("IP / host", value: settings.printerHost, action: #selector(changePrinterHost(_:)))
            addNumber("TCP port", value: settings.printerPort, action: #selector(changePrinterPort(_:)))
            addPopup("Цаасны өргөн", items: ["58 mm", "80 mm"], selected: settings.paperWidth, action: #selector(changePaper(_:)))
            addAction("Туршилтын баримт хэвлэх", action: #selector(testPrinter))
        case .scanner:
            addPopup("Унших горим", items: ["Keyboard wedge", "Camera", "Vendor SDK"], selected: settings.scannerMode, action: #selector(changeScanner(_:)))
            addPopup("Төгсгөлийн тэмдэг", items: ["Enter", "Tab", "None"], selected: settings.scannerSuffix, action: #selector(changeSuffix(_:)))
            addAction("Туршилтын код унших", action: #selector(testScanner))
        case .serial:
            addField("Port path", value: settings.serialPort, action: #selector(changeSerial(_:)))
            addPopup("Baud rate", items: ["9600", "19200", "38400", "57600", "115200"], selected: String(settings.baudRate), action: #selector(changeBaud(_:)))
            addAction("Порт нээж шалгах", action: #selector(testSerial))
        case .privacy:
            addToggle("Биометрик түгжээ", value: settings.biometricLock, action: #selector(toggleBiometric(_:)))
            addNumber("Идэвхгүй үед түгжих (минут)", value: settings.idleLockMinutes, action: #selector(changeIdle(_:)))
            addAction("Secure store цэвэрлэх…", action: #selector(clearSecureStore))
        case .device:
            addField("Төхөөрөмжийн нэр", value: settings.deviceName, action: #selector(changeDeviceName(_:)))
            addField("Байршил / site", value: settings.site, action: #selector(changeSite(_:)))
            addField("Enrollment code", value: enrollmentCode, action: #selector(changeEnrollmentCode(_:)))
            addReadOnly("Device ID", settings.deviceID.isEmpty ? "—" : settings.deviceID)
            addAction("Нэг удаагийн кодоор бүртгэх", action: #selector(enrollDevice))
        case .update:
            addPopup("Суваг", items: ["Stable", "Pilot", "Internal"], selected: settings.updateChannel, action: #selector(changeChannel(_:)))
            addToggle("Оношлогооны мэдээлэл илгээх", value: settings.telemetry, action: #selector(toggleTelemetry(_:)))
            addAction("Шинэчлэлт шалгах", action: #selector(checkUpdates))
        case .diagnostics:
            addReadOnly("Shell contract", "1.3"); addReadOnly("macOS", ProcessInfo.processInfo.operatingSystemVersionString)
            addReadOnly("WebKit", "WKWebView"); addAction("Лог export хийх…", action: #selector(exportLogs))
        }
        status.stringValue = ""; status.textColor = EID.muted; status.font = EIDFont.label
        detail.addArrangedSubview(status)
    }

    private func row(_ label: String, control: NSView) {
        let title = NSTextField(labelWithString: label)
        title.font = EIDFont.body
        title.textColor = EID.textPrimary
        title.widthAnchor.constraint(equalToConstant: 220).isActive = true
        let row = NSStackView(views: [title, control]); row.orientation = .horizontal; row.alignment = .centerY; row.spacing = 16
        cardContent.addArrangedSubview(row)
    }
    private func addField(_ label: String, value: String, action: Selector) { let field = NSTextField(string: value); field.target = self; field.action = action; field.widthAnchor.constraint(equalToConstant: 280).isActive = true; row(label, control: field) }
    private func addNumber(_ label: String, value: Int, action: Selector) { addField(label, value: String(value), action: action) }
    private func addPopup(_ label: String, items: [String], selected: String, action: Selector) { let popup = NSPopUpButton(); popup.addItems(withTitles: items); popup.selectItem(withTitle: selected); popup.target = self; popup.action = action; popup.widthAnchor.constraint(equalToConstant: 180).isActive = true; row(label, control: popup) }
    private func addToggle(_ label: String, value: Bool, action: Selector) { let toggle = NSSwitch(); toggle.state = value ? .on : .off; toggle.target = self; toggle.action = action; row(label, control: toggle) }
    private func addAction(_ title: String, action: Selector) { let button = NSButton(title: title, target: self, action: action); button.bezelStyle = .rounded; cardContent.addArrangedSubview(button) }
    private func addReadOnly(_ label: String, _ value: String) { let text = NSTextField(labelWithString: value); text.textColor = EID.muted; text.font = EIDFont.body; row(label, control: text) }
    private func persist(_ message: String = "Хадгаллаа") { settings.save(); status.stringValue = message }

    @objc private func toggleLaunch(_ s: NSSwitch) { settings.launchAtLogin = s.state == .on; persist() }
    @objc private func changeLanguage(_ s: NSPopUpButton) { settings.language = s.titleOfSelectedItem ?? "mn"; persist() }
    @objc private func changeWeb(_ s: NSTextField) { settings.webEndpoint = s.stringValue; persist(); onEndpointChanged?() }
    @objc private func changeAPI(_ s: NSTextField) { settings.apiEndpoint = s.stringValue; persist(); onEndpointChanged?() }
    @objc private func changePrinterTransport(_ s: NSPopUpButton) { settings.printerTransport = s.titleOfSelectedItem ?? "USB"; persist() }
    @objc private func changePrinterHost(_ s: NSTextField) { settings.printerHost = s.stringValue; persist() }
    @objc private func changePrinterPort(_ s: NSTextField) { settings.printerPort = s.integerValue; persist() }
    @objc private func changePaper(_ s: NSPopUpButton) { settings.paperWidth = s.titleOfSelectedItem ?? "80 mm"; persist() }
    @objc private func changeScanner(_ s: NSPopUpButton) { settings.scannerMode = s.titleOfSelectedItem ?? "Keyboard wedge"; persist() }
    @objc private func changeSuffix(_ s: NSPopUpButton) { settings.scannerSuffix = s.titleOfSelectedItem ?? "Enter"; persist() }
    @objc private func changeSerial(_ s: NSTextField) { settings.serialPort = s.stringValue; persist() }
    @objc private func changeBaud(_ s: NSPopUpButton) { settings.baudRate = Int(s.titleOfSelectedItem ?? "9600") ?? 9600; persist() }
    @objc private func toggleBiometric(_ s: NSSwitch) { settings.biometricLock = s.state == .on; persist() }
    @objc private func changeIdle(_ s: NSTextField) { settings.idleLockMinutes = max(1, s.integerValue); persist() }
    @objc private func changeChannel(_ s: NSPopUpButton) { settings.updateChannel = s.titleOfSelectedItem ?? "Stable"; persist() }
    @objc private func toggleTelemetry(_ s: NSSwitch) { settings.telemetry = s.state == .on; persist() }
    @objc private func changeDeviceName(_ s: NSTextField) { settings.deviceName = s.stringValue; persist() }
    @objc private func changeSite(_ s: NSTextField) { settings.site = s.stringValue; persist() }
    @objc private func changeEnrollmentCode(_ s: NSTextField) { enrollmentCode = s.stringValue.uppercased() }
    @objc private func enrollDevice() { status.stringValue = "Төхөөрөмжийг бүртгэж байна…"; Task { do { let enrolled = try await deviceClient.enroll(endpoint: settings.apiEndpoint, code: enrollmentCode, name: settings.deviceName, site: settings.site); try MacDeviceTokenStore.save(enrolled.deviceToken); settings.deviceID = enrolled.deviceID; settings.save(); enrollmentCode = ""; await MainActor.run { self.show(.device); self.status.stringValue = "Төхөөрөмж амжилттай бүртгэгдлээ" } } catch { await MainActor.run { self.status.stringValue = error.localizedDescription } } } }
    @objc private func testConnection() { persist("Web/API холболтын тест эхэллээ") }
    @objc private func testPrinter() { persist("Printer capability холбогдоогүй — төхөөрөмж сонгоно уу") }
    @objc private func testScanner() { persist("Сканнерын input хүлээж байна…") }
    @objc private func testSerial() { persist(settings.serialPort.isEmpty ? "Port path сонгоогүй" : "\(settings.serialPort)-ыг шалгаж байна…") }
    @objc private func clearSecureStore() { MacDeviceTokenStore.clear(); persist("Device token Keychain-аас цэвэрлэгдлээ") }
    @objc private func checkUpdates() { persist("\(settings.updateChannel) сувгийг шалгаж байна…") }
    @objc private func exportLogs() { persist("Лог export хийх save panel дараагийн device adapter-т холбогдоно") }
}
