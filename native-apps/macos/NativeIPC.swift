//
//  NativeIPC.swift
//  GeregeNexusNativeMac
//
//  Created for Open Gerege Nexus Desktop Platform
//  Pure Native Swift + AppKit + WKWebView IPC Bridge
//

import Foundation
import WebKit
import AppKit
import LocalAuthentication

public class NativeIPCBridge: NSObject, WKScriptMessageHandler {
    private weak var webView: WKWebView?
    private weak var windowController: MainWindowController?

    public init(webView: WKWebView, windowController: MainWindowController) {
        self.webView = webView
        self.windowController = windowController
        super.init()
    }

    public func userContentController(_ userContentController: WKUserContentController, didReceive message: WKScriptMessage) {
        guard message.name == "geregeShell" else { return }

        guard message.frameInfo.isMainFrame,
              sameOrigin(message.frameInfo.request.url, webView?.url),
              let json = message.body as? [String: Any] else { return }
        handleCommand(json)
    }

    private func sameOrigin(_ lhs: URL?, _ rhs: URL?) -> Bool {
        guard let lhs, let rhs else { return false }
        return lhs.scheme?.lowercased() == rhs.scheme?.lowercased()
            && lhs.host?.lowercased() == rhs.host?.lowercased()
            && lhs.port == rhs.port
    }

    private func handleCommand(_ json: [String: Any]) {
        guard let command = json["method"] as? String else { return }
        let requestId = json["id"] as? String ?? ""
        let params = json["params"] as? [String: Any] ?? [:]

        print("[NativeIPC Bridge] Handling command: \(command), requestId: \(requestId)")

        switch command {
        case "external.open":
            guard let raw = params["url"] as? String, let url = URL(string: raw),
                  ["http", "https", "mailto", "tel"].contains(url.scheme?.lowercased() ?? "") else {
                resolve(requestId, ok: false, value: "URL scheme not allowed"); return
            }
            NSWorkspace.shared.open(url); resolve(requestId, ok: true, value: NSNull())
        case "print.system":
            webView?.evaluateJavaScript("window.print()") { _, error in
                self.resolve(requestId, ok: error == nil, value: error?.localizedDescription ?? NSNull())
            }
        case "menu.changed":
            resolve(requestId, ok: true, value: NSNull())
        case "shell.openPane":
            // Ажлын муж бүрхүүлийн эзэмшдэг дэлгэц рүү шилжихийг хүсэж байна.
            // Шинэ цонх нээхгүй — ижил хүрээн доторх дэлгэц солигдоно.
            switch params["pane"] as? String {
            case "settings":
                DispatchQueue.main.async { self.windowController?.showSettings() }
                resolve(requestId, ok: true, value: NSNull())
            case "work":
                DispatchQueue.main.async { self.windowController?.showWorkArea() }
                resolve(requestId, ok: true, value: NSNull())
            default:
                resolve(requestId, ok: false, value: "Unknown pane")
            }
        case "auth.reLogin":
            windowController?.showNativeLogin()
            resolve(requestId, ok: true, value: NSNull())
        case "auth.lock":
            authenticate(requestId)
        case "biometric.authenticate":
            authenticate(requestId)
        case "device.identity":
            let settings = NativeSettings.load()
            resolve(requestId, ok: true, value: ["id":settings.deviceID,"name":settings.deviceName,"site":settings.site,"platform":"macos","form_factor":"desktop"])
        default:
            resolve(requestId, ok: false, value: "Unsupported method: \(command)")
        }
    }

    private func authenticate(_ requestId: String) {
        let context = LAContext(); var error: NSError?
        guard context.canEvaluatePolicy(.deviceOwnerAuthentication, error: &error) else { resolve(requestId, ok: false, value: error?.localizedDescription ?? "Touch ID тохируулаагүй байна"); return }
        context.evaluatePolicy(.deviceOwnerAuthentication, localizedReason: "Gerege Nexus ажлын хэсгийг дахин нээх") { success, failure in
            self.resolve(requestId, ok: success, value: success ? ["authenticated": true] : (failure?.localizedDescription ?? "Баталгаажуулалт цуцлагдсан"))
        }
    }

    private func resolve(_ id: String, ok: Bool, value: Any) {
        guard let idData = try? JSONSerialization.data(withJSONObject: id, options: .fragmentsAllowed),
              let valueData = try? JSONSerialization.data(withJSONObject: value, options: .fragmentsAllowed),
              let idJSON = String(data: idData, encoding: .utf8),
              let valueJSON = String(data: valueData, encoding: .utf8) else { return }
        let js = "window.__geregeShellResolve(\(idJSON),\(ok ? "true" : "false"),\(valueJSON));"
        DispatchQueue.main.async {
            self.webView?.evaluateJavaScript(js, completionHandler: nil)
        }
    }
}
