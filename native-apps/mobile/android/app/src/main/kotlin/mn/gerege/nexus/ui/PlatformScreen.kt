package mn.gerege.nexus.ui

import android.annotation.SuppressLint
import android.content.Intent
import android.net.Uri
import android.webkit.WebResourceRequest
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.viewinterop.AndroidView
import mn.gerege.nexus.AppConfig

/**
 * Платформын ажлын муж — ширээний `PlatformView`-ийн дүйцэл.
 *
 * Хаяг нь `AppConfig.baseUrl` — Android дээр `https://mobile.nexus.gerege.mn`,
 * native дуудлагууд явдаг ЯГ ТЭР гарал. Ингэснээр WebView доторх `/api/v1`
 * дуудлага same-origin хэвээр үлдэж, `WorkAreaSession`-ий суулгасан session
 * cookie илгээгдэнэ.
 *
 * ponytail: `window.GeregeShell` inject хийхгүй — ширээний бүрхүүл ч хийдэггүй
 * тул гурван клиент дээр ажлын муж ижилхэн ажиллана. Гэрээний бүтэн гүүр
 * хэрэгтэй болбол гурвуулан дээр НЭГ дор нэмнэ.
 */
// JavaScript нь Next.js-ийн ажлын мужид ЗААВАЛ хэрэгтэй. Ачаалагдах хаяг нь
// зөвхөн энэ суулгацын өөрийн гарал — `WorkAreaWebViewClient` бусад бүх хаягийг
// гадагш гаргана.
@SuppressLint("SetJavaScriptEnabled")
@Composable
fun PlatformScreen() {
    AndroidView(
        modifier = Modifier.fillMaxSize(),
        factory = { context ->
            WebView(context).apply {
                settings.javaScriptEnabled = true
                settings.domStorageEnabled = true
                webViewClient = WorkAreaWebViewClient(AppConfig.host)
                loadUrl(AppConfig.baseUrl)
            }
        },
    )
}

/**
 * Ажлын муж дотор үлдэх, гаднах бүхнийг системд өгөх.
 *
 * Гэрээний §1a: хүрээнээс гарч болох цорын ганц зүйл нь popup. Өөр гарал руу
 * заасан холбоос нь ажлын мужийг гуравдагч этгээдийн хуудсаар СОЛИХ гэж
 * байгаа хэрэг — тэр нь session cookie-тэй хүрээн дотор болох ёсгүй.
 */
private class WorkAreaWebViewClient(private val host: String) : WebViewClient() {
    override fun shouldOverrideUrlLoading(view: WebView, request: WebResourceRequest): Boolean {
        val url = request.url ?: return false
        if (url.host?.equals(host, ignoreCase = true) == true) return false
        if (url.scheme?.lowercase() !in setOf("http", "https")) return true
        runCatching {
            view.context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(url.toString())))
        }
        return true
    }
}
