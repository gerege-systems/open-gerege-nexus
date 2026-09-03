// Тохиргоо — design/README.md §1d-гийн бүлэглэсэн жагсаалт.
//
// Бүлэг бүр: eyebrow (13sp caps fg3) + card (surface1, хүрээ 6%, радиус 16),
// доторх мөрүүд 48dp min, 32dp икон хайрцагтай, хооронд нь 4% зураас. Хуучин
// master-detail хоёр түвшний бүтэц нэг гүйлгэдэг жагсаалт болов; талбар,
// үйлдэл бүр хэвээрээ. Top bar («Тохиргоо» + version badge) нь хүрээний
// ShellTopBar дотор — энэ дэлгэц зөвхөн агуулгаа зурна.

@file:OptIn(androidx.compose.material3.ExperimentalMaterial3Api::class)

package mn.gerege.nexus

import android.content.Context
import android.net.Uri
import android.os.Build
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.launch
import mn.gerege.nexus.core.device.DeviceEnrollmentApi
import mn.gerege.nexus.ui.brand.BrandSectionLabel
import mn.gerege.nexus.ui.brand.TonalButton
import mn.gerege.nexus.ui.icons.Lucide
import mn.gerege.nexus.ui.theme.LocalGw
import mn.gerege.nexus.ui.theme.Radius
import mn.gerege.nexus.ui.theme.Space

private class AndroidSettings(context: Context) {
    private val store = context.getSharedPreferences("native-settings-v1", Context.MODE_PRIVATE)
    // Web ба API нэг host дээр — энэ build-ийн domain шугам (DeviceLine).
    var webEndpoint by mutableStateOf(DeviceLine.migrate(store.getString("webEndpoint", null)))
    var apiEndpoint by mutableStateOf(DeviceLine.migrate(store.getString("apiEndpoint", null)))
    var printerTransport by mutableStateOf(store.getString("printerTransport", "USB")!!)
    var printerHost by mutableStateOf(store.getString("printerHost", "")!!)
    var printerPort by mutableStateOf(store.getString("printerPort", "9100")!!)
    var paperWidth by mutableStateOf(store.getString("paperWidth", "80 mm")!!)
    var scannerMode by mutableStateOf(store.getString("scannerMode", "Keyboard wedge")!!)
    var serialPort by mutableStateOf(store.getString("serialPort", "")!!)
    var baudRate by mutableStateOf(store.getString("baudRate", "9600")!!)
    var biometricLock by mutableStateOf(store.getBoolean("biometricLock", true))
    var idleMinutes by mutableStateOf(store.getString("idleMinutes", "5")!!)
    var deviceName by mutableStateOf(store.getString("deviceName", Build.MODEL)!!)
    var siteName by mutableStateOf(store.getString("siteName", "")!!)
    var dedicatedMode by mutableStateOf(store.getBoolean("dedicatedMode", BuildConfig.FORM_FACTOR == "kiosk"))
    var rebootHour by mutableStateOf(store.getString("rebootHour", "03:00")!!)
    var drawerPulse by mutableStateOf(store.getString("drawerPulse", "120")!!)
    var updateChannel by mutableStateOf(store.getString("updateChannel", "Stable")!!)
    var telemetry by mutableStateOf(store.getBoolean("telemetry", true))
    var enrollmentCode by mutableStateOf("")
    var enrolledDeviceId by mutableStateOf(store.getString("enrolledDeviceId", "")!!)
    var enrollmentStatus by mutableStateOf(if (enrolledDeviceId.isBlank()) "Бүртгэгдээгүй" else "ACTIVE")
    fun save() { store.edit().putInt("schemaVersion", 2).putString("webEndpoint", webEndpoint).putString("apiEndpoint", apiEndpoint)
        .putString("printerTransport", printerTransport).putString("printerHost", printerHost).putString("printerPort", printerPort).putString("paperWidth", paperWidth)
        .putString("scannerMode", scannerMode).putString("serialPort", serialPort).putString("baudRate", baudRate).putBoolean("biometricLock", biometricLock)
        .putString("idleMinutes", idleMinutes).putString("deviceName", deviceName).putString("siteName", siteName).putBoolean("dedicatedMode", dedicatedMode)
        .putString("rebootHour", rebootHour).putString("drawerPulse", drawerPulse).putString("updateChannel", updateChannel).putBoolean("telemetry", telemetry)
        .putString("enrolledDeviceId", enrolledDeviceId).apply() }
}

@Composable
fun NativeSettingsScreen(themeMode: String, onThemeMode: (String) -> Unit) {
    val context = LocalContext.current
    val gw = LocalGw.current
    val settings = remember { AndroidSettings(context) }
    val tokenStore = remember { DeviceTokenStore(context) }
    val deviceApi = remember(settings.apiEndpoint) {
        val root = settings.apiEndpoint.trimEnd('/')
        DeviceEnrollmentApi(if (root.endsWith("/api/v1")) "$root/" else "$root/api/v1/")
    }
    val scope = rememberCoroutineScope()
    var status by remember { mutableStateOf("") }
    val peripheral = BuildConfig.FORM_FACTOR in setOf("kiosk", "pos")
    LaunchedEffect(Unit) {
        tokenStore.load()?.let { token ->
            runCatching { deviceApi.me(token) }
                .onSuccess { settings.enrolledDeviceId = it.id; settings.deviceName = it.name; settings.enrollmentStatus = it.status }
                .onFailure { settings.enrollmentStatus = "Token хүчингүй эсвэл сервер холбогдохгүй байна" }
        }
    }
    val enrollDevice: () -> Unit = {
        scope.launch {
            status = "Төхөөрөмжийг бүртгэж байна…"
            runCatching { deviceApi.enroll(settings.enrollmentCode, settings.deviceName, "android", BuildConfig.FORM_FACTOR, settings.siteName, BuildConfig.VERSION_NAME, "Android ${Build.VERSION.RELEASE}") }
                .onSuccess { tokenStore.save(it.token); settings.enrolledDeviceId = it.id; settings.enrollmentCode = ""; settings.enrollmentStatus = "ACTIVE"; settings.save(); status = "Төхөөрөмж амжилттай бүртгэгдлээ" }
                .onFailure { status = it.message ?: "Enrollment амжилтгүй" }
        }
    }
    val rotateToken: () -> Unit = { scope.launch { val current = tokenStore.load(); if (current == null) { status = "Төхөөрөмж бүртгэгдээгүй"; return@launch }; runCatching { deviceApi.rotateToken(current) }.onSuccess { tokenStore.save(it); status = "Device token шинэчлэгдлээ" }.onFailure { status = it.message ?: "Token шинэчилсэнгүй" } } }

    Box(Modifier.fillMaxSize().background(gw.bg)) {
        Column(
            Modifier.widthIn(max = 640.dp).align(Alignment.TopCenter)
                .verticalScroll(rememberScrollState())
                .padding(horizontal = Space.lg, vertical = Space.xl),
            verticalArrangement = Arrangement.spacedBy(Space.xl),
        ) {
            SettingsGroup("ХОЛБОЛТ", listOf(
                { InfoRow(Lucide.Server, "Сервер", Uri.parse(settings.webEndpoint).host ?: settings.webEndpoint, dot = gw.credit, dotLabel = "Онлайн") },
                { FieldRow(Lucide.Server, "Web endpoint", settings.webEndpoint) { settings.webEndpoint = it } },
                { FieldRow(Lucide.Server, "API endpoint", settings.apiEndpoint) { settings.apiEndpoint = it } },
                { ActionRow(Lucide.Activity, "Холболт шалгах") { status = "Web/API холболтыг шалгаж байна…" } },
            ))
            SettingsGroup("ТӨХӨӨРӨМЖИЙН БҮРТГЭЛ", listOf<@Composable () -> Unit>(
                { FieldRow(Lucide.Smartphone, "Нэр", settings.deviceName) { settings.deviceName = it } },
                { FieldRow(Lucide.MapPin, "Байршил / site", settings.siteName) { settings.siteName = it } },
                { FieldRow(Lucide.QrCode, "Бүртгэлийн код", settings.enrollmentCode) { settings.enrollmentCode = it.uppercase() } },
                { InfoRow(Lucide.Check, "Device ID", settings.enrolledDeviceId.ifBlank { "—" }) },
                { InfoRow(Lucide.ShieldCheck, "Төлөв", settings.enrollmentStatus) },
                { ActionRow(Lucide.QrCode, "Нэг удаагийн кодоор холбох", accent = true, onClick = enrollDevice) },
            ) + if (settings.enrolledDeviceId.isNotBlank()) listOf<@Composable () -> Unit>(
                { ActionRow(Lucide.RefreshCw, "Device token шинэчлэх", onClick = rotateToken) },
            ) else emptyList())
            if (peripheral) {
                SettingsGroup("ПРИНТЕР", listOf(
                    { PillsRow(Lucide.Printer, "Холболтын төрөл", listOf("USB", "Network", "Serial"), settings.printerTransport) { settings.printerTransport = it } },
                    { FieldRow(Lucide.Server, "IP / host", settings.printerHost) { settings.printerHost = it } },
                    { FieldRow(Lucide.Server, "TCP port", settings.printerPort, KeyboardType.Number) { settings.printerPort = it } },
                    { PillsRow(Lucide.FileText, "Цаас", listOf("58 mm", "80 mm"), settings.paperWidth) { settings.paperWidth = it } },
                    { ActionRow(Lucide.Printer, "Туршилтын баримт хэвлэх") {
                        if (settings.printerTransport == "Network") scope.launch {
                            runCatching { PeripheralAdapters.printNetworkTest(settings.printerHost, settings.printerPort.toIntOrNull() ?: 9100) }
                                .onSuccess { status = "Туршилтын баримт илгээгдлээ" }
                                .onFailure { status = it.message ?: "Printer алдаа" }
                        } else status = PeripheralAdapters.usbSummary(context)
                    } },
                ))
                SettingsGroup("СКАННЕР", listOf(
                    { PillsRow(Lucide.QrCode, "Унших горим", listOf("Keyboard wedge", "Camera", "Vendor SDK"), settings.scannerMode) { settings.scannerMode = it } },
                    { ActionRow(Lucide.QrCode, "Туршилтын код унших") { status = if (settings.scannerMode == "Keyboard wedge") "Сканнерын keyboard input хүлээж байна…" else PeripheralAdapters.usbSummary(context) } },
                ))
                SettingsGroup("SERIAL ПОРТ", listOf(
                    { FieldRow(Lucide.Server, "Port", settings.serialPort) { settings.serialPort = it } },
                    { FieldRow(Lucide.Server, "Baud rate", settings.baudRate, KeyboardType.Number) { settings.baudRate = it } },
                    { ActionRow(Lucide.Activity, "USB/Serial төхөөрөмж шалгах") { status = PeripheralAdapters.usbSummary(context) } },
                ))
            }
            if (BuildConfig.FORM_FACTOR == "pos") SettingsGroup("CASH DRAWER", listOf(
                { FieldRow(Lucide.Printer, "Printer pulse (ms)", settings.drawerPulse, KeyboardType.Number) { settings.drawerPulse = it } },
                { ActionRow(Lucide.ChevronRight, "Шургуулга нээх") {
                    scope.launch {
                        runCatching { PeripheralAdapters.openDrawer(settings.printerHost, settings.printerPort.toIntOrNull() ?: 9100, settings.drawerPulse.toIntOrNull() ?: 120) }
                            .onSuccess { status = "Шургуулгын pulse илгээгдлээ" }
                            .onFailure { status = it.message ?: "Drawer алдаа" }
                    }
                } },
            ))
            if (BuildConfig.FORM_FACTOR == "kiosk") SettingsGroup("LOCKDOWN", listOf(
                { SwitchRow(Lucide.Lock, "Dedicated device mode", null, settings.dedicatedMode) { settings.dedicatedMode = it } },
                { FieldRow(Lucide.Clock, "Өдөр тутмын reboot", settings.rebootHour) { settings.rebootHour = it } },
                { ActionRow(Lucide.Lock, "Lock task mode шалгах") { status = "Device owner эрх шаардлагатай" } },
            ))
            SettingsGroup("ХАРАГДАЦ", listOf(
                { PillsRow(Lucide.Moon, "Горим", listOf("Бараан", "Цайвар", "Систем"), themeLabel(themeMode)) { onThemeMode(themeValue(it)) } },
            ))
            SettingsGroup("ХАМГААЛАЛТ", listOf(
                { SwitchRow(Lucide.Lock, "Биометрик түгжээ", "Апп нээхэд хурууны хээ шаардана", settings.biometricLock) { settings.biometricLock = it } },
                { FieldRow(Lucide.Clock, "Идэвхгүй үед түгжих (минут)", settings.idleMinutes, KeyboardType.Number) { settings.idleMinutes = it } },
            ))
            SettingsGroup("ШИНЭЧЛЭЛТ", listOf(
                { PillsRow(Lucide.RefreshCw, "Суваг", listOf("Stable", "Pilot", "Internal"), settings.updateChannel) { settings.updateChannel = it } },
                { SwitchRow(Lucide.Activity, "Оношлогооны мэдээлэл", null, settings.telemetry) { settings.telemetry = it } },
                { ActionRow(Lucide.RefreshCw, "Шинэчлэлт шалгах") { status = "${settings.updateChannel} сувгийг шалгаж байна…" } },
            ))
            SettingsGroup("ДИАГНОСТИК", listOf(
                { InfoRow(Lucide.Smartphone, "Android", "${Build.VERSION.RELEASE} / API ${Build.VERSION.SDK_INT}") },
                { InfoRow(Lucide.ShieldCheck, "Shell contract", "1.4") },
                { InfoRow(Lucide.Server, "Domain шугам", Uri.parse(settings.webEndpoint).host ?: settings.webEndpoint) },
                { InfoRow(Lucide.Monitor, "WebView", android.webkit.WebView.getCurrentWebViewPackage()?.versionName ?: "Unknown") },
                { ActionRow(Lucide.FileText, "Лог export хийх…") { status = "Log exporter device adapter-т холбогдоно" } },
            ))
            Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(Space.lg)) {
                Text(status, color = gw.fg3, fontSize = 13.sp, modifier = Modifier.weight(1f))
                Box(Modifier.width(160.dp)) {
                    TonalButton("Хадгалах", height = 48.dp, fontSize = 15.sp) { settings.save(); status = "Хадгаллаа" }
                }
            }
        }
    }
}

private fun themeLabel(mode: String) = when (mode) { "dark" -> "Бараан"; "light" -> "Цайвар"; else -> "Систем" }
private fun themeValue(label: String) = when (label) { "Бараан" -> "dark"; "Цайвар" -> "light"; else -> "system" }

/* ---------- Бүлэг ба мөрийн хэлбэрүүд (§1d) ---------- */

@Composable
private fun SettingsGroup(eyebrow: String, rows: List<@Composable () -> Unit>) {
    val gw = LocalGw.current
    Column(verticalArrangement = Arrangement.spacedBy(Space.sm)) {
        BrandSectionLabel(eyebrow)
        Surface(
            shape = RoundedCornerShape(Radius.lg),
            color = gw.surface1,
            border = BorderStroke(1.dp, gw.border),
        ) {
            Column {
                rows.forEachIndexed { index, row ->
                    if (index > 0) HorizontalDivider(color = gw.divider)
                    row()
                }
            }
        }
    }
}

@Composable
private fun IconBox(icon: ImageVector) {
    val gw = LocalGw.current
    Box(
        Modifier.size(32.dp).background(gw.surface2, RoundedCornerShape(Radius.sm)),
        contentAlignment = Alignment.Center,
    ) {
        Icon(icon, contentDescription = null, tint = gw.fg2, modifier = Modifier.size(16.dp))
    }
}

@Composable
private fun InfoRow(icon: ImageVector, label: String, value: String, dot: Color? = null, dotLabel: String? = null) {
    val gw = LocalGw.current
    Row(
        Modifier.fillMaxWidth().heightIn(min = 48.dp).padding(horizontal = Space.lg, vertical = Space.md),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(Space.md),
    ) {
        IconBox(icon)
        Text(label, color = gw.fg1, fontSize = 15.sp, modifier = Modifier.weight(1f))
        Text(value, color = gw.fg3, fontSize = 15.sp)
        if (dot != null) {
            Box(Modifier.size(8.dp).background(dot, CircleShape))
            if (dotLabel != null) Text(dotLabel, color = gw.fg2, fontSize = 13.sp, fontWeight = FontWeight.SemiBold)
        }
    }
}

@Composable
private fun FieldRow(icon: ImageVector, label: String, value: String, keyboardType: KeyboardType = KeyboardType.Text, onChange: (String) -> Unit) {
    val gw = LocalGw.current
    Row(
        Modifier.fillMaxWidth().heightIn(min = 48.dp).padding(horizontal = Space.lg, vertical = Space.md),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(Space.md),
    ) {
        IconBox(icon)
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(Space.xxs)) {
            Text(label, color = gw.fg3, fontSize = 12.sp)
            BasicTextField(
                value = value,
                onValueChange = onChange,
                textStyle = TextStyle(color = gw.fg1, fontSize = 15.sp),
                singleLine = true,
                keyboardOptions = KeyboardOptions(keyboardType = keyboardType),
                cursorBrush = SolidColor(gw.brand),
                modifier = Modifier.fillMaxWidth(),
            )
        }
    }
}

@Composable
private fun SwitchRow(icon: ImageVector, label: String, subtitle: String?, checked: Boolean, onChange: (Boolean) -> Unit) {
    val gw = LocalGw.current
    Row(
        Modifier.fillMaxWidth().heightIn(min = 48.dp).padding(horizontal = Space.lg, vertical = Space.md),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(Space.md),
    ) {
        IconBox(icon)
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(Space.xxs)) {
            Text(label, color = gw.fg1, fontSize = 15.sp)
            if (subtitle != null) Text(subtitle, color = gw.fg3, fontSize = 12.sp)
        }
        Switch(
            checked = checked,
            onCheckedChange = onChange,
            colors = SwitchDefaults.colors(
                checkedTrackColor = gw.brand,
                checkedThumbColor = gw.onBrand,
                uncheckedTrackColor = gw.surface3,
                uncheckedThumbColor = gw.fg3,
                uncheckedBorderColor = gw.borderStrong,
            ),
        )
    }
}

/** Сонголтын pill-үүд — идэвхтэй нь brand filled, бусад нь surface2 (§1d). */
@Composable
private fun PillsRow(icon: ImageVector, label: String, options: List<String>, selected: String, onSelect: (String) -> Unit) {
    val gw = LocalGw.current
    Column(
        Modifier.fillMaxWidth().padding(horizontal = Space.lg, vertical = Space.md),
        verticalArrangement = Arrangement.spacedBy(Space.md),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(Space.md)) {
            IconBox(icon)
            Text(label, color = gw.fg1, fontSize = 15.sp)
        }
        Row(Modifier.padding(start = 44.dp), horizontalArrangement = Arrangement.spacedBy(Space.sm)) {
            options.forEach { option ->
                val active = option == selected
                Surface(
                    shape = RoundedCornerShape(50),
                    color = if (active) gw.brand else gw.surface2,
                    modifier = Modifier.clickable { onSelect(option) },
                ) {
                    Text(
                        option,
                        Modifier.padding(horizontal = Space.md, vertical = Space.xs + Space.xxs),
                        color = if (active) gw.onBrand else gw.fg2,
                        fontSize = 13.sp,
                        fontWeight = FontWeight.SemiBold,
                    )
                }
            }
        }
    }
}

@Composable
private fun ActionRow(icon: ImageVector, label: String, accent: Boolean = false, onClick: () -> Unit) {
    val gw = LocalGw.current
    Row(
        Modifier.fillMaxWidth().heightIn(min = 48.dp).clickable(onClick = onClick).padding(horizontal = Space.lg, vertical = Space.md),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(Space.md),
    ) {
        IconBox(icon)
        Text(label, color = if (accent) gw.brand else gw.fg1, fontSize = 15.sp, fontWeight = if (accent) FontWeight.SemiBold else FontWeight.Normal, modifier = Modifier.weight(1f))
        Icon(Lucide.ChevronRight, contentDescription = null, tint = gw.fg4, modifier = Modifier.size(16.dp))
    }
}
