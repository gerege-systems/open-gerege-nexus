// Нэвтрэх дэлгэц — design/README.md §1a, §1b, §1e-гийн Compose буулгалт.
//
// Шатлалын зарчим (§1a): eID зам өргөгдсөн card дотор, «ҮНДСЭН» тэмдэгтэй,
// цэнхэр filled товчтой; админ нэвтрэлт card-гүй, outline товчтой хоёрдогч
// бүс. Цэнхэр зөвхөн гурван газар — бамбай икон, filled товч, tonal товч.
//
// Төлөвүүд (§1b) дэлгэц солихгүй: eID card доторх агуулга л AnimatedContent-
// ээр (fade 200ms) солигдоно. Countdown-ы эх сурвалж нь LoginPhase.Waiting-ийн
// expiresAtMillis — watch() үргэлж утга өгдөг.
//
// Алдааны төлөвт card хэрэглэгчийн хэлээр бичсэн мессежийг л харуулна
// (AuthStateMachine.describeFailure); Java-гийн exception бичвэр ба жинхэнэ
// endpoint нь «Диагностик» дор нуугдана. Интернэтгүй үед (Connectivity.kt)
// card-ын дээр анхааруулга гарна — хүсэлт илгээхээс өмнө шалтгааныг хэлнэ.

package mn.gerege.nexus

import android.content.Intent
import android.graphics.Bitmap
import android.net.Uri
import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.FastOutSlowInEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color as ComposeColor
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.google.zxing.BarcodeFormat
import com.google.zxing.qrcode.QRCodeWriter
import kotlinx.coroutines.delay
import mn.gerege.nexus.core.auth.AuthStateMachine
import mn.gerege.nexus.core.auth.LoginPhase
import mn.gerege.nexus.ui.brand.BrandBadge
import mn.gerege.nexus.ui.brand.BrandField
import mn.gerege.nexus.ui.brand.BrandScreen
import mn.gerege.nexus.ui.brand.BrandSectionLabel
import mn.gerege.nexus.ui.brand.BrandSecurityFooter
import mn.gerege.nexus.ui.brand.BrandWordmark
import mn.gerege.nexus.ui.brand.LoadingPrimaryButton
import mn.gerege.nexus.ui.brand.QuietButton
import mn.gerege.nexus.ui.brand.TonalButton
import mn.gerege.nexus.ui.brand.isLightTheme
import mn.gerege.nexus.ui.icons.Lucide
import mn.gerege.nexus.ui.theme.LocalGw
import mn.gerege.nexus.ui.theme.Radius
import mn.gerege.nexus.ui.theme.Space

@Composable
internal fun NativeLogin(auth: AuthStateMachine, deviceToken: String?, apiOrigin: String) {
    val phase by auth.phase.collectAsStateWithLifecycle()
    val context = LocalContext.current
    val waiting = phase as? LoginPhase.Waiting
    // App2App: device-link session эхэлмэгц (deviceLinkUrl нь цэвэр session id)
    // eID аппыг session-тэй нь нээнэ. Push урсгал deviceLinkUrl буцаадаггүй тул
    // энэ зөвхөн «eID апп-аар нэвтрэх»-д ажиллана; нэг л удаа (key солигдохгүй).
    LaunchedEffect(waiting?.deviceLinkUrl) {
        if (BuildConfig.FORM_FACTOR in setOf("mobile", "tablet")) waiting?.deviceLinkUrl?.let {
            openEidApp(context, it)
        }
    }
    when (BuildConfig.FORM_FACTOR) {
        "kiosk" -> KioskLogin(auth, phase)
        "pos" -> PosLogin(auth, phase, deviceToken, apiOrigin)
        "tablet" -> DefaultLogin(auth, phase, deviceToken, apiOrigin)
        else -> MobileLogin(auth, phase, apiOrigin)
    }
}

// eID Mongolia аппын Android package. startActivity нээхийг зөвшөөрдөг тул
// scheme resolve хийхэд <queries> хэрэггүй; суулгаагүй бол ActivityNotFound шиднэ.
private const val EID_APP_PACKAGE = "mn.eidmongolia.dan"

/**
 * App2App: eID Mongolia аппыг session-тэй нь нээх.
 *
 * `sessionId` нь backend-ийн device-link эхлүүлэгчээс ирсэн цэвэр session id
 * (kiosk дээр QR-д ордог утга). Мобайл дээр түүнийг eID аппын өөрийн бүртгэсэн
 * deep link болгоно — web EIDLogin.tsx-тэй яг ижил хэлбэр:
 * `geregesmartid://approve?sessionId=<id>`. eID апп нээгдэж баталгаажуулах кодоо
 * үзүүлнэ; иргэн баталсны дараа Nexus-ийн poll «Нэвтэрлээ» болж, back дархад
 * Nexus руу буцна. Апп суулгаагүй бол Play Store-ийн хуудсыг нээнэ.
 */
private fun openEidApp(context: android.content.Context, sessionId: String) {
    val approve = Uri.parse("geregesmartid://approve?sessionId=" + Uri.encode(sessionId))
    val launched = runCatching { context.startActivity(Intent(Intent.ACTION_VIEW, approve)) }.isSuccess
    if (!launched) runCatching {
        context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse("https://play.google.com/store/apps/details?id=$EID_APP_PACKAGE")))
    }
}

/* ================== v2 mobile нэвтрэх (design_handoff v2) ==================
 * eID үндсэн зам; админ нь доод «Админ» линкээр нээгддэг ижил хэмжээт card
 * (modal биш). Төлөвүүд eID card-ын дотор AnimatedContent-ээр солигдоно —
 * хүрээний өнгө солигдохгүй, статусыг зөвхөн цэг/икон/label илэрхийлнэ (§1b).
 * Бүх бичвэр stringResource — Тохиргооны «Хэл»-ээс mn/en/ru сэлгэнэ.        */

@Composable
private fun MobileLogin(auth: AuthStateMachine, phase: LoginPhase, apiOrigin: String) {
    var adminMode by remember { mutableStateOf(false) }
    var nationalId by remember { mutableStateOf("") }
    var email by remember { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    BrandScreen {
        Column(
            Modifier.fillMaxSize()
                .padding(start = Space.xl, end = Space.xl, top = Space.xxxl, bottom = Space.xl)
                .imePadding(),
        ) {
            MobileHeader()
            Spacer(Modifier.height(Space.xxl))
            AnimatedContent(
                targetState = adminMode,
                transitionSpec = { fadeIn(tween(200)) togetherWith fadeOut(tween(200)) },
                label = "login-mode",
            ) { admin ->
                if (admin) AdminCardV2(
                    phase, email, { email = it }, password, { password = it },
                    onBack = { adminMode = false },
                    onSubmit = { auth.password(email, password) },
                ) else EidCardV2(
                    phase = phase, apiOrigin = apiOrigin,
                    nationalId = nationalId, onNationalId = { nationalId = it.uppercase() },
                    onSend = { auth.push(nationalId) },
                    onAppToApp = { auth.appToApp("") },
                    onCancel = auth::cancel,
                    onRetry = { if (nationalId.isNotBlank()) auth.push(nationalId) else auth.cancel() },
                )
            }
            Spacer(Modifier.weight(1f))
            MobileFooter(showAdmin = !adminMode) { adminMode = true; auth.cancel() }
        }
    }
}

@Composable private fun MobileHeader() {
    val gw = LocalGw.current
    Column(verticalArrangement = Arrangement.spacedBy(Space.xl)) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(Space.md)) {
            BrandWordmark(40.dp, Radius.md)
            Row {
                Text("GEREGE", color = gw.fg3, fontSize = 13.sp, fontWeight = FontWeight.SemiBold, letterSpacing = 13.sp * 0.16f)
                Text(" / ", color = gw.fg4, fontSize = 13.sp, fontWeight = FontWeight.SemiBold, letterSpacing = 13.sp * 0.16f)
                Text("NEXUS", color = gw.fg3, fontSize = 13.sp, fontWeight = FontWeight.SemiBold, letterSpacing = 13.sp * 0.16f)
            }
        }
        Text(stringResource(R.string.v2_title), color = gw.fg1, fontSize = 34.sp, lineHeight = 39.sp, fontWeight = FontWeight.Bold)
    }
}

@Composable private fun MobileFooter(showAdmin: Boolean, onAdmin: () -> Unit) {
    val gw = LocalGw.current
    Row(
        Modifier.fillMaxWidth().heightIn(min = 48.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.SpaceBetween,
    ) {
        Row(Modifier.weight(1f), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(Space.sm)) {
            Icon(Lucide.Lock, contentDescription = null, tint = gw.fg3, modifier = Modifier.size(14.dp))
            Text(stringResource(R.string.v2_footer), color = gw.fg3, fontSize = 12.sp, fontWeight = FontWeight.SemiBold)
        }
        if (showAdmin) Box(
            Modifier.height(48.dp).clickable(onClick = onAdmin).padding(horizontal = Space.sm),
            contentAlignment = Alignment.Center,
        ) { Text(stringResource(R.string.v2_admin), color = gw.brand, fontSize = 13.sp, fontWeight = FontWeight.SemiBold) }
    }
}

@Composable private fun V2Card(content: @Composable () -> Unit) {
    val gw = LocalGw.current
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(Radius.xl),
        color = gw.surface1,
        border = BorderStroke(1.dp, gw.border),
        shadowElevation = if (isLightTheme()) 8.dp else 0.dp,
    ) { content() }
}

@Composable
private fun EidCardV2(
    phase: LoginPhase, apiOrigin: String,
    nationalId: String, onNationalId: (String) -> Unit,
    onSend: () -> Unit, onAppToApp: () -> Unit, onCancel: () -> Unit, onRetry: () -> Unit,
) {
    V2Card {
        AnimatedContent(
            targetState = phase, contentKey = ::phaseKey,
            transitionSpec = { fadeIn(tween(200)) togetherWith fadeOut(tween(200)) },
            label = "eid-state",
        ) { state ->
            when (phaseKey(state)) {
                "form" -> if (state is LoginPhase.Starting) SendingCardV2(nationalId) else IdleCardV2(nationalId, onNationalId, onSend, onAppToApp)
                "waiting" -> WaitingCardV2(state as? LoginPhase.Waiting, nationalId, onCancel)
                "success" -> SuccessCardV2()
                "expired" -> ExpiredCardV2(onRetry)
                "refused" -> RefusedCardV2(onRetry)
                else -> ErrorCardV2(state as? LoginPhase.Error, apiOrigin, onRetry)
            }
        }
    }
}

/** Статусын мөр (§1b): 8dp цэг + ALL CAPS label, баруунд заавал биш trailing. */
@Composable private fun StatusRowV2(color: ComposeColor, label: String, trailing: String? = null, pulse: Boolean = false) {
    val gw = LocalGw.current
    Row(
        Modifier.fillMaxWidth().heightIn(min = 24.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.SpaceBetween,
    ) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(Space.sm)) {
            if (pulse) PulseDot() else Box(Modifier.size(8.dp).background(color, CircleShape))
            Text(label, color = color, fontSize = 13.sp, fontWeight = FontWeight.SemiBold, letterSpacing = 13.sp * 0.08f)
        }
        if (trailing != null) Text(trailing, color = gw.fg2, fontSize = 13.sp, fontWeight = FontWeight.SemiBold)
    }
}

/** Төвлөрсөн статус агуулга: 56dp тонал дугуй + икон, гарчиг, тайлбар. */
@Composable private fun CenteredStatusV2(color: ComposeColor, soft: ComposeColor, icon: ImageVector, title: String, desc: String) {
    val gw = LocalGw.current
    Column(
        Modifier.fillMaxWidth(),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(Space.md),
    ) {
        Box(Modifier.size(56.dp).background(soft, CircleShape), contentAlignment = Alignment.Center) {
            Icon(icon, contentDescription = null, tint = color, modifier = Modifier.size(24.dp))
        }
        Text(title, color = gw.fg1, fontSize = 17.sp, fontWeight = FontWeight.SemiBold, textAlign = TextAlign.Center, lineHeight = 23.sp)
        Text(desc, color = gw.fg2, fontSize = 15.sp, textAlign = TextAlign.Center, lineHeight = 22.sp)
    }
}

/** Terminal төлөвийн бүрхүүл: дээр статус мөр, дунд төвлөрсөн, доор үйлдэл. */
@Composable private fun TerminalCardV2(
    color: ComposeColor, soft: ComposeColor, statusLabel: String,
    icon: ImageVector, title: String, desc: String, action: @Composable () -> Unit,
) {
    Column(Modifier.fillMaxWidth().heightIn(min = 296.dp).padding(Space.xl)) {
        StatusRowV2(color, statusLabel)
        Box(Modifier.weight(1f).fillMaxWidth().padding(vertical = Space.sm), contentAlignment = Alignment.Center) {
            CenteredStatusV2(color, soft, icon, title, desc)
        }
        action()
    }
}

@Composable private fun IdleCardV2(nationalId: String, onNationalId: (String) -> Unit, onSend: () -> Unit, onAppToApp: () -> Unit) {
    val gw = LocalGw.current
    Column(
        Modifier.padding(horizontal = Space.xl, vertical = Space.xxl),
        verticalArrangement = Arrangement.spacedBy(Space.lg),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(Space.sm)) {
            Icon(Lucide.ShieldCheck, contentDescription = null, tint = gw.brand, modifier = Modifier.size(24.dp))
            Text(stringResource(R.string.v2_eid_title), color = gw.fg1, fontSize = 20.sp, fontWeight = FontWeight.SemiBold)
        }
        BrandField(nationalId, onNationalId, stringResource(R.string.v2_reg_number), height = 64.dp, fontSize = 17.sp)
        LoadingPrimaryButton(stringResource(R.string.v2_send), isLoading = false, isEnabled = true, height = 64.dp, fontSize = 17.sp, trailingIcon = Lucide.ArrowRight, onClick = onSend)
        TonalButton(stringResource(R.string.v2_app2app), height = 64.dp, fontSize = 17.sp, leadingIcon = Lucide.Smartphone, onClick = onAppToApp)
    }
}

@Composable private fun SendingCardV2(nationalId: String) {
    val gw = LocalGw.current
    Column(Modifier.fillMaxWidth().heightIn(min = 296.dp).padding(Space.xl)) {
        Column(verticalArrangement = Arrangement.spacedBy(Space.lg)) {
            StatusRowV2(gw.brand, stringResource(R.string.v2_st_sending))
            Surface(Modifier.fillMaxWidth().height(56.dp), RoundedCornerShape(Radius.md), color = gw.surface2, border = BorderStroke(1.dp, gw.borderStrong)) {
                Box(Modifier.padding(horizontal = Space.lg), contentAlignment = Alignment.CenterStart) {
                    Text(nationalId.ifBlank { stringResource(R.string.v2_reg_number) }, color = if (nationalId.isBlank()) gw.fg3 else gw.fg2, fontSize = 17.sp)
                }
            }
        }
        Spacer(Modifier.weight(1f))
        Surface(Modifier.fillMaxWidth().height(56.dp), RoundedCornerShape(Radius.md), color = gw.brandDeep) {
            Row(Modifier.fillMaxSize(), horizontalArrangement = Arrangement.Center, verticalAlignment = Alignment.CenterVertically) {
                CircularProgressIndicator(color = gw.onBrand, strokeWidth = 2.dp, modifier = Modifier.size(18.dp))
                Spacer(Modifier.width(Space.sm))
                Text(stringResource(R.string.v2_sending), color = gw.onBrand.copy(alpha = 0.7f), fontSize = 17.sp, fontWeight = FontWeight.SemiBold)
            }
        }
    }
}

@Composable private fun WaitingCardV2(waiting: LoginPhase.Waiting?, nationalId: String, onCancel: () -> Unit) {
    val gw = LocalGw.current
    val remaining = rememberRemainingMillis(waiting)
    val total = remember(waiting) { ((waiting?.expiresAtMillis ?: 0L) - System.currentTimeMillis()).coerceAtLeast(1L) }
    Column(Modifier.fillMaxWidth().heightIn(min = 296.dp).padding(Space.xl)) {
        Column(verticalArrangement = Arrangement.spacedBy(Space.lg)) {
            StatusRowV2(gw.brand, stringResource(R.string.v2_st_waiting), trailing = remaining?.let { formatMmSs(it) }, pulse = true)
            Text(stringResource(R.string.v2_wait_title), color = gw.fg1, fontSize = 17.sp, fontWeight = FontWeight.SemiBold)
            CodeBoxesV2(waiting?.verificationCode.orEmpty())
            LinearProgressIndicator(
                progress = { if (remaining != null) (remaining.toFloat() / total).coerceIn(0f, 1f) else 1f },
                modifier = Modifier.fillMaxWidth().height(4.dp),
                color = gw.brand, trackColor = gw.surface3, gapSize = 0.dp, drawStopIndicator = {},
            )
            Text(
                if (nationalId.isBlank()) stringResource(R.string.v2_wait_hint_generic) else stringResource(R.string.v2_wait_hint, nationalId),
                color = gw.fg3, fontSize = 13.sp, lineHeight = 20.sp,
            )
        }
        Spacer(Modifier.weight(1f))
        QuietButton(stringResource(R.string.v2_cancel), height = 56.dp, fontSize = 15.sp, onClick = onCancel)
    }
}

@Composable private fun CodeBoxesV2(code: String) {
    val gw = LocalGw.current
    val digits = if (code.isEmpty()) "····" else code
    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(Space.sm)) {
        digits.forEach { d ->
            Surface(Modifier.weight(1f).height(64.dp), RoundedCornerShape(Radius.md), color = gw.brandSoft, border = BorderStroke(1.dp, gw.brandDeep)) {
                Box(contentAlignment = Alignment.Center) {
                    Text(if (d == '·') "" else d.toString(), color = gw.fg1, fontSize = 28.sp, fontWeight = FontWeight.Bold)
                }
            }
        }
    }
}

@Composable private fun SuccessCardV2() {
    val gw = LocalGw.current
    TerminalCardV2(
        gw.credit, gw.creditSoft, stringResource(R.string.v2_st_success),
        Lucide.Check, stringResource(R.string.v2_success_title), stringResource(R.string.v2_success_desc),
    ) {
        Surface(Modifier.fillMaxWidth().height(56.dp), RoundedCornerShape(Radius.md), color = gw.creditSoft) {
            Row(Modifier.fillMaxSize(), horizontalArrangement = Arrangement.Center, verticalAlignment = Alignment.CenterVertically) {
                CircularProgressIndicator(color = gw.credit, strokeWidth = 2.dp, modifier = Modifier.size(16.dp))
                Spacer(Modifier.width(Space.sm))
                Text(stringResource(R.string.v2_entering), color = gw.credit, fontSize = 15.sp, fontWeight = FontWeight.SemiBold)
            }
        }
    }
}

@Composable private fun ExpiredCardV2(onResend: () -> Unit) {
    val gw = LocalGw.current
    TerminalCardV2(
        gw.gold, gw.goldSoft, stringResource(R.string.v2_st_expired),
        Lucide.Clock, stringResource(R.string.v2_expired_title), stringResource(R.string.v2_expired_desc),
    ) { QuietButton(stringResource(R.string.v2_resend), height = 56.dp, fontSize = 15.sp, leadingIcon = Lucide.RefreshCw, onClick = onResend) }
}

@Composable private fun RefusedCardV2(onRetry: () -> Unit) {
    val gw = LocalGw.current
    TerminalCardV2(
        gw.debit, gw.debitSoft, stringResource(R.string.v2_st_refused),
        Lucide.X, stringResource(R.string.v2_refused_title), stringResource(R.string.v2_refused_desc),
    ) { QuietButton(stringResource(R.string.v2_new_request), height = 56.dp, fontSize = 15.sp, onClick = onRetry) }
}

@Composable private fun ErrorCardV2(error: LoginPhase.Error?, apiOrigin: String, onRetry: () -> Unit) {
    val gw = LocalGw.current
    var showDiag by remember { mutableStateOf(false) }
    Column(Modifier.fillMaxWidth().heightIn(min = 296.dp).padding(Space.xl)) {
        StatusRowV2(gw.debit, stringResource(R.string.v2_st_error))
        Box(Modifier.weight(1f).fillMaxWidth().padding(vertical = Space.sm), contentAlignment = Alignment.Center) {
            CenteredStatusV2(gw.debit, gw.debitSoft, Lucide.TriangleAlert, error?.message ?: stringResource(R.string.v2_error_title), stringResource(R.string.v2_error_desc))
        }
        Row(horizontalArrangement = Arrangement.spacedBy(Space.sm)) {
            QuietButton(stringResource(R.string.v2_retry), Modifier.weight(1f), height = 56.dp, fontSize = 15.sp, onClick = onRetry)
            QuietButton(stringResource(R.string.v2_diagnostics), Modifier.weight(1f), height = 56.dp, fontSize = 15.sp) { showDiag = !showDiag }
        }
        if (showDiag) Text("$apiOrigin · 1.4\n${error?.detail.orEmpty()}", color = gw.fg3, fontSize = 12.sp, modifier = Modifier.padding(top = Space.sm))
    }
}

@Composable private fun AdminCardV2(
    phase: LoginPhase, email: String, onEmail: (String) -> Unit, password: String, onPassword: (String) -> Unit,
    onBack: () -> Unit, onSubmit: () -> Unit,
) {
    val gw = LocalGw.current
    V2Card {
        Column(Modifier.padding(horizontal = Space.xl, vertical = Space.xxl), verticalArrangement = Arrangement.spacedBy(Space.lg)) {
            Row(
                Modifier.heightIn(min = 24.dp).clickable(onClick = onBack),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(Space.sm),
            ) {
                Icon(Lucide.ArrowLeft, contentDescription = null, tint = gw.fg3, modifier = Modifier.size(16.dp))
                Text(stringResource(R.string.v2_admin_title), color = gw.fg1, fontSize = 20.sp, fontWeight = FontWeight.SemiBold)
            }
            BrandField(email, onEmail, stringResource(R.string.v2_email), height = 64.dp, fontSize = 17.sp, keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Email))
            BrandField(password, onPassword, stringResource(R.string.v2_password), height = 64.dp, fontSize = 17.sp, keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password), visualTransformation = PasswordVisualTransformation())
            LoadingPrimaryButton(stringResource(R.string.v2_admin_submit), isLoading = phase is LoginPhase.Starting, isEnabled = phase !is LoginPhase.Starting, height = 64.dp, fontSize = 17.sp, trailingIcon = null, onClick = onSubmit)
            if (phase is LoginPhase.Error) Text(phase.message, color = gw.debit, fontSize = 13.sp)
        }
    }
}

/* ---------- Mobile + tablet (§1a, §1e-tablet) ---------- */

@Composable
private fun DefaultLogin(auth: AuthStateMachine, phase: LoginPhase, deviceToken: String?, apiOrigin: String) {
    val tablet = BuildConfig.FORM_FACTOR == "tablet"
    var email by remember { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    var nationalId by remember { mutableStateOf("") }
    var staffPin by remember { mutableStateOf("") }
    val busy = phase is LoginPhase.Starting || phase is LoginPhase.Waiting
    val online = rememberOnline()
    BrandScreen {
        Box(
            Modifier.fillMaxSize().padding(horizontal = Space.xl).imePadding(),
            contentAlignment = Alignment.Center,
        ) {
            Column(
                Modifier.widthIn(max = if (tablet) 520.dp else 420.dp)
                    .verticalScroll(rememberScrollState())
                    .padding(vertical = Space.xl),
                verticalArrangement = Arrangement.spacedBy(Space.xl),
            ) {
                LoginHeader(tablet)
                if (!online) OfflineNotice()
                EidCard(
                    phase = phase,
                    apiOrigin = apiOrigin,
                    nationalId = nationalId,
                    onNationalId = { nationalId = it.uppercase() },
                    inlineSend = tablet,
                    showAppToApp = true,
                    onSend = { auth.push(nationalId) },
                    // Callback хоосон: native App2App-д approve хийсний дараа eID
                    // апп биднийг хөтөч рүү (nexus.gerege.mn) шидэхийг хүсэхгүй —
                    // Nexus өөрөө poll хийж дуусгаад, back дархад буцаж ирнэ.
                    onAppToApp = { auth.appToApp("") },
                    onCancel = auth::cancel,
                    onRetry = { if (nationalId.isNotBlank()) auth.push(nationalId) else auth.cancel() },
                )
                OrDivider()
                if (tablet && deviceToken != null) Row(horizontalArrangement = Arrangement.spacedBy(Space.xl)) {
                    Column(Modifier.weight(1f)) { StaffZone(staffPin, { staffPin = it }, busy) { auth.staffPin(staffPin, deviceToken) } }
                    Column(Modifier.weight(1f)) { AdminZone(email, { email = it }, password, { password = it }, busy) { auth.password(email, password) } }
                } else {
                    AdminZone(email, { email = it }, password, { password = it }, busy) { auth.password(email, password) }
                }
                BrandSecurityFooter(stringResource(R.string.auth_eid_footer))
            }
        }
    }
}

@Composable
private fun LoginHeader(horizontal: Boolean) {
    val gw = LocalGw.current
    if (horizontal) Row(
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(Space.lg),
    ) {
        BrandWordmark(48.dp, Radius.md)
        Column(verticalArrangement = Arrangement.spacedBy(Space.xs)) {
            BrandSectionLabel("GEREGE / NEXUS", fontSize = 12.sp, trackingEm = 0.16f)
            Text("Таны баталгаатай ажлын орчин", color = gw.fg1, fontSize = 28.sp, lineHeight = 32.sp, fontWeight = FontWeight.Bold)
        }
    } else Column(verticalArrangement = Arrangement.spacedBy(Space.md)) {
        BrandWordmark(48.dp, Radius.md)
        BrandSectionLabel("GEREGE / NEXUS", fontSize = 12.sp, trackingEm = 0.16f)
        Text("Таны баталгаатай\nажлын орчин", color = gw.fg1, fontSize = 34.sp, lineHeight = 39.sp, fontWeight = FontWeight.Bold)
    }
}

/* ---------- eID үндсэн card + төлөвүүд (§1b) ---------- */

private fun phaseKey(phase: LoginPhase): String = when (phase) {
    LoginPhase.Idle, LoginPhase.Starting -> "form"
    is LoginPhase.Waiting -> "waiting"
    LoginPhase.Success -> "success"
    LoginPhase.Expired -> "expired"
    LoginPhase.Refused -> "refused"
    is LoginPhase.Error -> "error"
}

@Composable
private fun EidCard(
    phase: LoginPhase,
    apiOrigin: String,
    nationalId: String,
    onNationalId: (String) -> Unit,
    inlineSend: Boolean,
    showAppToApp: Boolean,
    onSend: () -> Unit,
    onAppToApp: () -> Unit,
    onCancel: () -> Unit,
    onRetry: () -> Unit,
) {
    val gw = LocalGw.current
    val borderColor by animateColorAsState(
        targetValue = when (phase) {
            is LoginPhase.Waiting -> gw.brand.copy(alpha = 0.35f)
            LoginPhase.Success -> gw.credit.copy(alpha = 0.4f)
            LoginPhase.Refused, is LoginPhase.Error -> gw.debit.copy(alpha = 0.35f)
            else -> gw.borderStrong
        },
        label = "eid-card-border",
    )
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(Radius.xl),
        color = gw.surface1,
        border = BorderStroke(1.dp, borderColor),
        shadowElevation = if (isLightTheme()) 8.dp else 0.dp,
    ) {
        Column(Modifier.padding(Space.xl), verticalArrangement = Arrangement.spacedBy(Space.lg)) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(Space.sm + Space.xxs)) {
                Icon(Lucide.ShieldCheck, contentDescription = null, tint = gw.brand, modifier = Modifier.size(20.dp))
                Text("eID-ээр нэвтрэх", color = gw.fg1, fontSize = 17.sp, fontWeight = FontWeight.SemiBold)
                Spacer(Modifier.weight(1f))
                when (phase) {
                    is LoginPhase.Waiting -> Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(Space.sm)) {
                        PulseDot()
                        BrandSectionLabel("ХҮЛЭЭЖ БАЙНА", fontSize = 12.sp, color = gw.brand)
                    }
                    LoginPhase.Success -> BrandBadge("АМЖИЛТТАЙ", color = gw.credit, container = gw.creditSoft)
                    LoginPhase.Expired -> BrandBadge("ХУГАЦАА ДУУССАН", color = gw.gold, container = gw.goldSoft)
                    LoginPhase.Refused -> BrandBadge("ТАТГАЛЗСАН", color = gw.debit, container = gw.debitSoft)
                    is LoginPhase.Error -> BrandBadge("АЛДАА", color = gw.debit, container = gw.debitSoft)
                    else -> BrandBadge("ҮНДСЭН")
                }
            }
            AnimatedContent(
                targetState = phase,
                contentKey = ::phaseKey,
                transitionSpec = { fadeIn(tween(200)) togetherWith fadeOut(tween(200)) },
                label = "eid-card-state",
            ) { state ->
                when (phaseKey(state)) {
                    "form" -> EidForm(state is LoginPhase.Starting, nationalId, onNationalId, inlineSend, showAppToApp, onSend, onAppToApp)
                    "waiting" -> WaitingBlock(state as? LoginPhase.Waiting, nationalId, onCancel)
                    "success" -> SuccessBlock()
                    "expired" -> ExpiredBlock(onRetry)
                    "refused" -> RefusedBlock(onRetry)
                    else -> ErrorBlock(state as? LoginPhase.Error, apiOrigin, onRetry)
                }
            }
        }
    }
}

@Composable
private fun EidForm(
    loading: Boolean,
    nationalId: String,
    onNationalId: (String) -> Unit,
    inlineSend: Boolean,
    showAppToApp: Boolean,
    onSend: () -> Unit,
    onAppToApp: () -> Unit,
) {
    val gw = LocalGw.current
    Column(verticalArrangement = Arrangement.spacedBy(Space.lg)) {
        if (inlineSend) Row(horizontalArrangement = Arrangement.spacedBy(Space.md)) {
            BrandField(nationalId, onNationalId, stringResource(R.string.auth_eid_reg_number), Modifier.weight(1f), enabled = !loading)
            LoadingPrimaryButton(stringResource(R.string.auth_eid_send_request), loading, !loading, Modifier.weight(1f), onClick = onSend)
        } else {
            BrandField(nationalId, onNationalId, stringResource(R.string.auth_eid_reg_number), enabled = !loading)
            LoadingPrimaryButton(stringResource(R.string.auth_eid_send_request), loading, !loading, onClick = onSend)
        }
        if (loading) Text("Хүсэлт илгээж байна…", color = gw.fg2, fontSize = 15.sp)
        if (showAppToApp) TonalButton(stringResource(R.string.auth_action_app_to_app), isEnabled = !loading, leadingIcon = Lucide.Smartphone, onClick = onAppToApp)
    }
}

@Composable
private fun WaitingBlock(waiting: LoginPhase.Waiting?, nationalId: String, onCancel: () -> Unit) {
    val gw = LocalGw.current
    Column(verticalArrangement = Arrangement.spacedBy(Space.lg)) {
        Text("eID апп дээрх кодтой тулгана уу", color = gw.fg1, fontSize = 17.sp, fontWeight = FontWeight.SemiBold)
        CodeBoxes(waiting?.verificationCode.orEmpty())
        CountdownBlock(waiting)
        Text(
            if (nationalId.isBlank()) "eID апп руу хүсэлт илгээгдлээ"
            else "$nationalId бүртгэлтэй eID апп руу хүсэлт илгээгдлээ",
            color = gw.fg3,
            fontSize = 13.sp,
        )
        QuietButton(stringResource(R.string.auth_action_cancel), onClick = onCancel)
    }
}

@Composable
private fun CodeBoxes(code: String) {
    val gw = LocalGw.current
    Row(
        Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(Space.md, Alignment.CenterHorizontally),
    ) {
        code.take(6).forEach { digit ->
            Surface(
                modifier = Modifier.width(56.dp).height(64.dp),
                shape = RoundedCornerShape(Radius.md),
                color = gw.brandSoft,
                border = BorderStroke(1.dp, gw.brandDeep),
            ) {
                Box(contentAlignment = Alignment.Center) {
                    Text(digit.toString(), color = gw.fg1, fontSize = 28.sp, fontWeight = FontWeight.Bold)
                }
            }
        }
    }
}

/** mm:ss хорогдол ба пропорциональ progress — эх нь Waiting.expiresAtMillis. */
@Composable
private fun CountdownBlock(waiting: LoginPhase.Waiting?) {
    val gw = LocalGw.current
    val remaining = rememberRemainingMillis(waiting) ?: return
    val total = remember(waiting) { (waiting?.expiresAtMillis ?: 0L) - System.currentTimeMillis() }.coerceAtLeast(1L)
    Column(verticalArrangement = Arrangement.spacedBy(Space.sm)) {
        LinearProgressIndicator(
            progress = { (remaining.toFloat() / total).coerceIn(0f, 1f) },
            modifier = Modifier.fillMaxWidth().height(4.dp),
            color = gw.brand,
            trackColor = gw.surface3,
            gapSize = 0.dp,
            drawStopIndicator = {},
        )
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
            Text("Хүсэлт хүчинтэй", color = gw.fg3, fontSize = 13.sp)
            Text(formatMmSs(remaining), color = gw.fg2, fontSize = 13.sp, fontWeight = FontWeight.SemiBold)
        }
    }
}

@Composable
private fun rememberRemainingMillis(waiting: LoginPhase.Waiting?): Long? {
    val expires = waiting?.expiresAtMillis ?: return null
    var now by remember { mutableLongStateOf(System.currentTimeMillis()) }
    LaunchedEffect(expires) {
        while (true) { now = System.currentTimeMillis(); delay(1_000) }
    }
    return (expires - now).coerceAtLeast(0L)
}

private fun formatMmSs(millis: Long): String {
    val sec = millis / 1_000
    return "%02d:%02d".format(sec / 60, sec % 60)
}

/** 8dp цэнхэр цэг, 1.6s ease-in-out pulse (1 → 0.45). */
@Composable
private fun PulseDot() {
    val gw = LocalGw.current
    val transition = rememberInfiniteTransition(label = "pulse")
    val alpha by transition.animateFloat(
        initialValue = 1f,
        targetValue = 0.45f,
        animationSpec = infiniteRepeatable(tween(800, easing = FastOutSlowInEasing), RepeatMode.Reverse),
        label = "pulse-alpha",
    )
    Box(Modifier.size(8.dp).graphicsLayer { this.alpha = alpha }.background(gw.brand, CircleShape))
}

@Composable
private fun SuccessBlock() {
    val gw = LocalGw.current
    Column(verticalArrangement = Arrangement.spacedBy(Space.lg)) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(Space.md)) {
            Box(Modifier.size(32.dp).background(gw.creditSoft, CircleShape), contentAlignment = Alignment.Center) {
                Icon(Lucide.Check, contentDescription = null, tint = gw.credit, modifier = Modifier.size(18.dp))
            }
            Text("Баталгаажлаа — нэвтэрч байна", color = gw.fg1, fontSize = 15.sp, fontWeight = FontWeight.SemiBold)
        }
        LinearProgressIndicator(
            progress = { 1f },
            modifier = Modifier.fillMaxWidth().height(4.dp),
            color = gw.credit,
            trackColor = gw.surface3,
            gapSize = 0.dp,
            drawStopIndicator = {},
        )
    }
}

@Composable
private fun ExpiredBlock(onResend: () -> Unit) {
    val gw = LocalGw.current
    Column(verticalArrangement = Arrangement.spacedBy(Space.lg)) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(Space.md)) {
            Icon(Lucide.Clock, contentDescription = null, tint = gw.gold, modifier = Modifier.size(20.dp))
            Text("Хариу ирсэнгүй — код хүчингүй боллоо", color = gw.fg1, fontSize = 15.sp)
        }
        QuietButton("Дахин илгээх", leadingIcon = Lucide.RefreshCw, onClick = onResend)
    }
}

@Composable
private fun RefusedBlock(onRetry: () -> Unit) {
    val gw = LocalGw.current
    Column(verticalArrangement = Arrangement.spacedBy(Space.lg)) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(Space.md)) {
            Box(Modifier.size(32.dp).background(gw.debitSoft, CircleShape), contentAlignment = Alignment.Center) {
                Icon(Lucide.X, contentDescription = null, tint = gw.debit, modifier = Modifier.size(18.dp))
            }
            Text("eID апп дээр хүсэлтийг татгалзсан", color = gw.fg1, fontSize = 15.sp)
        }
        QuietButton("Шинэ хүсэлт илгээх", onClick = onRetry)
    }
}

@Composable
private fun ErrorBlock(error: LoginPhase.Error?, apiOrigin: String, onRetry: () -> Unit) {
    val gw = LocalGw.current
    var showDiag by remember { mutableStateOf(false) }
    Column(verticalArrangement = Arrangement.spacedBy(Space.lg)) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(Space.md)) {
            Icon(Lucide.TriangleAlert, contentDescription = null, tint = gw.debit, modifier = Modifier.size(20.dp))
            Text(error?.message ?: "Сервертэй холбогдож чадсангүй", color = gw.fg1, fontSize = 15.sp)
        }
        Row(horizontalArrangement = Arrangement.spacedBy(Space.md)) {
            QuietButton(stringResource(R.string.auth_action_retry), Modifier.weight(1f), onClick = onRetry)
            QuietButton("Диагностик", Modifier.weight(1f)) { showDiag = !showDiag }
        }
        // Техник мэдээлэл зөвхөн энд: аппын бодитоор хандаж буй шугам (тохиргооноос
        // өөрчилсөн бол өөрчилсөн нь) ба exception-ийн бүрэн бичвэр. Дэмжлэгт
        // хандахад яг үүнийг л уншуулна.
        if (showDiag) Column(verticalArrangement = Arrangement.spacedBy(Space.xs)) {
            Text("Шугам: $apiOrigin · Shell contract 1.4", color = gw.fg3, fontSize = 12.sp)
            error?.detail?.let { Text(it, color = gw.fg3, fontSize = 12.sp) }
        }
    }
}

/**
 * Интернэтгүй үед card-ын дээр гарах анхааруулга. Алдаа биш — хүсэлт хараахан
 * илгээгдээгүй, зүгээр л илгээвэл юу болохыг урьдчилж хэлж байна. Товчнууд
 * идэвхтэй хэвээр: Android-ийн validation сүлжээ сэргэсний дараа хэдэн секунд
 * хоцордог тул хэрэглэгчийг түгжихгүй.
 */
@Composable
private fun OfflineNotice() {
    val gw = LocalGw.current
    Surface(shape = RoundedCornerShape(Radius.md), color = gw.goldSoft) {
        Row(
            Modifier.padding(horizontal = Space.lg, vertical = Space.md),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(Space.md),
        ) {
            Icon(Lucide.WifiOff, contentDescription = null, tint = gw.gold, modifier = Modifier.size(18.dp))
            Text("Интернэт холболт алга — Wi-Fi эсвэл мобайл датагаа шалгана уу", color = gw.fg1, fontSize = 13.sp)
        }
    }
}

/* ---------- Хоёрдогч бүсүүд ---------- */

@Composable
private fun OrDivider() {
    val gw = LocalGw.current
    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(Space.md)) {
        HorizontalDivider(Modifier.weight(1f), color = gw.divider)
        Text("эсвэл", color = gw.fg4, fontSize = 13.sp)
        HorizontalDivider(Modifier.weight(1f), color = gw.divider)
    }
}

@Composable
private fun ColumnScope.AdminZone(
    email: String,
    onEmail: (String) -> Unit,
    password: String,
    onPassword: (String) -> Unit,
    busy: Boolean,
    onSubmit: () -> Unit,
) {
    Column(verticalArrangement = Arrangement.spacedBy(Space.md)) {
        BrandSectionLabel("АДМИН НЭВТРЭЛТ")
        BrandField(
            email, onEmail, stringResource(R.string.auth_field_email),
            height = 48.dp, fontSize = 15.sp, filled = false, enabled = !busy,
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Email),
        )
        BrandField(
            password, onPassword, stringResource(R.string.auth_field_password),
            height = 48.dp, fontSize = 15.sp, filled = false, enabled = !busy,
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password),
            visualTransformation = PasswordVisualTransformation(),
        )
        QuietButton(stringResource(R.string.auth_action_admin_sign_in), isEnabled = !busy, onClick = onSubmit)
    }
}

@Composable
private fun ColumnScope.StaffZone(pin: String, onPin: (String) -> Unit, busy: Boolean, onSubmit: () -> Unit) {
    val gw = LocalGw.current
    Column(verticalArrangement = Arrangement.spacedBy(Space.md)) {
        BrandSectionLabel("АЖИЛТНЫ ХУРДАН НЭВТРЭЛТ", color = gw.gold)
        BrandField(
            pin, { onPin(it.filter(Char::isDigit).take(12)) }, stringResource(R.string.auth_field_staff_pin),
            height = 48.dp, fontSize = 15.sp, enabled = !busy,
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.NumberPassword),
            visualTransformation = PasswordVisualTransformation(),
        )
        TonalButton("Нэвтрэх", isEnabled = pin.length >= 4 && !busy, height = 48.dp, fontSize = 15.sp, onClick = onSubmit)
    }
}

/* ---------- POS (§1e): PIN pad үндсэн, eID хоёрдогч ---------- */

@Composable
private fun PosLogin(auth: AuthStateMachine, phase: LoginPhase, deviceToken: String?, apiOrigin: String) {
    var nationalId by remember { mutableStateOf("") }
    var pin by remember { mutableStateOf("") }
    val busy = phase is LoginPhase.Starting || phase is LoginPhase.Waiting
    BrandScreen {
        Box(
            Modifier.fillMaxSize().padding(horizontal = Space.xl).imePadding(),
            contentAlignment = Alignment.Center,
        ) {
            Column(
                Modifier.widthIn(max = 420.dp).verticalScroll(rememberScrollState()).padding(vertical = Space.xl),
                verticalArrangement = Arrangement.spacedBy(Space.xl),
            ) {
                LoginHeader(horizontal = false)
                if (!rememberOnline()) OfflineNotice()
                if (deviceToken != null) PinPadCard(pin, { pin = it }, busy) { auth.staffPin(pin, deviceToken) }
                OrDivider()
                EidCard(
                    phase = phase,
                    apiOrigin = apiOrigin,
                    nationalId = nationalId,
                    onNationalId = { nationalId = it.uppercase() },
                    inlineSend = false,
                    showAppToApp = false,
                    onSend = { auth.push(nationalId) },
                    onAppToApp = {},
                    onCancel = auth::cancel,
                    onRetry = { if (nationalId.isNotBlank()) auth.push(nationalId) else auth.cancel() },
                )
                BrandSecurityFooter("Нууц үг дамжуулахгүй")
            }
        }
    }
}

@Composable
private fun PinPadCard(pin: String, onPin: (String) -> Unit, busy: Boolean, onSubmit: () -> Unit) {
    val gw = LocalGw.current
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(Radius.xl),
        color = gw.surface1,
        border = BorderStroke(1.dp, gw.borderStrong),
        shadowElevation = if (isLightTheme()) 8.dp else 0.dp,
    ) {
        Column(Modifier.padding(Space.xl), verticalArrangement = Arrangement.spacedBy(Space.lg)) {
            BrandSectionLabel("АЖИЛТНЫ ХУРДАН НЭВТРЭЛТ", color = gw.gold)
            Row(
                Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(Space.md, Alignment.CenterHorizontally),
            ) {
                repeat(4) { index ->
                    Box(Modifier.size(12.dp).background(if (index < pin.length) gw.fg1 else gw.surface3, CircleShape))
                }
            }
            val rows = listOf(listOf("1", "2", "3"), listOf("4", "5", "6"), listOf("7", "8", "9"), listOf("del", "0", "ok"))
            Column(verticalArrangement = Arrangement.spacedBy(Space.md)) {
                rows.forEach { keys ->
                    Row(horizontalArrangement = Arrangement.spacedBy(Space.md)) {
                        keys.forEach { key ->
                            when (key) {
                                "del" -> PinKey(enabled = !busy && pin.isNotEmpty(), container = ComposeColor.Transparent, onTap = { onPin(pin.dropLast(1)) }) {
                                    Text("Арилгах", color = gw.fg3, fontSize = 15.sp, fontWeight = FontWeight.SemiBold)
                                }
                                "ok" -> PinKey(enabled = !busy && pin.length >= 4, container = gw.brand, onTap = onSubmit) {
                                    Icon(Lucide.Check, contentDescription = "Нэвтрэх", tint = gw.onBrand, modifier = Modifier.size(20.dp))
                                }
                                else -> PinKey(enabled = !busy, container = gw.surface2, onTap = { if (pin.length < 12) onPin(pin + key) }) {
                                    Text(key, color = gw.fg1, fontSize = 17.sp, fontWeight = FontWeight.SemiBold)
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun RowScope.PinKey(
    enabled: Boolean,
    container: ComposeColor,
    onTap: () -> Unit,
    content: @Composable () -> Unit,
) {
    Surface(
        modifier = Modifier.weight(1f).height(48.dp),
        shape = RoundedCornerShape(Radius.md),
        color = if (enabled) container else container.copy(alpha = container.alpha * 0.5f),
    ) {
        Box(
            Modifier.fillMaxSize().clickable(enabled = enabled, onClick = onTap),
            contentAlignment = Alignment.Center,
        ) { content() }
    }
}

/* ---------- Kiosk (§1e): QR self-service ---------- */

@Composable
private fun KioskLogin(auth: AuthStateMachine, phase: LoginPhase) {
    val gw = LocalGw.current
    val waiting = phase as? LoginPhase.Waiting
    BrandScreen {
        Column(Modifier.fillMaxSize()) {
            Row(
                Modifier.fillMaxWidth().height(56.dp).padding(horizontal = Space.xl),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                BrandSectionLabel("KIOSK · SELF-SERVICE", fontSize = 12.sp, trackingEm = 0.16f)
                Spacer(Modifier.weight(1f))
                val remaining = rememberRemainingMillis(waiting)
                if (remaining != null) Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(Space.sm)) {
                    Icon(Lucide.Clock, contentDescription = null, tint = gw.gold, modifier = Modifier.size(16.dp))
                    Text(formatMmSs(remaining), color = gw.gold, fontSize = 13.sp, fontWeight = FontWeight.SemiBold)
                }
            }
            HorizontalDivider(color = gw.border)
            Box(Modifier.weight(1f).fillMaxWidth(), contentAlignment = Alignment.Center) {
                Column(
                    Modifier.widthIn(max = 560.dp).verticalScroll(rememberScrollState()).padding(Space.xl),
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.spacedBy(Space.xl),
                ) {
                    BrandWordmark(64.dp, Radius.lg)
                    Text(
                        "eID-ээрээ уншуулж нэвтэрнэ үү",
                        color = gw.fg1, fontSize = 34.sp, lineHeight = 39.sp,
                        fontWeight = FontWeight.Bold, textAlign = TextAlign.Center,
                    )
                    if (!rememberOnline()) OfflineNotice()
                    Surface(shape = RoundedCornerShape(Radius.xxl), color = ComposeColor.White) {
                        Box(Modifier.padding(Space.xl), contentAlignment = Alignment.Center) {
                            val link = waiting?.deviceLinkUrl
                            if (link != null) Image(
                                qrBitmap(link).asImageBitmap(), "eID QR",
                                Modifier.size(220.dp),
                            ) else Box(Modifier.size(220.dp), contentAlignment = Alignment.Center) {
                                Icon(Lucide.QrCode, contentDescription = null, tint = ComposeColor(0xFFC2CADA), modifier = Modifier.size(96.dp))
                            }
                        }
                    }
                    when (phase) {
                        is LoginPhase.Waiting -> Text(
                            "eID апп дээрх кодтой тулгана уу · ${phase.verificationCode}",
                            color = gw.fg2, fontSize = 15.sp, fontWeight = FontWeight.SemiBold,
                        )
                        LoginPhase.Success -> KioskStatus(Lucide.Check, gw.credit, "Баталгаажлаа — нэвтэрч байна")
                        LoginPhase.Expired -> KioskStatus(Lucide.Clock, gw.gold, "Хариу ирсэнгүй — код хүчингүй боллоо")
                        LoginPhase.Refused -> KioskStatus(Lucide.X, gw.debit, "eID апп дээр хүсэлтийг татгалзсан")
                        is LoginPhase.Error -> KioskStatus(Lucide.TriangleAlert, gw.debit, phase.message)
                        else -> Column(
                            Modifier.widthIn(max = 320.dp).fillMaxWidth(),
                            verticalArrangement = Arrangement.spacedBy(Space.md),
                        ) {
                            KioskStep(1, "eID апп-аа нээнэ")
                            KioskStep(2, "QR кодыг уншуулна")
                            KioskStep(3, "Апп дотроо баталгаажуулна")
                        }
                    }
                    LoadingPrimaryButton(
                        "eID QR үүсгэх",
                        isLoading = phase is LoginPhase.Starting,
                        isEnabled = phase !is LoginPhase.Starting,
                        height = 64.dp, fontSize = 20.sp, trailingIcon = Lucide.RefreshCw,
                    ) { auth.appToApp("") }
                    Text("QR код 2 минут хүчинтэй · Нууц үг дамжуулахгүй", color = gw.fg3, fontSize = 15.sp)
                }
            }
        }
    }
}

@Composable
private fun KioskStep(number: Int, text: String) {
    val gw = LocalGw.current
    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(Space.md)) {
        Box(Modifier.size(32.dp).background(gw.brandSoft, CircleShape), contentAlignment = Alignment.Center) {
            Text("$number", color = gw.brand, fontSize = 15.sp, fontWeight = FontWeight.SemiBold)
        }
        Text(text, color = gw.fg2, fontSize = 15.sp)
    }
}

@Composable
private fun KioskStatus(icon: androidx.compose.ui.graphics.vector.ImageVector, tint: ComposeColor, text: String) {
    val gw = LocalGw.current
    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(Space.md)) {
        Icon(icon, contentDescription = null, tint = tint, modifier = Modifier.size(20.dp))
        Text(text, color = gw.fg1, fontSize = 15.sp, fontWeight = FontWeight.SemiBold)
    }
}

internal fun qrBitmap(value: String, size: Int = 440): Bitmap {
    val matrix = QRCodeWriter().encode(value, BarcodeFormat.QR_CODE, size, size)
    return Bitmap.createBitmap(size, size, Bitmap.Config.RGB_565).apply {
        for (y in 0 until size) for (x in 0 until size) setPixel(x, y, if (matrix[x, y]) android.graphics.Color.BLACK else android.graphics.Color.WHITE)
    }
}
