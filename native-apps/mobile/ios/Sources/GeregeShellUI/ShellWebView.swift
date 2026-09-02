#if os(iOS)
import GeregeShellKit
import SwiftUI
import WebKit
import LocalAuthentication

public struct ShellWebView: UIViewRepresentable {
    public let cookies: [SessionCookie]
    public let formFactor: String
    public let route: String
    public let onRelogin: @MainActor () -> Void
    /// Ажлын муж бүрхүүлийн эзэмшдэг дэлгэц рүү шилжихийг хүсэх суваг. Шинэ
    /// цонх/sheet нээхгүй — ижил хүрээн доторх дэлгэц солигдоно. Танихгүй
    /// нэрэнд `false` буцаавал гүүр reject хийнэ.
    public let onOpenPane: @MainActor (String) -> Bool
    private let webOrigin: URL

    public init(cookies: [SessionCookie], formFactor: String, route: String = "/",
                onRelogin: @escaping @MainActor () -> Void,
                onOpenPane: @escaping @MainActor (String) -> Bool = { _ in false }) {
        self.cookies = cookies; self.formFactor = formFactor; self.route = route
        self.onRelogin = onRelogin; self.onOpenPane = onOpenPane
        let configured = GeregeDeviceLine.migrate(UserDefaults.standard.string(forKey: "native.settings.webEndpoint"))
        self.webOrigin = URL(string: configured.trimmingCharacters(in: CharacterSet(charactersIn: "/"))) ?? URL(string: GeregeDeviceLine.origin)!
    }

    public func makeCoordinator() -> Coordinator { Coordinator(webOrigin: webOrigin, onRelogin: onRelogin, onOpenPane: onOpenPane) }

    public func makeUIView(context: Context) -> WKWebView {
        let controller = WKUserContentController()
        controller.add(context.coordinator, name: "geregeShell")
        controller.addUserScript(WKUserScript(source: bridgeScript, injectionTime: .atDocumentStart, forMainFrameOnly: true))
        let configuration = WKWebViewConfiguration(); configuration.userContentController = controller
        let webView = WKWebView(frame: .zero, configuration: configuration)
        webView.navigationDelegate = context.coordinator
        webView.uiDelegate = context.coordinator
        let group = DispatchGroup()
        cookies.forEach { source in
            var properties: [HTTPCookiePropertyKey: Any] = [.name: source.name, .value: source.value, .domain: source.domain, .path: source.path]
            if source.secure { properties[.secure] = "TRUE" }
            guard let cookie = HTTPCookie(properties: properties) else { return }
            group.enter(); configuration.websiteDataStore.httpCookieStore.setCookie(cookie) { group.leave() }
        }
        group.notify(queue: .main) { webView.load(URLRequest(url: routeURL(route))) }
        context.coordinator.webView = webView
        return webView
    }

    public func updateUIView(_ uiView: WKWebView, context: Context) {
        guard context.coordinator.route != route else { return }
        context.coordinator.route = route
        uiView.load(URLRequest(url: routeURL(route)))
    }

    private func routeURL(_ route: String) -> URL {
        webOrigin.appending(path: route.trimmingCharacters(in: CharacterSet(charactersIn: "/")))
    }

    private var bridgeScript: String { """
    (()=>{if(window.GeregeShell)return;const p=new Map(),l=new Map();let n=0;
    window.__geregeShellResolve=(id,ok,v)=>{const x=p.get(id);if(!x)return;p.delete(id);ok?x.resolve(v):x.reject(new Error(String(v)))};
    window.__geregeShellEmit=(name,payload)=>(l.get(name)||[]).slice().forEach(fn=>fn(payload));
    window.GeregeShell=Object.freeze({version:'1.4',platform:'ios',formFactor:'\(formFactor)',capabilities:Object.freeze(['biometric','shell.pane']),
    invoke(method,params={}){return new Promise((resolve,reject)=>{const id=String(++n);p.set(id,{resolve,reject});window.webkit.messageHandlers.geregeShell.postMessage({id,method,params})})},
    on(name,h){const a=l.get(name)||[];a.push(h);l.set(name,a);return()=>{const i=a.indexOf(h);if(i>=0)a.splice(i,1)}}});document.documentElement.setAttribute('data-shell','ios')})();
    """ }

    @MainActor public final class Coordinator: NSObject, WKScriptMessageHandler, WKNavigationDelegate, WKUIDelegate {
        weak var webView: WKWebView?
        let webOrigin: URL
        let onRelogin: @MainActor () -> Void
        let onOpenPane: @MainActor (String) -> Bool
        var route = "/"
        init(webOrigin: URL, onRelogin: @escaping @MainActor () -> Void, onOpenPane: @escaping @MainActor (String) -> Bool) {
            self.webOrigin = webOrigin; self.onRelogin = onRelogin; self.onOpenPane = onOpenPane
        }

        public func userContentController(_ userContentController: WKUserContentController, didReceive message: WKScriptMessage) {
            guard message.frameInfo.isMainFrame, sameOrigin(message.frameInfo.request.url, webOrigin),
                  let body = message.body as? [String: Any], let id = body["id"] as? String, let method = body["method"] as? String else { return }
            switch method {
            case "auth.reLogin": Task { @MainActor in onRelogin(); resolve(id, true, NSNull()) }
            case "auth.lock", "biometric.authenticate": authenticate(id)
            case "menu.changed": resolve(id, true, NSNull())
            case "shell.openPane":
                let pane = (body["params"] as? [String: Any])?["pane"] as? String ?? ""
                Task { @MainActor in
                    let opened = onOpenPane(pane)
                    resolve(id, opened, opened ? NSNull() : "Unknown pane")
                }
            default: resolve(id, false, "Unsupported method: \(method)")
            }
        }

        private func authenticate(_ id: String) {
            let context = LAContext(); var error: NSError?
            guard context.canEvaluatePolicy(.deviceOwnerAuthentication, error: &error) else { resolve(id, false, error?.localizedDescription ?? "Face ID / Touch ID тохируулаагүй байна"); return }
            context.evaluatePolicy(.deviceOwnerAuthentication, localizedReason: "Gerege Nexus ажлын хэсгийг дахин нээх") { success, failure in
                Task { @MainActor in self.resolve(id, success, success ? ["authenticated": true] : (failure?.localizedDescription ?? "Баталгаажуулалт цуцлагдсан")) }
            }
        }

        public func webView(_ webView: WKWebView, decidePolicyFor action: WKNavigationAction,
                            decisionHandler: @escaping @MainActor @Sendable (WKNavigationActionPolicy) -> Void) {
            guard action.targetFrame?.isMainFrame != false, let url = action.request.url else { decisionHandler(.allow); return }
            if sameOrigin(url, webOrigin) { decisionHandler(.allow); return }
            if ["http", "https", "mailto", "tel"].contains(url.scheme?.lowercased() ?? "") { UIApplication.shared.open(url) }
            decisionHandler(.cancel)
        }

        /// `target="_blank"` болон `window.open` нь хүрээнээс тасарсан хоёр дахь
        /// webview үүсгэхийг хүсдэг. `nil` буцаах нь тэрийг үүсгэхгүй гэсэн үг —
        /// хаяг нь зөвшөөрөгдсөн scheme-тэй бол Safari дээр нээгдэнэ.
        public func webView(_ webView: WKWebView, createWebViewWith configuration: WKWebViewConfiguration,
                            for navigationAction: WKNavigationAction,
                            windowFeatures: WKWindowFeatures) -> WKWebView? {
            if let url = navigationAction.request.url,
               ["http", "https", "mailto", "tel"].contains(url.scheme?.lowercased() ?? "") {
                UIApplication.shared.open(url)
            }
            return nil
        }

        private func sameOrigin(_ lhs: URL?, _ rhs: URL?) -> Bool {
            guard let lhs, let rhs else { return false }
            return lhs.scheme?.lowercased() == rhs.scheme?.lowercased() && lhs.host?.lowercased() == rhs.host?.lowercased() && lhs.port == rhs.port
        }
        private func resolve(_ id: String, _ ok: Bool, _ value: Any) {
            guard let idData = try? JSONSerialization.data(withJSONObject: id, options: .fragmentsAllowed),
                  let valueData = try? JSONSerialization.data(withJSONObject: value, options: .fragmentsAllowed),
                  let idJSON = String(data: idData, encoding: .utf8), let valueJSON = String(data: valueData, encoding: .utf8) else { return }
            webView?.evaluateJavaScript("window.__geregeShellResolve(\(idJSON),\(ok),\(valueJSON))")
        }
    }
}
#endif
