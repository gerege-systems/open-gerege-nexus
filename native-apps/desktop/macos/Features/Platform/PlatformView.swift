import SwiftUI
import WebKit
import AppKit
import LocalAuthentication

/// Gerege Nexus-ийн ажлын муж — энэ аппын нэг дэлгэц.
///
/// Цөмийн `docs/SHELL_CONTRACT.md` §1a: клиент бүр НЭГ хүрээтэй. Платформын
/// вэб апп нь тэр хүрээн доторх дэлгэц болохоос хоёр дахь цонх биш — тиймээс
/// энэ нь sidebar-ын бусад route-той яг ижил байдлаар солигдоно.
///
/// Хаяг нь `AppConfig.baseURL` — native дуудлагууд явдаг ЯГ ТЭР гарал.
/// Ингэснээр webview доторх `/api/v1` дуудлага same-origin хэвээр үлдэж,
/// session cookie нь `SameSite=Strict` байхад ажиллана. Хоёр өөр гарал
/// ашигласан бол тэр cookie хэзээ ч илгээгдэхгүй байх байв.
struct PlatformView: View {
    @EnvironmentObject private var appState: AppState
    @ObservedObject private var loc = LocalizationService.shared

    var body: some View {
        // `URL(string:)!` БИШ: суурь хаяг нь Settings-ээс дарагддаг хэрэглэгчийн
        // утга (`AppConfig.baseURLKey`). Буруу бичсэн хаяг нь аппыг унагах
        // ёсгүй — уншигдах мессеж өгөх ёстой.
        if let url = URL(string: AppConfig.baseURL), url.scheme?.hasPrefix("http") == true {
            PlatformWebView(url: url, appState: appState)
                .background(Color.eidSurface)
        } else {
            VStack(spacing: 8) {
                Image(systemName: "exclamationmark.triangle")
                    .font(.system(size: 28))
                    .foregroundStyle(Color.eidWarning)
                Text(loc.pick("Серверийн хаяг буруу байна.",
                              "The server address is not valid.",
                              "Неверный адрес сервера.",
                              "服务器地址无效。"))
                Text(AppConfig.baseURL)
                    .font(.system(.caption, design: .monospaced))
                    .foregroundStyle(Color.eidMuted)
                    .textSelection(.enabled)
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity)
            .background(Color.eidSurface)
        }
    }
}

private struct PlatformWebView: NSViewRepresentable {
    let url: URL
    /// Гүүр нь route-ыг хөдөлгөх ёстой тул төлөвийг ЗААВАЛ дамжуулна —
    /// singleton биш: энэ аппын AppState нь `@StateObject`-оор эзэмшигддэг
    /// бөгөөд хоёр дахь хуулбар үүсгэх нь хоёр өөр үнэн болно.
    let appState: AppState

    func makeCoordinator() -> ShellBridge { ShellBridge() }

    func makeNSView(context: Context) -> WKWebView {
        let controller = WKUserContentController()
        controller.add(context.coordinator, name: "geregeShell")

        let config = WKWebViewConfiguration()
        config.userContentController = controller
        // Ажлын муж нь тусдаа цонх нээхийг оролдвол систем браузарт өгнө —
        // §1a: хүрээнээс гарч болох цорын ганц зүйл нь popup, вэб хуудас биш.
        config.preferences.javaScriptCanOpenWindowsAutomatically = false

        let webView = WKWebView(frame: .zero, configuration: config)
        webView.navigationDelegate = context.coordinator
        webView.uiDelegate = context.coordinator
        context.coordinator.webView = webView
        context.coordinator.appState = appState
        webView.load(URLRequest(url: url))
        return webView
    }

    func updateNSView(_ nsView: WKWebView, context: Context) {
        context.coordinator.appState = appState
    }
}

/// `window.GeregeShell` гүүр.
///
/// Хуучин AppKit бүрхүүлээс зөөгдсөн. Ялгаа нь ганц: `shell.openPane` нь
/// цонхны контроллер биш, `AppState`-ийн route-ыг хөдөлгөнө — энэ апп дээр
/// «бүрхүүлийн эзэмшдэг дэлгэц» гэдэг нь sidebar-ын route юм.
final class ShellBridge: NSObject, WKScriptMessageHandler, WKNavigationDelegate, WKUIDelegate {
    weak var webView: WKWebView?
    weak var appState: AppState?
    private var reloadsAfterTermination = 0

    func userContentController(_ controller: WKUserContentController, didReceive message: WKScriptMessage) {
        guard message.name == "geregeShell",
              message.frameInfo.isMainFrame,
              sameOrigin(message.frameInfo.request.url, webView?.url),
              let json = message.body as? [String: Any] else { return }
        handle(json)
    }

    /// Гуравдагч этгээдийн iframe нь бүрхүүлийн API руу хүрэх ёсгүй. Гарал
    /// тулгах нь `isMainFrame`-ийн дээр нэмэлт давхарга: main frame нь
    /// шилжсэн байж болно.
    private func sameOrigin(_ lhs: URL?, _ rhs: URL?) -> Bool {
        guard let lhs, let rhs else { return false }
        return lhs.scheme?.lowercased() == rhs.scheme?.lowercased()
            && lhs.host?.lowercased() == rhs.host?.lowercased()
            && lhs.port == rhs.port
    }

    private func handle(_ json: [String: Any]) {
        guard let method = json["method"] as? String else { return }
        let id = json["id"] as? String ?? ""
        let params = json["params"] as? [String: Any] ?? [:]

        switch method {
        case "external.open":
            guard let raw = params["url"] as? String, let url = URL(string: raw),
                  ["http", "https", "mailto", "tel"].contains(url.scheme?.lowercased() ?? "") else {
                resolve(id, ok: false, value: "URL scheme not allowed"); return
            }
            NSWorkspace.shared.open(url)
            resolve(id, ok: true, value: NSNull())

        case "print.system":
            webView?.evaluateJavaScript("window.print()") { _, error in
                self.resolve(id, ok: error == nil, value: error?.localizedDescription ?? NSNull())
            }

        case "menu.changed":
            resolve(id, ok: true, value: NSNull())

        case "shell.openPane":
            // Ажлын муж бүрхүүлийн эзэмшдэг дэлгэц рүү шилжихийг хүсэж байна.
            // Шинэ цонх нээгдэхгүй — sidebar-ын route солигдоно.
            switch params["pane"] as? String {
            case "settings":
                // Тохиргоо нь route биш ХУУДАС (sheet). Өмнө нь энд `.privacy`
                // руу заадаг байсан нь тэр цэс хасагдахад мөхөс болов.
                DispatchQueue.main.async { self.appState?.settingsPresented = true }
                resolve(id, ok: true, value: NSNull())
            case "work":
                DispatchQueue.main.async { self.appState?.selectedTab = .platform }
                resolve(id, ok: true, value: NSNull())
            default:
                resolve(id, ok: false, value: "Unknown pane")
            }

        case "auth.reLogin":
            DispatchQueue.main.async { self.appState?.logout() }
            resolve(id, ok: true, value: NSNull())

        case "auth.lock", "biometric.authenticate":
            authenticate(id)

        case "device.identity":
            resolve(id, ok: true, value: [
                "platform": "macos",
                "form_factor": "desktop",
                "app": AppConfig.serviceName,
            ])

        default:
            resolve(id, ok: false, value: "Unsupported method: \(method)")
        }
    }

    private func authenticate(_ id: String) {
        let context = LAContext()
        var error: NSError?
        guard context.canEvaluatePolicy(.deviceOwnerAuthentication, error: &error) else {
            resolve(id, ok: false, value: error?.localizedDescription ?? "Touch ID тохируулаагүй байна")
            return
        }
        context.evaluatePolicy(.deviceOwnerAuthentication,
                               localizedReason: "Ажлын хэсгийг дахин нээх") { ok, failure in
            self.resolve(id, ok: ok,
                         value: ok ? ["authenticated": true]
                                   : (failure?.localizedDescription ?? "Баталгаажуулалт цуцлагдсан"))
        }
    }

    private func resolve(_ id: String, ok: Bool, value: Any) {
        guard let idData = try? JSONSerialization.data(withJSONObject: id, options: .fragmentsAllowed),
              let valueData = try? JSONSerialization.data(withJSONObject: value, options: .fragmentsAllowed),
              let idJSON = String(data: idData, encoding: .utf8),
              let valueJSON = String(data: valueData, encoding: .utf8) else { return }
        let js = "window.__geregeShellResolve(\(idJSON),\(ok ? "true" : "false"),\(valueJSON));"
        DispatchQueue.main.async { self.webView?.evaluateJavaScript(js, completionHandler: nil) }
    }

    // MARK: - Процессын төгсгөл

    /// WebKit-ийн контентын процесс үхэхэд webview нь ХООСОН цагаан хэвээр
    /// үлддэг — өөрөө сэргэдэггүй. Санах ойн дарамт, жинхэнэ эвдрэл, эсвэл
    /// хостын процесс эхлэхгүй байх нь бүгд ижил байдлаар харагдана: хүн
    /// «Платформ дарахад юу ч гарахгүй» гэж хэлнэ.
    ///
    /// Нэг л удаа дахин ачаална. Давталтад ороход хамгаалалт хэрэгтэй:
    /// процесс нь тогтмол унаж байвал дахин ачаалах бүр түүнийг дахин
    /// унагана.
    func webViewWebContentProcessDidTerminate(_ webView: WKWebView) {
        guard reloadsAfterTermination < 3 else {
            NSLog("PlatformView: контентын процесс дахин дахин үхэж байна — дахин ачаалахаа болилоо")
            return
        }
        reloadsAfterTermination += 1
        NSLog("PlatformView: контентын процесс үхлээ — дахин ачаалж байна (\(reloadsAfterTermination))")
        webView.reload()
    }

    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
        // Амжилттай ачаалсан нь тоолуурыг тэглэнэ — «гурван удаа» гэдэг нь
        // дараалсан бүтэлгүйтэл байх ёстой, аппын насан туршийн нийлбэр биш.
        reloadsAfterTermination = 0
    }

    // MARK: - Навигаци

    /// `target="_blank"` ба `window.open` нь хоёр дахь webview нээхгүй —
    /// хаягийг систем браузарт өгнө (§1a).
    func webView(_ webView: WKWebView, createWebViewWith configuration: WKWebViewConfiguration,
                 for navigationAction: WKNavigationAction,
                 windowFeatures: WKWindowFeatures) -> WKWebView? {
        if let url = navigationAction.request.url { NSWorkspace.shared.open(url) }
        return nil
    }
}
