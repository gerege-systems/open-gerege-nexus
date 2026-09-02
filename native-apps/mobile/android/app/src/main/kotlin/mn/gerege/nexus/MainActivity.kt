package mn.gerege.nexus

import android.annotation.SuppressLint
import android.content.Intent
import android.content.Context
import android.graphics.Color
import android.net.Uri
import android.os.Bundle
import android.os.Build
import android.app.admin.DevicePolicyManager
import android.content.ComponentName
import android.webkit.CookieManager
import android.webkit.WebResourceRequest
import android.webkit.WebView
import android.webkit.WebViewClient
import android.view.KeyEvent
import android.view.ViewGroup
import androidx.activity.compose.BackHandler
import androidx.activity.compose.setContent
import mn.gerege.nexus.ui.brand.BrandScreen
import mn.gerege.nexus.ui.brand.BrandSectionLabel
import mn.gerege.nexus.ui.brand.BrandWordmark
import mn.gerege.nexus.ui.brand.BrandSecurityFooter
import mn.gerege.nexus.ui.brand.LoadingPrimaryButton
import mn.gerege.nexus.ui.theme.GeregeNexusTheme
import mn.gerege.nexus.ui.theme.LocalGw
import mn.gerege.nexus.ui.theme.Radius
import mn.gerege.nexus.ui.theme.Space
import androidx.compose.foundation.background
import androidx.compose.foundation.Image
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color as ComposeColor
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.fragment.app.FragmentActivity
import androidx.biometric.BiometricManager
import androidx.biometric.BiometricPrompt
import androidx.core.content.ContextCompat
import androidx.webkit.JavaScriptReplyProxy
import androidx.webkit.WebMessageCompat
import androidx.webkit.WebViewCompat
import androidx.webkit.WebViewFeature
import kotlinx.coroutines.MainScope
import kotlinx.coroutines.launch
import mn.gerege.nexus.core.auth.AuthApi
import mn.gerege.nexus.core.auth.AuthStateMachine
import mn.gerege.nexus.core.auth.LoginPhase
import mn.gerege.nexus.core.device.DeviceEnrollmentApi
import org.json.JSONObject
import com.google.zxing.BarcodeFormat
import com.google.zxing.qrcode.QRCodeWriter
import android.graphics.Bitmap

class MainActivity : FragmentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val nativeSettings = getSharedPreferences("native-settings-v1", MODE_PRIVATE)
        // Web ба API нэг host дээр — энэ build-ийн domain шугам. Ажлын мужаас
        // гарах дуудлага same-origin болж, session cookie нь SameSite=Strict
        // хэвээр ажиллана.
        val webOrigin = DeviceLine.migrate(nativeSettings.getString("webEndpoint", null)).trimEnd('/')
        val apiRoot = DeviceLine.migrate(nativeSettings.getString("apiEndpoint", null)).trimEnd('/')
        val apiBase = if (apiRoot.endsWith("/api/v1")) "$apiRoot/" else "$apiRoot/api/v1/"
        val auth = AuthStateMachine(AuthApi(apiBase), MainScope())
        DeviceTokenStore(this).load()?.let { token -> MainScope().launch { runCatching { DeviceEnrollmentApi(apiBase).telemetry(token,"shell.started") } } }
        if (BuildConfig.FORM_FACTOR == "kiosk") { val policy=getSystemService(DEVICE_POLICY_SERVICE) as DevicePolicyManager;val admin=ComponentName(this,GeregeDeviceAdminReceiver::class.java);if(policy.isDeviceOwnerApp(packageName))policy.setLockTaskPackages(admin,arrayOf(packageName));runCatching{startLockTask()} }
        val fcmToken=intent.getStringExtra("fcm_token") ?: getSharedPreferences("native-settings-v1",MODE_PRIVATE).getString("fcmToken",null)
        setContent { GeregeApp(auth, webOrigin, apiRoot, fcmToken, ::authenticateBiometric) }
    }

    private fun authenticateBiometric(done: (Boolean, String?) -> Unit) {
        val available = BiometricManager.from(this).canAuthenticate(BiometricManager.Authenticators.BIOMETRIC_STRONG or BiometricManager.Authenticators.DEVICE_CREDENTIAL)
        if (available != BiometricManager.BIOMETRIC_SUCCESS) { done(false, "Биометрик/төхөөрөмжийн түгжээ тохируулаагүй байна"); return }
        val prompt = BiometricPrompt(this, ContextCompat.getMainExecutor(this), object : BiometricPrompt.AuthenticationCallback() {
            override fun onAuthenticationSucceeded(result: BiometricPrompt.AuthenticationResult) = done(true, null)
            override fun onAuthenticationError(errorCode: Int, errString: CharSequence) = done(false, errString.toString())
        })
        prompt.authenticate(BiometricPrompt.PromptInfo.Builder().setTitle("Gerege Nexus түгжээ").setSubtitle("Ажлын хэсгийг дахин нээхийн тулд баталгаажуулна уу").setAllowedAuthenticators(BiometricManager.Authenticators.BIOMETRIC_STRONG or BiometricManager.Authenticators.DEVICE_CREDENTIAL).build())
    }
}

@Composable
private fun GeregeApp(auth: AuthStateMachine, webOrigin: String, apiOrigin: String, fcmToken:String?, biometric: ((Boolean, String?) -> Unit) -> Unit) {
    val phase by auth.phase.collectAsStateWithLifecycle()
    val context = LocalContext.current
    val deviceToken = remember { DeviceTokenStore(context).load() }
    var authenticated by remember { mutableStateOf(BuildConfig.FORM_FACTOR == "kiosk" && deviceToken != null) }
    LaunchedEffect(phase) { if (phase is LoginPhase.Success) { authenticated = true; if(fcmToken!=null)auth.registerPushToken(fcmToken,context.packageName) } }
    GeregeNexusTheme {
        if (authenticated) WorkArea(auth, webOrigin, apiOrigin, deviceToken, biometric) { authenticated = false; auth.cancel() }
        else NativeLogin(auth, deviceToken)
    }
}

@Composable
private fun NativeLogin(auth: AuthStateMachine, deviceToken: String?) {
    val phase by auth.phase.collectAsStateWithLifecycle()
    var email by remember { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    var nationalId by remember { mutableStateOf("") }
    var staffPin by remember { mutableStateOf("") }
    val context = LocalContext.current
    val waiting = phase as? LoginPhase.Waiting
    LaunchedEffect(waiting?.deviceLinkUrl) {
        if(BuildConfig.FORM_FACTOR in setOf("mobile","tablet")) waiting?.deviceLinkUrl?.let { runCatching { context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(it))) } }
    }
    val gw = LocalGw.current
    BrandScreen {
      Box(Modifier.fillMaxSize().padding(Space.xl), contentAlignment = Alignment.Center) {
        Column(Modifier.widthIn(max = if (BuildConfig.FORM_FACTOR == "tablet") 520.dp else 420.dp), verticalArrangement = Arrangement.spacedBy(Space.md)) {
            BrandWordmark()
            BrandSectionLabel("GEREGE / NEXUS")
            Text("Таны баталгаатай\nажлын орчин", color = gw.fg1, fontSize = 34.sp, lineHeight = 38.sp, fontWeight = FontWeight.Bold)
            Spacer(Modifier.height(Space.md))
            if(BuildConfig.FORM_FACTOR != "kiosk") { OutlinedTextField(email, { email = it }, Modifier.fillMaxWidth(), label = { Text(stringResource(R.string.auth_field_email)) }, singleLine = true, keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Email));OutlinedTextField(password, { password = it }, Modifier.fillMaxWidth(), label = { Text(stringResource(R.string.auth_field_password)) }, singleLine = true, visualTransformation = PasswordVisualTransformation());LoadingPrimaryButton(label = stringResource(R.string.auth_action_admin_sign_in), isLoading = phase is LoginPhase.Starting, isEnabled = phase !is LoginPhase.Starting && phase !is LoginPhase.Waiting, onClick = { auth.password(email, password) });HorizontalDivider(Modifier.padding(vertical = Space.sm), color = gw.divider) }
            OutlinedTextField(nationalId, { nationalId = it.uppercase() }, Modifier.fillMaxWidth(), label = { Text(stringResource(R.string.auth_eid_reg_number)) }, singleLine = true)
            LoadingPrimaryButton(label = stringResource(R.string.auth_eid_send_request), isLoading = phase is LoginPhase.Starting, isEnabled = phase !is LoginPhase.Starting && phase !is LoginPhase.Waiting, onClick = { auth.push(nationalId) })
            if(BuildConfig.FORM_FACTOR in setOf("mobile","tablet")) OutlinedButton({ auth.appToApp("https://nexus.gerege.mn/auth/eid/callback") }, Modifier.fillMaxWidth(), enabled = phase !is LoginPhase.Starting && phase !is LoginPhase.Waiting) { Text(stringResource(R.string.auth_action_app_to_app)) }
            if(BuildConfig.FORM_FACTOR=="kiosk") { OutlinedButton({ auth.appToApp("") },Modifier.fillMaxWidth(),enabled=phase !is LoginPhase.Starting&&phase !is LoginPhase.Waiting){Text("eID QR үүсгэх")};waiting?.deviceLinkUrl?.let{Image(qrBitmap(it).asImageBitmap(),"eID QR",Modifier.size(220.dp).align(Alignment.CenterHorizontally))} }
            if (BuildConfig.FORM_FACTOR in setOf("pos", "tablet") && deviceToken != null) {
                HorizontalDivider(Modifier.padding(vertical = Space.sm), color = gw.divider); BrandSectionLabel("АЖИЛТНЫ ХУРДАН НЭВТРЭЛТ")
                OutlinedTextField(staffPin, { staffPin = it.filter(Char::isDigit).take(12) }, Modifier.fillMaxWidth(), label = { Text("PIN") }, singleLine = true, visualTransformation = PasswordVisualTransformation(), keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.NumberPassword))
                Button({ auth.staffPin(staffPin, deviceToken) }, Modifier.fillMaxWidth(), enabled = staffPin.length >= 4 && phase !is LoginPhase.Starting) { Text(stringResource(R.string.auth_action_staff_sign_in)) }
            }
            StatusBlock(phase)
            if (phase is LoginPhase.Starting || phase is LoginPhase.Waiting) TextButton(auth::cancel, Modifier.align(Alignment.End)) { Text(stringResource(R.string.auth_action_cancel)) }
            BrandSecurityFooter("Нууц үг дамжуулахгүй · Баталгаажуулалт eID апп дотор")
        }
      }
    }
}

private fun qrBitmap(value:String,size:Int=440):Bitmap{val matrix=QRCodeWriter().encode(value,BarcodeFormat.QR_CODE,size,size);return Bitmap.createBitmap(size,size,Bitmap.Config.RGB_565).apply{for(y in 0 until size)for(x in 0 until size)setPixel(x,y,if(matrix[x,y])Color.BLACK else Color.WHITE)}}

@Composable private fun StatusBlock(phase: LoginPhase) {
    val text = when (phase) {
        LoginPhase.Idle -> ""
        LoginPhase.Starting -> "Хүсэлт эхлүүлж байна…"
        is LoginPhase.Waiting -> "eID апп дээрх кодтой тулгана уу  ·  ${phase.verificationCode}"
        LoginPhase.Success -> "Амжилттай нэвтэрлээ"
        LoginPhase.Expired -> "Хугацаа дууслаа. Шинэ хүсэлт эхлүүлнэ үү."
        LoginPhase.Refused -> "Хүсэлтээс татгалзсан байна."
        is LoginPhase.Error -> phase.message
    }
    val gw = LocalGw.current
    if (text.isNotEmpty()) Surface(color = gw.surface2, shape = RoundedCornerShape(Radius.md)) {
        Text(text, Modifier.fillMaxWidth().padding(Space.lg), color = gw.fg1)
    }
}

private enum class Pane { Work, Settings }

/**
 * Аппын цорын ганц хүрээ.
 *
 * Дүрэм: **popup-аас бусад бүх зүйл энэ хүрээн дотор**. Ажлын муж ба тохиргоо
 * хоёр нь ижил top bar болон bottom nav-ын дунд солигддог дэлгэцүүд — тусдаа
 * Activity эсвэл Dialog нээгдэхгүй. Тусдаа гарах эрхтэй зүйл бол зөвхөн
 * `BiometricPrompt`, системийн зөвшөөрлийн харилцах цонх зэрэг popup-ууд.
 *
 * WebView нь `remember`-т хадгалагдана. Энэ нь чимэг биш: өмнө нь тохиргоо
 * гарахад бүх хүрээ солигдож, `AndroidView` композициос гарахад WebView устаж
 * байсан — буцаж ирэхэд ажлын муж эхнээсээ ачаалагдаж, хэрэглэгчийн байсан
 * хуудас, гүйлгэсэн байрлал, бөглөж байсан маягт бүгд алга болно.
 */
@SuppressLint("SetJavaScriptEnabled")
@Composable private fun WorkArea(auth: AuthStateMachine, webOrigin: String, apiOrigin: String, deviceToken: String?, biometric: ((Boolean, String?) -> Unit) -> Unit, relogin: () -> Unit) {
    var pane by remember { mutableStateOf(Pane.Work) }
    // Шугамын нүүр. Тэнд тухайн платформын өөрийн эхлэх дэлгэц
    // рендерлэгдэнэ (frontend/app/line/<platform>).
    val startRoute = "/"
    var activeRoute by remember { mutableStateOf(startRoute) }
    val context = LocalContext.current
    val openPane: (String) -> Boolean = { requested ->
        when (requested) {
            "settings" -> { pane = Pane.Settings; true }
            "work" -> { pane = Pane.Work; true }
            else -> false
        }
    }
    val nativeWebView = remember {
        WebView(context).apply {
            setBackgroundColor(Color.rgb(11, 15, 23)); settings.javaScriptEnabled = true; settings.domStorageEnabled = true
            CookieManager.getInstance().setAcceptCookie(true)
            auth.sessionCookies().forEach { cookie ->
                val flags = buildString { append("${cookie.name}=${cookie.value}; Path=${cookie.path}; SameSite=Lax"); if (cookie.secure) append("; Secure"); if (cookie.httpOnly) append("; HttpOnly") }
                CookieManager.getInstance().setCookie(webOrigin, flags)
            }
            if (deviceToken != null) CookieManager.getInstance().setCookie(apiOrigin, "device_token=$deviceToken; Path=/api/v1/devices; HttpOnly; SameSite=Strict")
            CookieManager.getInstance().flush()
            installBridge(this, webOrigin, biometric, relogin, openPane)
            webViewClient = object : WebViewClient() {
                override fun shouldOverrideUrlLoading(view: WebView, request: WebResourceRequest): Boolean {
                    if (request.isForMainFrame && sameOrigin(request.url, Uri.parse(webOrigin))) return false
                    if (request.isForMainFrame && request.url.scheme in setOf("http", "https", "mailto", "tel")) context.startActivity(Intent(Intent.ACTION_VIEW, request.url))
                    return true
                }
            }
            val scanBuffer=StringBuilder();setOnKeyListener { _,code,event -> if(event.action!=KeyEvent.ACTION_DOWN)return@setOnKeyListener false;if(code==KeyEvent.KEYCODE_ENTER&&scanBuffer.isNotEmpty()){val payload=JSONObject().put("value",scanBuffer.toString()).put("format","keyboard-wedge");evaluateJavascript("window.__geregeShellEmit&&window.__geregeShellEmit('shell:scan',${payload})",null);scanBuffer.clear();true}else{event.unicodeChar.takeIf{it>0}?.let{scanBuffer.append(it.toChar())};false} }
            loadUrl("$webOrigin$startRoute")
        }
    }
    // `remember` нь WebView-г амьд байлгадаг ч сүүлд нь чөлөөлдөггүй. Хүрээ
    // өөрөө композициос гарахад (жишээ нь гарах үед) native талын нөөцийг
    // буцаана.
    DisposableEffect(Unit) { onDispose { (nativeWebView.parent as? ViewGroup)?.removeView(nativeWebView); nativeWebView.destroy() } }
    // Тохиргоо нээлттэй үед back нь ажлын муж руу буцаана — тусдаа Activity
    // байхгүй тул системийн back өөрөө үүнийг мэдэхгүй. Ажлын муж дээр back нь
    // өмнөх шигээ системд үлдэнэ.
    BackHandler(enabled = pane != Pane.Work) { pane = Pane.Work }
    Column(Modifier.fillMaxSize()) {
      Surface(tonalElevation = 3.dp) { Row(Modifier.fillMaxWidth().height(48.dp).padding(horizontal = 12.dp), verticalAlignment = Alignment.CenterVertically) {
        Text("Gerege Nexus", fontWeight = FontWeight.SemiBold, modifier = Modifier.weight(1f))
        TextButton({ pane = if (pane == Pane.Settings) Pane.Work else Pane.Settings }) {
            Text(if (pane == Pane.Settings) "Ажлын муж" else "Төхөөрөмжийн тохиргоо")
        }
      } }
      Box(Modifier.weight(1f).fillMaxWidth()) {
        when (pane) {
          // Тохиргооноос буцаж ирэхэд AndroidView шинэ holder үүсгээд ижил
          // WebView-г дахин нэмнэ. Хуучин holder-оос салгаагүй бол Android
          // "The specified child already has a parent" гэж унана.
          Pane.Work -> AndroidView(
            factory = { (nativeWebView.parent as? ViewGroup)?.removeView(nativeWebView); nativeWebView },
            modifier = Modifier.fillMaxSize(),
          )
          Pane.Settings -> NativeSettingsScreen { pane = Pane.Work }
        }
      }
      // Bottom nav нь дэлгэц солигдох бүрд байрандаа үлдэнэ — энэ бол хүрээний
      // chrome, ажлын мужийн хэсэг биш.
      if (BuildConfig.FORM_FACTOR in setOf("mobile", "tablet")) NavigationBar(containerColor = LocalGw.current.surface1) {
        listOf("▦" to ("Аппууд" to "/apps"), "▤" to ("Баримт" to "/documents"), "▥" to ("Тайлан" to "/reports")).forEach { (icon, item) ->
          NavigationBarItem(selected = pane == Pane.Work && activeRoute == item.second, onClick = { activeRoute = item.second; pane = Pane.Work; nativeWebView.loadUrl(webOrigin + item.second) }, icon = { Text(icon, fontSize = 20.sp) }, label = { Text(item.first) })
        }
        NavigationBarItem(selected = pane == Pane.Settings, onClick = { pane = Pane.Settings }, icon = { Text("⚙", fontSize = 19.sp) }, label = { Text("Тохиргоо") })
      }
    }
}

private fun installBridge(webView: WebView, webOrigin: String, biometric: ((Boolean, String?) -> Unit) -> Unit, relogin: () -> Unit, openPane: (String) -> Boolean) {
    val capabilities = when(BuildConfig.FORM_FACTOR){"kiosk"->listOf("escpos","scanner","device.identity","kiosk.lockdown","telemetry","shell.pane");"pos"->listOf("escpos","scanner","device.identity","secure-store","telemetry","biometric","shell.pane");else->listOf("secure-store","biometric","telemetry","shell.pane")}
    val script = """(()=>{if(window.GeregeShell)return;const p=new Map(),l=new Map();let n=0;window.__geregeShellResolve=(id,ok,v)=>{const x=p.get(id);if(!x)return;p.delete(id);ok?x.resolve(v):x.reject(new Error(String(v)))};window.__geregeShellEmit=(name,payload)=>(l.get(name)||[]).slice().forEach(fn=>fn(payload));window.GeregeShell=Object.freeze({version:'1.4',platform:'android',formFactor:'${BuildConfig.FORM_FACTOR}',capabilities:Object.freeze(${org.json.JSONArray(capabilities)}),invoke(method,params={}){return new Promise((resolve,reject)=>{const id=String(++n);p.set(id,{resolve,reject});window.geregeNative.postMessage(JSON.stringify({id,method,params}))})},on(name,h){const a=l.get(name)||[];a.push(h);l.set(name,a);return()=>{const i=a.indexOf(h);if(i>=0)a.splice(i,1)}}});document.documentElement.setAttribute('data-shell','android')})();"""
    if (WebViewFeature.isFeatureSupported(WebViewFeature.DOCUMENT_START_SCRIPT)) WebViewCompat.addDocumentStartJavaScript(webView, script, setOf(webOrigin))
    WebViewCompat.addWebMessageListener(webView, "geregeNative", setOf(webOrigin), object : WebViewCompat.WebMessageListener {
        override fun onPostMessage(view: WebView, message: WebMessageCompat, sourceOrigin: Uri, isMainFrame: Boolean, replyProxy: JavaScriptReplyProxy) {
            if (!isMainFrame || !sameOrigin(sourceOrigin, Uri.parse(webOrigin))) return
            val body = runCatching { JSONObject(message.data ?: "") }.getOrNull() ?: return
            val id = body.optString("id"); val method = body.optString("method")
            when (method) {
                "auth.reLogin" -> { relogin(); resolve(view, id, true, null) }
                "auth.lock", "biometric.authenticate" -> biometric { ok, error -> resolve(view, id, ok, if (ok) JSONObject().put("authenticated",true) else error) }
                "device.identity" -> { val prefs=view.context.getSharedPreferences("native-settings-v1",Context.MODE_PRIVATE);resolve(view,id,true,JSONObject().put("id",prefs.getString("enrolledDeviceId","")).put("name",prefs.getString("deviceName",Build.MODEL)).put("site",prefs.getString("siteName","")).put("platform","android").put("form_factor",BuildConfig.FORM_FACTOR)) }
                "escpos.print" -> { val params=body.optJSONObject("params")?:JSONObject();val prefs=view.context.getSharedPreferences("native-settings-v1",Context.MODE_PRIVATE);val host=prefs.getString("printerHost","")!!;val port=prefs.getString("printerPort","9100")!!.toIntOrNull()?:9100;val bytes=if(params.has("base64"))android.util.Base64.decode(params.optString("base64"),android.util.Base64.DEFAULT)else params.optString("text").toByteArray();MainScope().launch { runCatching { PeripheralAdapters.printRaw(host,port,bytes,params.optBoolean("cut",true)) }.onSuccess { resolve(view,id,true,null) }.onFailure { resolve(view,id,false,it.message) } } }
                "escpos.drawer" -> { val prefs=view.context.getSharedPreferences("native-settings-v1",Context.MODE_PRIVATE);MainScope().launch { runCatching { PeripheralAdapters.openDrawer(prefs.getString("printerHost","")!!,prefs.getString("printerPort","9100")!!.toIntOrNull()?:9100,prefs.getString("drawerPulse","120")!!.toIntOrNull()?:120) }.onSuccess { resolve(view,id,true,null) }.onFailure { resolve(view,id,false,it.message) } } }
                "kiosk.lockdown" -> { val enabled=body.optJSONObject("params")?.optBoolean("enabled",true)?:true;val activity=view.context as? android.app.Activity;runCatching { if(enabled)activity?.startLockTask() else activity?.stopLockTask() }.onSuccess { resolve(view,id,true,JSONObject().put("enabled",enabled)) }.onFailure { resolve(view,id,false,it.message) } }
                "scanner.start", "scanner.stop" -> resolve(view,id,true,null)
                "menu.changed" -> resolve(view, id, true, null)
                // Ажлын муж бүрхүүлийн эзэмшдэг дэлгэц рүү шилжихийг хүсэж
                // байна. Шинэ Activity нээхгүй — ижил хүрээн доторх дэлгэц
                // солигдоно.
                "shell.openPane" -> { val opened = openPane(body.optJSONObject("params")?.optString("pane") ?: ""); resolve(view, id, opened, if (opened) null else "Unknown pane") }
                else -> resolve(view, id, false, "Unsupported method: $method")
            }
        }
    })
}

private fun sameOrigin(left: Uri, right: Uri): Boolean = left.scheme.equals(right.scheme, true) && left.host.equals(right.host, true) && left.port == right.port

private fun resolve(view: WebView, id: String, ok: Boolean, value: Any?) {
    val encoded = when(value){null->"null";is JSONObject->value.toString();is Number,is Boolean->value.toString();else->JSONObject.quote(value.toString())}
    view.post { view.evaluateJavascript("window.__geregeShellResolve(${JSONObject.quote(id)},$ok,$encoded)", null) }
}
