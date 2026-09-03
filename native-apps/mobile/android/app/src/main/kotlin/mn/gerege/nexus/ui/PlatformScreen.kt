package mn.gerege.nexus.ui

import android.annotation.SuppressLint
import android.content.Intent
import android.net.Uri
import android.webkit.WebResourceRequest
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.activity.compose.BackHandler
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.width
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import mn.gerege.nexus.AppConfig
import mn.gerege.nexus.R
import mn.gerege.nexus.ui.theme.LocalGw

/**
 * Платформын ажлын муж — iOS-ийн `MobilePlatformPage`-ийн дүйцэл.
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
    val gw = LocalGw.current
    // Буцах нь БҮРХҮҮЛИЙНХ (гэрээний §1) — вэб хуудас өөрийн буцахыг зурахгүй.
    // Товч нь зөвхөн буцах түүх байх үед гарна: шугамын нүүрэн дээр энэ мөр
    // огт эзлэхгүй, идэвхгүй саарал товч тэнд худал зүйл хэлнэ.
    var web by remember { mutableStateOf<WebView?>(null) }
    var canGoBack by remember { mutableStateOf(false) }

    // Системийн буцах дохио мөн ажлын мужид үйлчилнэ — эс бөгөөс хүн ажлын
    // мужийн гүнд орчихоод аппаас бүхэлд нь гарна.
    BackHandler(enabled = canGoBack) { web?.goBack() }

    Column(Modifier.fillMaxSize()) {
        if (canGoBack) {
            Surface(color = gw.surface1) {
                TextButton(onClick = { web?.goBack() }) {
                    Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = null)
                    Spacer(Modifier.width(6.dp))
                    Text(stringResource(R.string.Nav_Platform))
                }
            }
            HorizontalDivider(color = gw.border)
        }

        AndroidView(
            modifier = Modifier.fillMaxSize(),
            factory = { context ->
                WebView(context).apply {
                    settings.javaScriptEnabled = true
                    settings.domStorageEnabled = true
                    webViewClient = WorkAreaWebViewClient(AppConfig.host) { canGoBack = it }
                    loadUrl(AppConfig.baseUrl)
                    web = this
                }
            },
        )
    }
}

/**
 * Ажлын муж дотор үлдэх, гаднах бүхнийг системд өгөх.
 *
 * Гэрээний §1a: хүрээнээс гарч болох цорын ганц зүйл нь popup. Өөр гарал руу
 * заасан холбоос нь ажлын мужийг гуравдагч этгээдийн хуудсаар СОЛИХ гэж
 * байгаа хэрэг — тэр нь session cookie-тэй хүрээн дотор болох ёсгүй.
 */
private class WorkAreaWebViewClient(
    private val host: String,
    private val onHistoryChanged: (Boolean) -> Unit,
) : WebViewClient() {
    override fun doUpdateVisitedHistory(view: WebView, url: String?, isReload: Boolean) {
        onHistoryChanged(view.canGoBack())
    }


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
