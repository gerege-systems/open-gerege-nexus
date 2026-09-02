//
//  MainWindowController.swift
//  GeregeNexusNativeMac
//
//  Created for Open Gerege Nexus Desktop Platform
//  Аппын ГАНЦ хүрээ: native chrome + дотор нь солигддог дэлгэцүүд.
//

import Foundation
import AppKit
import WebKit

/// Аппын цорын ганц цонх.
///
/// Дүрэм: **popup-аас бусад бүх зүйл энэ хүрээн дотор**. Нэвтрэлт, ажлын муж,
/// тохиргоо гурвуул нэг цонхны дотор солигддог дэлгэцүүд. Тусдаа `NSWindow`
/// нээх эрхтэй зүйл бол зөвхөн `NSAlert`, `NSMenu`, `NSSavePanel` гэх мэт
/// богино насалдаг, эцэг цонхондоо холбогддог popup-ууд.
///
/// Ажлын муж нь `mac.nexus.gerege.mn` — macOS-ийн domain шугам. Тэр host нь
/// өөрийн `/api/v1`-ээ мөн үйлчилдэг тул webview доторх дуудлага same-origin
/// болж, session cookie нь `SameSite=Strict` хэвээр ажиллана.
public class MainWindowController: NSWindowController, WKNavigationDelegate, WKUIDelegate, NativeLoginDelegate {
    /// Хүрээн доторх дэлгэцүүд. Шинэ native дэлгэц нэмэх гэж байвал энд нэмнэ —
    /// цонх нэмэхгүй.
    private enum Pane: Int {
        case work, settings
    }

    public var webView: WKWebView!
    private var ipcBridge: NativeIPCBridge?
    private var loginController: NativeLoginViewController?
    private var settingsPane: SettingsPaneViewController?
    private var activePane: Pane = .work

    private let ribbon = NSView()
    private let contentArea = NSView()
    private let railStack = NSStackView()
    private let rail = NSView()
    private let paneHost = NSView()
    private let profileButton = NSButton()
    private let footer = NSView()
    private let footerStatus = NSTextField(labelWithString: "●  Холбогдож байна")
    private var railButtons: [NSButton] = []
    private var profile: NativeUserProfile?

    private let ribbonHeight: CGFloat = 56
    private let footerHeight: CGFloat = 30
    private let railWidth: CGFloat = 168
    private let settings = NativeSettings.load()
    private var baseURLString: String { settings.webEndpoint.trimmingCharacters(in: CharacterSet(charactersIn: "/")) }

    public init() {
        let mask: NSWindow.StyleMask = [.titled, .closable, .miniaturizable, .resizable]
        let rect = NSRect(x: 100, y: 100, width: 1280, height: 820)
        let window = NSWindow(contentRect: rect, styleMask: mask, backing: .buffered, defer: false)

        window.title = "Open Gerege Nexus Native"
        window.minSize = NSSize(width: 1024, height: 680)
        window.titlebarAppearsTransparent = false
        window.center()

        super.init(window: window)
        setupFrame()
    }

    required init?(coder: NSCoder) {
        fatalError("init(coder:) has not been implemented")
    }

    // MARK: - Хүрээ

    /// Ribbon → (rail + pane host) → footer гэсэн гурван мөрийг нэг удаа барина.
    ///
    /// Байрлалыг гар аргаар frame бодохын оронд Auto Layout-аар тогтоосон нь
    /// санаатай: дэлгэц солигдох болгонд ажлын мужийн өндрийг дахин тооцох
    /// шаардлагагүй болж, footer нь бүтэн дэлгэцийн горимд ч байрандаа үлдэнэ.
    private func setupFrame() {
        guard let window = window, let content = window.contentView else { return }
        content.wantsLayer = true

        setupWebView()
        setupRibbon()
        setupRail()
        setupFooter()

        [ribbon, contentArea, footer].forEach {
            $0.translatesAutoresizingMaskIntoConstraints = false
            content.addSubview($0)
        }
        [rail, paneHost].forEach {
            $0.translatesAutoresizingMaskIntoConstraints = false
            contentArea.addSubview($0)
        }

        NSLayoutConstraint.activate([
            ribbon.topAnchor.constraint(equalTo: content.topAnchor),
            ribbon.leadingAnchor.constraint(equalTo: content.leadingAnchor),
            ribbon.trailingAnchor.constraint(equalTo: content.trailingAnchor),
            ribbon.heightAnchor.constraint(equalToConstant: ribbonHeight),

            contentArea.topAnchor.constraint(equalTo: ribbon.bottomAnchor),
            contentArea.leadingAnchor.constraint(equalTo: content.leadingAnchor),
            contentArea.trailingAnchor.constraint(equalTo: content.trailingAnchor),
            contentArea.bottomAnchor.constraint(equalTo: footer.topAnchor),

            footer.leadingAnchor.constraint(equalTo: content.leadingAnchor),
            footer.trailingAnchor.constraint(equalTo: content.trailingAnchor),
            footer.bottomAnchor.constraint(equalTo: content.bottomAnchor),
            footer.heightAnchor.constraint(equalToConstant: footerHeight),

            rail.topAnchor.constraint(equalTo: contentArea.topAnchor),
            rail.leadingAnchor.constraint(equalTo: contentArea.leadingAnchor),
            rail.bottomAnchor.constraint(equalTo: contentArea.bottomAnchor),
            rail.widthAnchor.constraint(equalToConstant: railWidth),

            paneHost.topAnchor.constraint(equalTo: contentArea.topAnchor),
            paneHost.leadingAnchor.constraint(equalTo: rail.trailingAnchor),
            paneHost.trailingAnchor.constraint(equalTo: contentArea.trailingAnchor),
            paneHost.bottomAnchor.constraint(equalTo: contentArea.bottomAnchor),
        ])

        webView.frame = paneHost.bounds
        webView.autoresizingMask = [.width, .height]
        paneHost.addSubview(webView)

        showNativeLogin()
    }

    private func setupWebView() {
        let config = WKWebViewConfiguration()
        let contentController = WKUserContentController()

        // Bridge Contract v1.4. The object exists before hydration and only in
        // the main frame. Native replies through one JSON-only entry point.
        let initScript = WKUserScript(
            source: """
            (() => {
              if (window.GeregeShell) return;
              const pending = new Map(), listeners = new Map(); let sequence = 0;
              window.__geregeShellResolve = (id, ok, value) => {
                const pair = pending.get(id); if (!pair) return; pending.delete(id);
                ok ? pair.resolve(value) : pair.reject(new Error(String(value)));
              };
              window.__geregeShellEmit = (name, payload) =>
                (listeners.get(name) || []).slice().forEach(fn => fn(payload));
              window.GeregeShell = Object.freeze({
                version: '1.4', platform: 'macos', formFactor: 'desktop',
                capabilities: Object.freeze(['external.open', 'print.system', 'biometric', 'device.identity', 'shell.pane']),
                invoke(method, params = {}) {
                  return new Promise((resolve, reject) => {
                    const id = String(++sequence); pending.set(id, {resolve, reject});
                    window.webkit.messageHandlers.geregeShell.postMessage({id, method, params});
                  });
                },
                on(name, handler) {
                  const list = listeners.get(name) || []; list.push(handler); listeners.set(name, list);
                  return () => { const i = list.indexOf(handler); if (i >= 0) list.splice(i, 1); };
                }
              });
              document.documentElement.setAttribute('data-shell', 'macos');
            })();
            """,
            injectionTime: .atDocumentStart,
            forMainFrameOnly: true
        )
        contentController.addUserScript(initScript)

        config.userContentController = contentController
        // Шинэ цонх нээх эрхийг хаана. Ажлын мужаас гарсан `window.open` нь
        // хүрээнээс тасарсан хоёр дахь цонх үүсгэдэг байсан; одоо WKUIDelegate
        // тэдгээрийг барьж аваад системийн хөтөч рүү явуулна.
        config.preferences.javaScriptCanOpenWindowsAutomatically = false

        webView = WKWebView(frame: .zero, configuration: config)
        webView.navigationDelegate = self
        webView.uiDelegate = self
        webView.customUserAgent = "GeregeNexusNativeMac/1.0 (Macintosh; Intel Mac OS X)"

        ipcBridge = NativeIPCBridge(webView: webView, windowController: self)
        contentController.add(ipcBridge!, name: "geregeShell")
    }

    private func setupFooter() {
        footer.wantsLayer = true
        footer.layer?.backgroundColor = EID.chrome.cgColor
        footer.isHidden = true
        footerStatus.textColor = EID.chromeAccent
        footerStatus.font = .systemFont(ofSize: 11, weight: .medium)
        let info = NSTextField(labelWithString: "macOS · Desktop   |   Shell v1.4   |   \(URL(string: baseURLString)?.host ?? "—")")
        info.textColor = NSColor.white.withAlphaComponent(0.55); info.font = .systemFont(ofSize: 11)
        let spacer = NSView()
        let stack = NSStackView(views: [footerStatus, spacer, info]); stack.orientation = .horizontal; stack.alignment = .centerY; stack.spacing = 12; stack.translatesAutoresizingMaskIntoConstraints = false
        footer.addSubview(stack)
        NSLayoutConstraint.activate([stack.leadingAnchor.constraint(equalTo: footer.leadingAnchor, constant: 14), stack.trailingAnchor.constraint(equalTo: footer.trailingAnchor, constant: -14), stack.topAnchor.constraint(equalTo: footer.topAnchor), stack.bottomAnchor.constraint(equalTo: footer.bottomAnchor)])
        spacer.setContentHuggingPriority(.defaultLow, for: .horizontal)
    }

    private func setupRibbon() {
        ribbon.wantsLayer = true
        ribbon.layer?.backgroundColor = EID.chrome.cgColor
        ribbon.layer?.borderWidth = 1
        ribbon.layer?.borderColor = NSColor.white.withAlphaComponent(0.08).cgColor
        ribbon.isHidden = true

        let brand = NSTextField(labelWithString: "GEREGE / NEXUS")
        brand.font = .systemFont(ofSize: 12, weight: .bold)
        brand.textColor = EID.chromeAccent
        let reload = ribbonButton("arrow.clockwise", "Дахин ачаалах", #selector(ribbonReload))
        let lock = ribbonButton("lock", "Түгжих", #selector(ribbonLock))
        profileButton.target = self
        profileButton.action = #selector(showProfileMenu)
        profileButton.isBordered = false
        profileButton.imagePosition = .imageLeading
        profileButton.image = NSImage(systemSymbolName: "person.crop.circle.fill", accessibilityDescription: "Profile")
        profileButton.contentTintColor = .white
        profileButton.font = .systemFont(ofSize: 13, weight: .semibold)

        let spacer = NSView()
        // Тохиргоо ribbon-оос хасагдсан: түүнийг зүүн талын rail эзэмшинэ.
        // Нэг үйлдэл хоёр газар байвал аль нь идэвхтэйг харуулах төлөв хоёр
        // тийш хуваагдана.
        let stack = NSStackView(views: [brand, spacer, reload, lock, profileButton])
        stack.orientation = .horizontal
        stack.alignment = .centerY
        stack.spacing = 10
        stack.translatesAutoresizingMaskIntoConstraints = false
        ribbon.addSubview(stack)
        NSLayoutConstraint.activate([
            stack.leadingAnchor.constraint(equalTo: ribbon.leadingAnchor, constant: 20),
            stack.trailingAnchor.constraint(equalTo: ribbon.trailingAnchor, constant: -14),
            stack.topAnchor.constraint(equalTo: ribbon.topAnchor),
            stack.bottomAnchor.constraint(equalTo: ribbon.bottomAnchor),
            spacer.widthAnchor.constraint(greaterThanOrEqualToConstant: 20)
        ])
        spacer.setContentHuggingPriority(.defaultLow, for: .horizontal)
    }

    /// Зүүн талын native rail — хүрээн доторх дэлгэц сонгогч.
    ///
    /// Энэ нь тенантын апп цэс БИШ. Апп цэсийг ажлын муж өөрөө зурдаг (гэрээний
    /// §9), тиймээс тэднийг энд давхарлавал нэг цэс хоёр газар, хоёр өөр
    /// төлөвтэй болно. Rail-д зөвхөн бүрхүүлийн эзэмшдэг дэлгэцүүд байна.
    private func setupRail() {
        rail.wantsLayer = true
        rail.layer?.backgroundColor = EID.chrome.cgColor
        rail.isHidden = true

        railButtons = [
            railButton("square.grid.2x2", "Ажлын муж", .work),
            railButton("gearshape", "Тохиргоо", .settings),
        ]
        railStack.orientation = .vertical
        railStack.alignment = .leading
        railStack.spacing = 2
        railStack.edgeInsets = NSEdgeInsets(top: 14, left: 10, bottom: 14, right: 10)
        railButtons.forEach(railStack.addArrangedSubview)
        railStack.translatesAutoresizingMaskIntoConstraints = false
        rail.addSubview(railStack)
        NSLayoutConstraint.activate([
            railStack.leadingAnchor.constraint(equalTo: rail.leadingAnchor),
            railStack.trailingAnchor.constraint(equalTo: rail.trailingAnchor),
            railStack.topAnchor.constraint(equalTo: rail.topAnchor),
        ])
    }

    private func railButton(_ symbol: String, _ label: String, _ pane: Pane) -> NSButton {
        let button = NSButton(title: "  " + label, target: self, action: #selector(selectPane(_:)))
        button.tag = pane.rawValue
        button.isBordered = false
        button.image = NSImage(systemSymbolName: symbol, accessibilityDescription: label)
        button.imagePosition = .imageLeading
        button.alignment = .left
        button.font = .systemFont(ofSize: 13, weight: .medium)
        button.widthAnchor.constraint(equalToConstant: railWidth - 20).isActive = true
        button.heightAnchor.constraint(equalToConstant: 32).isActive = true
        return button
    }

    private func ribbonButton(_ symbol: String, _ label: String, _ action: Selector) -> NSButton {
        let button = NSButton(title: label, target: self, action: action)
        button.isBordered = false
        button.image = NSImage(systemSymbolName: symbol, accessibilityDescription: label)
        button.imagePosition = .imageLeading
        button.contentTintColor = NSColor.white.withAlphaComponent(0.82)
        button.font = .systemFont(ofSize: 12, weight: .medium)
        return button
    }

    // MARK: - Дэлгэц солих

    @objc private func selectPane(_ sender: NSButton) {
        guard let pane = Pane(rawValue: sender.tag) else { return }
        show(pane)
    }

    /// Хүрээн доторх дэлгэцийг солино. Цонх нээхгүй, цонх хаахгүй.
    private func show(_ pane: Pane) {
        activePane = pane
        if pane == .settings, settingsPane == nil {
            let controller = SettingsPaneViewController()
            controller.onEndpointChanged = { [weak self] in self?.reportEndpointChange() }
            controller.view.frame = paneHost.bounds
            controller.view.autoresizingMask = [.width, .height]
            paneHost.addSubview(controller.view)
            settingsPane = controller
        }
        webView.isHidden = pane != .work
        settingsPane?.view.isHidden = pane != .settings
        for button in railButtons {
            let selected = button.tag == pane.rawValue
            button.contentTintColor = selected ? EID.chromeAccent
                                               : NSColor.white.withAlphaComponent(0.72)
        }
    }

    public func showNativeLogin() {
        guard let content = window?.contentView else { return }
        // Нэвтрэлт бол мөн энэ хүрээний дэлгэц — chrome-ыг нуугаад бүтнээр нь
        // бүрхэнэ. Түүнээс биш тусдаа нэвтрэх цонх нээхгүй.
        ribbon.isHidden = true
        rail.isHidden = true
        footer.isHidden = true
        contentArea.isHidden = true

        if loginController == nil {
            let controller = NativeLoginViewController(apiEndpoint: settings.apiEndpoint)
            controller.delegate = self
            loginController = controller
        }
        guard let loginView = loginController?.view else { return }
        if loginView.superview == nil {
            loginView.translatesAutoresizingMaskIntoConstraints = false
            content.addSubview(loginView)
            NSLayoutConstraint.activate([
                loginView.topAnchor.constraint(equalTo: content.topAnchor),
                loginView.leadingAnchor.constraint(equalTo: content.leadingAnchor),
                loginView.trailingAnchor.constraint(equalTo: content.trailingAnchor),
                loginView.bottomAnchor.constraint(equalTo: content.bottomAnchor),
            ])
        }
        loginView.isHidden = false
    }

    func nativeLoginDidSucceed(cookies: [HTTPCookie], profile: NativeUserProfile) {
        self.profile = profile
        profileButton.title = profile.name.isEmpty ? "Хэрэглэгч" : profile.name
        let store = webView.configuration.websiteDataStore.httpCookieStore
        let group = DispatchGroup()
        for cookie in cookies {
            group.enter(); store.setCookie(cookie) { group.leave() }
        }
        group.notify(queue: .main) { [weak self] in
            guard let self else { return }
            self.loginController?.view.isHidden = true
            self.contentArea.isHidden = false
            self.ribbon.isHidden = false
            self.rail.isHidden = false
            self.footer.isHidden = false
            self.show(.work)
            // Шугамын нүүр. Тэнд тухайн платформын өөрийн эхлэх дэлгэц
            // рендерлэгдэнэ (frontend/app/line/<platform>).
            self.loadRelativePath("/")
        }
    }

    /// Тохиргооноос endpoint солигдоход шинэ хаяг руу шууд шилжүүлэхгүй.
    ///
    /// `baseURLString` нь эхлэхэд уншигдаж, cookie store, navigation
    /// allowlist, гүүрийн origin шалгалт гурвуул тэр утган дээр тогтсон байдаг
    /// — зөвхөн хуудсыг ачаалах нь тэднийг зөрүүтэй үлдээж, гүүр чимээгүй
    /// унтардаг. Тиймээс хэрэглэгчид үнэнийг нь хэлнэ.
    private func reportEndpointChange() {
        footerStatus.stringValue = "●  Шинэ хаяг аппыг дахин эхлүүлсний дараа хэрэгжинэ"
        footerStatus.textColor = .systemOrange
    }

    /// Цэсний "Тохиргоо…" ба гүүрийн `shell.openPane` хоёр эндээс ордог.
    /// Аль аль нь ижил хүрээн доторх дэлгэцийг харуулна.
    @objc public func showSettings() {
        guard !contentArea.isHidden else { return }
        show(.settings)
    }

    public func showWorkArea() {
        guard !contentArea.isHidden else { return }
        show(.work)
    }

    @objc private func ribbonReload() { reloadPage() }
    @objc private func ribbonLock() { showNativeLogin() }

    @objc private func showProfileMenu() {
        let menu = NSMenu()
        let name = NSMenuItem(title: profile?.name ?? "Хэрэглэгч", action: nil, keyEquivalent: "")
        name.isEnabled = false
        menu.addItem(name)
        if let email = profile?.email, !email.isEmpty {
            let emailItem = NSMenuItem(title: email, action: nil, keyEquivalent: "")
            emailItem.isEnabled = false
            menu.addItem(emailItem)
        }
        menu.addItem(.separator())
        menu.addItem(withTitle: "Тохиргоо…", action: #selector(showSettings), keyEquivalent: "")
        menu.addItem(withTitle: "Гарах", action: #selector(logout), keyEquivalent: "")
        menu.items.forEach { if $0.action != nil { $0.target = self } }
        menu.popUp(positioning: nil, at: NSPoint(x: 0, y: profileButton.bounds.height + 4), in: profileButton)
    }

    @objc public func logout() {
        Task {
            let endpoint = NativeSettings.load().apiEndpoint.trimmingCharacters(in: CharacterSet(charactersIn: "/"))
            let apiRoot = endpoint.hasSuffix("/api/v1") ? endpoint : endpoint + "/api/v1"
            if let url = URL(string: apiRoot + "/auth/logout") {
                var request = URLRequest(url: url); request.httpMethod = "POST"
                if let components = URLComponents(url: url, resolvingAgainstBaseURL: false) {
                    request.setValue("\(components.scheme ?? "https")://\(components.host ?? "")", forHTTPHeaderField: "Origin")
                }
                _ = try? await URLSession.shared.data(for: request)
            }
            await MainActor.run { self.clearSessionAndShowLogin() }
        }
    }

    private func clearSessionAndShowLogin() {
        profile = nil
        HTTPCookieStorage.shared.cookies?.filter { $0.name == "session_token" }.forEach(HTTPCookieStorage.shared.deleteCookie)
        let store = webView.configuration.websiteDataStore.httpCookieStore
        store.getAllCookies { cookies in
            let group = DispatchGroup()
            for cookie in cookies where cookie.name == "session_token" {
                group.enter(); store.delete(cookie) { group.leave() }
            }
            group.notify(queue: .main) { self.showNativeLogin() }
        }
    }

    public func loadRelativePath(_ path: String) {
        // Ажлын муж рүү чиглэсэн шилжилт нь ажлын мужийг ХАРУУЛНА. Үгүй бол
        // цэснээс "Апп дэлгүүр" дарахад хуудас нь нууц ачаалагдаад хэрэглэгч
        // тохиргоон дээрээ үлддэг.
        if !contentArea.isHidden { show(.work) }
        let fullURLString = baseURLString + path
        if let url = URL(string: fullURLString) {
            print("[MainWindowController] Navigating to: \(fullURLString)")
            let request = URLRequest(url: url)
            webView.load(request)
        }
    }

    public func reloadPage() {
        webView.reload()
    }

    // MARK: - WKNavigationDelegate

    public func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
        footerStatus.stringValue = "●  Холбогдсон · \(webView.url?.host ?? "Nexus")"
        footerStatus.textColor = EID.chromeAccent
        print("[MainWindowController] Page loaded successfully: \(webView.url?.absoluteString ?? "")")
    }

    public func webView(_ webView: WKWebView, decidePolicyFor navigationAction: WKNavigationAction,
                        decisionHandler: @escaping (WKNavigationActionPolicy) -> Void) {
        guard navigationAction.targetFrame?.isMainFrame != false,
              let url = navigationAction.request.url else {
            decisionHandler(.allow); return
        }
        let webOrigin = URL(string: baseURLString)
        let isWebOrigin = url.scheme == webOrigin?.scheme && url.host == webOrigin?.host && url.port == webOrigin?.port
        if isWebOrigin { decisionHandler(.allow); return }
        if ["http", "https", "mailto", "tel"].contains(url.scheme?.lowercased() ?? "") {
            NSWorkspace.shared.open(url)
        }
        decisionHandler(.cancel)
    }

    public func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) {
        footerStatus.stringValue = "●  Холболт тасарсан"
        footerStatus.textColor = .systemRed
        print("[MainWindowController] Navigation failed: \(error.localizedDescription)")
    }

    // MARK: - WKUIDelegate

    /// `target="_blank"` болон `window.open` нь шинэ webview цонх нээхийг
    /// хүсдэг. `nil` буцаах нь тэр цонхыг үүсгэхгүй гэсэн үг — хаяг нь
    /// зөвшөөрөгдсөн scheme-тэй бол системийн хөтчөөр нээгдэнэ.
    public func webView(_ webView: WKWebView, createWebViewWith configuration: WKWebViewConfiguration,
                        for navigationAction: WKNavigationAction,
                        windowFeatures: WKWindowFeatures) -> WKWebView? {
        if let url = navigationAction.request.url,
           ["http", "https", "mailto", "tel"].contains(url.scheme?.lowercased() ?? "") {
            NSWorkspace.shared.open(url)
        }
        return nil
    }
}
