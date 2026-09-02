@file:OptIn(androidx.compose.material3.ExperimentalMaterial3Api::class)

package mn.gerege.nexus

import android.content.Context
import android.net.Uri
import android.os.Build
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.launch
import mn.gerege.nexus.core.device.DeviceEnrollmentApi

private enum class SettingsSection(val title: String, val marker: String) {
    General("Ерөнхий", "01"), Connection("Холболт", "02"), Printer("Принтер", "03"),
    Scanner("Сканнер", "04"), Serial("Serial порт", "05"), Privacy("Нууцлал", "06"),
    Device("Төхөөрөмж", "07"), Lockdown("Lockdown", "08"), Drawer("Cash drawer", "09"),
    Update("Шинэчлэлт", "10"), Diagnostics("Оношлогоо", "11")
}

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
fun NativeSettingsScreen(onClose: () -> Unit) {
    val context = LocalContext.current
    val settings = remember { AndroidSettings(context) }
    val tokenStore = remember { DeviceTokenStore(context) }
    val deviceApi = remember(settings.apiEndpoint) {
        val root = settings.apiEndpoint.trimEnd('/')
        DeviceEnrollmentApi(if (root.endsWith("/api/v1")) "$root/" else "$root/api/v1/")
    }
    val scope = rememberCoroutineScope()
    var selected by remember { mutableStateOf<SettingsSection?>(SettingsSection.General) }
    var status by remember { mutableStateOf("") }
    val sections = remember { SettingsSection.entries.filter {
        when (it) {
            SettingsSection.Device -> BuildConfig.FORM_FACTOR in setOf("kiosk", "pos")
            SettingsSection.Lockdown -> BuildConfig.FORM_FACTOR == "kiosk"
            SettingsSection.Drawer -> BuildConfig.FORM_FACTOR == "pos"
            else -> true
        }
    } }
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
    val rotateToken:()->Unit={scope.launch { val current=tokenStore.load();if(current==null){status="Төхөөрөмж бүртгэгдээгүй";return@launch};runCatching{deviceApi.rotateToken(current)}.onSuccess{tokenStore.save(it);status="Device token шинэчлэгдлээ"}.onFailure{status=it.message?:"Token шинэчилсэнгүй"} }}
    BoxWithConstraints(Modifier.fillMaxSize()) {
        val wide = maxWidth >= 700.dp
        if (wide) Row(Modifier.fillMaxSize()) {
            SettingsMenu(sections, selected, { selected = it }, onClose, Modifier.width(260.dp).fillMaxHeight())
            VerticalDivider(); SettingsDetail(selected ?: SettingsSection.General, settings, status, { status = it }, {
                settings.save(); status = "Хадгаллаа"
            }, enrollDevice, rotateToken, Modifier.weight(1f))
        } else if (selected == null) {
            SettingsMenu(sections, selected, { selected = it }, onClose, Modifier.fillMaxSize())
        } else {
            Column(Modifier.fillMaxSize()) {
                Surface(tonalElevation = 2.dp) { Row(Modifier.fillMaxWidth().padding(8.dp), verticalAlignment = Alignment.CenterVertically) {
                    TextButton({ selected = null }) { Text("‹ Ангилал") }; Text(selected!!.title, fontWeight = FontWeight.SemiBold)
                } }
                SettingsDetail(selected!!, settings, status, { status = it }, { settings.save(); status = "Хадгаллаа" }, enrollDevice, rotateToken, Modifier.weight(1f))
            }
        }
    }
}

@Composable private fun SettingsMenu(sections: List<SettingsSection>, selected: SettingsSection?, select: (SettingsSection) -> Unit, close: () -> Unit, modifier: Modifier) {
    Surface(modifier, color = MaterialTheme.colorScheme.surfaceContainer) {
        LazyColumn(contentPadding = PaddingValues(16.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
            item { Text("DEVICE CONSOLE", color = MaterialTheme.colorScheme.primary, style = MaterialTheme.typography.labelMedium, fontWeight = FontWeight.Bold); Spacer(Modifier.height(14.dp)) }
            items(sections) { section ->
                val color = if (section == selected) MaterialTheme.colorScheme.secondaryContainer else MaterialTheme.colorScheme.surfaceContainer
                Surface(color = color, shape = MaterialTheme.shapes.small, modifier = Modifier.fillMaxWidth().clickable { select(section) }) {
                    Row(Modifier.padding(12.dp)) { Text(section.marker, color = MaterialTheme.colorScheme.primary, style = MaterialTheme.typography.labelSmall); Spacer(Modifier.width(12.dp)); Text(section.title) }
                }
            }
            item { Spacer(Modifier.height(20.dp)); OutlinedButton(close, Modifier.fillMaxWidth()) { Text("Ажлын хэсэг рүү буцах") } }
        }
    }
}

@Composable private fun SettingsDetail(section: SettingsSection, s: AndroidSettings, status: String, setStatus: (String) -> Unit, save: () -> Unit, enroll: () -> Unit, rotateToken:()->Unit, modifier: Modifier) {
    val context=LocalContext.current
    val scope=rememberCoroutineScope()
    Column(modifier.verticalScroll(rememberScrollState()).padding(28.dp), verticalArrangement = Arrangement.spacedBy(14.dp)) {
        Text(section.title, style = MaterialTheme.typography.headlineMedium, fontWeight = FontWeight.SemiBold)
        when (section) {
            SettingsSection.General -> { SettingText("Form factor", BuildConfig.FORM_FACTOR, {}, enabled = false); SettingText("Хэл", "mn", {}, enabled = false) }
            SettingsSection.Connection -> { SettingText("Web endpoint", s.webEndpoint, { s.webEndpoint = it }); SettingText("API endpoint", s.apiEndpoint, { s.apiEndpoint = it }); TestButton("Холболт шалгах") { setStatus("Web/API холболтыг шалгаж байна…") } }
            SettingsSection.Printer -> { SettingOptions("Холболтын төрөл", listOf("USB", "Network", "Serial"), s.printerTransport) { s.printerTransport = it }; SettingText("IP / host", s.printerHost, { s.printerHost = it }); SettingText("TCP port", s.printerPort, { s.printerPort = it }); SettingOptions("Цаас", listOf("58 mm", "80 mm"), s.paperWidth) { s.paperWidth = it }; TestButton("Туршилтын баримт хэвлэх") { if(s.printerTransport=="Network") scope.launch { runCatching { PeripheralAdapters.printNetworkTest(s.printerHost,s.printerPort.toIntOrNull()?:9100) }.onSuccess { setStatus("Туршилтын баримт илгээгдлээ") }.onFailure { setStatus(it.message?:"Printer алдаа") } } else setStatus(PeripheralAdapters.usbSummary(context)) } }
            SettingsSection.Scanner -> { SettingOptions("Унших горим", listOf("Keyboard wedge", "Camera", "Vendor SDK"), s.scannerMode) { s.scannerMode = it }; TestButton("Туршилтын код унших") { setStatus(if(s.scannerMode=="Keyboard wedge") "Сканнерын keyboard input хүлээж байна…" else PeripheralAdapters.usbSummary(context)) } }
            SettingsSection.Serial -> { SettingText("Port", s.serialPort, { s.serialPort = it }); SettingOptions("Baud rate", listOf("9600", "19200", "38400", "57600", "115200"), s.baudRate) { s.baudRate = it }; TestButton("USB/Serial төхөөрөмж шалгах") { setStatus(PeripheralAdapters.usbSummary(context)) } }
            SettingsSection.Privacy -> { SettingSwitch("Биометрик түгжээ", s.biometricLock) { s.biometricLock = it }; SettingText("Идэвхгүй үед түгжих (минут)", s.idleMinutes, { s.idleMinutes = it }); TestButton("Android Keystore цэвэрлэх…") { setStatus("Төхөөрөмжийн баталгаажуулалт шаардлагатай") } }
            SettingsSection.Device -> { SettingText("Төхөөрөмжийн нэр", s.deviceName, { s.deviceName = it }); SettingText("Байршил / site", s.siteName, { s.siteName = it }); SettingText("Enrollment code", s.enrollmentCode, { s.enrollmentCode = it.uppercase() }); SettingText("Device ID", s.enrolledDeviceId.ifBlank { "—" }, {}, false); SettingText("Төлөв", s.enrollmentStatus, {}, false); TestButton("Нэг удаагийн кодоор бүртгэх", enroll);if(s.enrolledDeviceId.isNotBlank())TestButton("Device token шинэчлэх",rotateToken) }
            SettingsSection.Lockdown -> { SettingSwitch("Dedicated device mode", s.dedicatedMode) { s.dedicatedMode = it }; SettingText("Өдөр тутмын reboot", s.rebootHour, { s.rebootHour = it }); TestButton("Lock task mode шалгах") { setStatus("Device owner эрх шаардлагатай") } }
            SettingsSection.Drawer -> { SettingText("Printer pulse (ms)", s.drawerPulse, { s.drawerPulse = it }); TestButton("Шургуулга нээх") { scope.launch { runCatching { PeripheralAdapters.openDrawer(s.printerHost,s.printerPort.toIntOrNull()?:9100,s.drawerPulse.toIntOrNull()?:120) }.onSuccess { setStatus("Шургуулгын pulse илгээгдлээ") }.onFailure { setStatus(it.message?:"Drawer алдаа") } } } }
            SettingsSection.Update -> { SettingOptions("Суваг", listOf("Stable", "Pilot", "Internal"), s.updateChannel) { s.updateChannel = it }; SettingSwitch("Оношлогооны мэдээлэл", s.telemetry) { s.telemetry = it }; TestButton("Шинэчлэлт шалгах") { setStatus("${s.updateChannel} сувгийг шалгаж байна…") } }
            SettingsSection.Diagnostics -> { SettingText("Android", "${Build.VERSION.RELEASE} / API ${Build.VERSION.SDK_INT}", {}, false); SettingText("Shell contract", "1.4", {}, false); SettingText("Domain шугам", Uri.parse(s.webEndpoint).host ?: s.webEndpoint, {}, false); SettingText("WebView", android.webkit.WebView.getCurrentWebViewPackage()?.versionName ?: "Unknown", {}, false); TestButton("Лог export хийх…") { setStatus("Log exporter device adapter-т холбогдоно") } }
        }
        HorizontalDivider(Modifier.padding(top = 12.dp)); Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween, verticalAlignment = Alignment.CenterVertically) {
            Text(status, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.weight(1f)); Button(save) { Text("Хадгалах") }
        }
    }
}

@Composable private fun SettingText(label: String, value: String, change: (String) -> Unit, enabled: Boolean = true) { OutlinedTextField(value, change, Modifier.fillMaxWidth(), label = { Text(label) }, enabled = enabled, singleLine = true) }
@Composable private fun SettingSwitch(label: String, value: Boolean, change: (Boolean) -> Unit) { Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) { Text(label, Modifier.weight(1f)); Switch(value, change) } }
@Composable private fun SettingOptions(label: String, values: List<String>, selected: String, change: (String) -> Unit) { Column { Text(label, style = MaterialTheme.typography.labelMedium); SingleChoiceSegmentedButtonRow { values.forEachIndexed { index, value -> SegmentedButton(selected == value, { change(value) }, SegmentedButtonDefaults.itemShape(index, values.size)) { Text(value) } } } } }
@Composable private fun TestButton(label: String, action: () -> Unit) { OutlinedButton(action) { Text(label) } }
