import SwiftUI
import WebKit

/// Платформын ажлын муж — гар дээрх `PlatformView`.
///
/// Ширээн дээр энэ нь sidebar-ын сүүлчийн мөр, энд табын зурвасын сүүлчийн
/// таб. Хоёулаа НЭГ дүрэмтэй (`docs/SHELL_CONTRACT.md` §1a): апп нэг хүрээтэй
/// тул ажлын муж нь хоёр дахь цонх биш, нөгөө дөрөвтэйгээ ижил байдлаар
/// солигддог дэлгэц.
///
/// Хаяг нь `AppConfig.baseURL` — iOS дээр `https://mobile.nexus.gerege.mn`,
/// native дуудлагууд явдаг ЯГ ТЭР гарал. Ингэснээр webview доторх `/api/v1`
/// дуудлага same-origin хэвээр үлдэж, `WorkAreaSession`-ий суулгасан
/// session cookie илгээгдэнэ. Өөр гарал ашигласан бол тэр cookie хэзээ ч
/// явахгүй байх байв.
///
/// ponytail: `window.GeregeShell` inject хийхгүй — ширээний бүрхүүл ч мөн
/// хийдэггүй тул ажлын муж хоёр клиент дээр ижилхэн «хөтчийн горим»-оор
/// ажиллана. Гэрээний бүтэн гүүр хэрэгтэй болбол хоёуланд нь НЭГ дор нэмнэ,
/// эс бөгөөс хоёр бүрхүүл өөр өөр гэрээтэй болно.
struct MobilePlatformPage: View {
    @EnvironmentObject private var appState: AppState
    @ObservedObject private var loc = LocalizationService.shared

    var body: some View {
        // `URL(string:)!` БИШ: суурь хаяг нь Тохиргооноос дарагддаг хэрэглэгчийн
        // утга. Буруу бичсэн хаяг аппыг унагах ёсгүй, уншигдах мессеж өгөх ёстой.
        if let url = URL(string: AppConfig.baseURL), url.scheme?.hasPrefix("http") == true {
            // Safe area-г ҮЛ ТООХГҮЙ: доор нь табын зурвас байгаа тул
            // хуудасны сүүлийн мөрүүд түүний ард орж, хүрч болохгүй болно.
            WorkAreaWebView(url: url)
        } else {
            MobilePage(title: loc.t("Nav_Platform"), subtitle: nil) {
                VStack(alignment: .leading, spacing: Theme.Space.sm) {
                    Image(systemName: "exclamationmark.triangle")
                        .font(.system(size: 24))
                        .foregroundStyle(Theme.accent)
                    Text(loc.pick("Серверийн хаяг буруу байна.",
                                  "The server address is not valid.",
                                  "Неверный адрес сервера.",
                                  "服务器地址无效。"))
                        .font(Theme.TypeScale.body)
                        .foregroundStyle(Theme.fg1)
                    Text(AppConfig.baseURL)
                        .font(Theme.TypeScale.monoSm)
                        .foregroundStyle(Theme.fg3)
                        .textSelection(.enabled)
                }
            }
        }
    }
}

private struct WorkAreaWebView: UIViewRepresentable {
    let url: URL

    func makeCoordinator() -> WorkAreaWebViewCoordinator { WorkAreaWebViewCoordinator() }

    func makeUIView(context: Context) -> WKWebView {
        let config = WKWebViewConfiguration()
        // Ажлын муж тусдаа цонх нээхийг оролдвол систем браузарт очно — §1a:
        // хүрээнээс гарч болох цорын ганц зүйл нь popup, вэб хуудас биш.
        config.preferences.javaScriptCanOpenWindowsAutomatically = false
        // ЗААВАЛ `.default()`: `WorkAreaSession` нь session cookie-г яг тэр
        // санд суулгадаг. Non-persistent сан ашигласан бол нэвтрэлт webview
        // рүү хэзээ ч хүрэхгүй.
        config.websiteDataStore = .default()

        let webView = WKWebView(frame: .zero, configuration: config)
        webView.navigationDelegate = context.coordinator
        webView.uiDelegate = context.coordinator
        webView.allowsBackForwardNavigationGestures = true
        webView.load(URLRequest(url: url))
        return webView
    }

    func updateUIView(_ webView: WKWebView, context: Context) {}
}

/// Навигаци ба процессын төгсгөл — ширээнийхтэй ижил дүрмүүд.
final class WorkAreaWebViewCoordinator: NSObject, WKNavigationDelegate, WKUIDelegate {
    private var reloadsAfterTermination = 0

    /// WebKit-ийн контентын процесс үхэхэд webview нь ХООСОН цагаан хэвээр
    /// үлддэг — өөрөө сэргэдэггүй. Гар утсан дээр энэ нь ховор биш: систем
    /// далд байгаа табын процессыг санах ойн дарамтад устгадаг.
    ///
    /// Гурван удаагийн дараа зогсоно: тогтмол унаж байгаа процессыг дахин
    /// ачаалах бүр дахин унагана.
    func webViewWebContentProcessDidTerminate(_ webView: WKWebView) {
        guard reloadsAfterTermination < 3 else { return }
        reloadsAfterTermination += 1
        webView.reload()
    }

    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
        // Амжилттай ачаалал тоолуурыг тэглэнэ — «гурван удаа» гэдэг нь
        // дараалсан бүтэлгүйтэл байх ёстой, аппын насан туршийн нийлбэр биш.
        reloadsAfterTermination = 0
    }

    /// `target="_blank"` ба `window.open` нь хоёр дахь webview нээхгүй —
    /// хаягийг систем браузарт өгнө (§1a).
    func webView(_ webView: WKWebView, createWebViewWith configuration: WKWebViewConfiguration,
                 for navigationAction: WKNavigationAction,
                 windowFeatures: WKWindowFeatures) -> WKWebView? {
        if let url = navigationAction.request.url, ["http", "https"].contains(url.scheme?.lowercased() ?? "") {
            UIApplication.shared.open(url)
        }
        return nil
    }
}
