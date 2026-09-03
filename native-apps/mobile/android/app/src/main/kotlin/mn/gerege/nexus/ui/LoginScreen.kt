package mn.gerege.nexus.ui

import android.content.Intent
import android.net.Uri
import androidx.compose.foundation.background
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsFocusedAsState
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Badge
import androidx.compose.material.icons.automirrored.filled.Launch
import androidx.compose.material.icons.filled.Verified
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.LocalTextStyle
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch
import mn.gerege.nexus.AppConfig
import mn.gerege.nexus.AppState
import mn.gerege.nexus.R
import mn.gerege.nexus.net.ApiClient
import mn.gerege.nexus.net.AuthResult
import mn.gerege.nexus.net.StoredIdentity
import mn.gerege.nexus.ui.components.*
import mn.gerege.nexus.ui.theme.LocalGw
import mn.gerege.nexus.ui.theme.Radius
import mn.gerege.nexus.ui.theme.Space

/**
 * Нэвтрэх дэлгэц — iOS-ийн `MobileLoginView`-тэй ЯГ ижил урсгал.
 *
 * Мак дээр QR/РД push-ыг ХӨРШ утас зөвшөөрдөг. Утсан дээр тэр зөвшөөрөгч нь
 * өөрөө байгаа тул session-ийг **app-to-app**-аар eID Mongolia апп руу
 * шилжүүлнэ: `geregesmartid://approve?sessionId=...`.
 *
 * Session нь ЭНЭ ПЛАТФОРМЫН RP-ээр үүснэ (`/api/v1/auth/eid/…`) — хөтөч дээрх
 * нэвтрэлт яг эдгээр route-уудыг дууддаг. Өмнө нь `/api/start` рүү явдаг
 * байсан бөгөөд nginx түүнийг eidmongolia.mn руу proxy хийдэг тул иргэн
 * утсан дээрээ ТЭДНИЙ demo RP-ийн нэрийг («RP Demo Bank») уншдаг байв.
 */
@Composable
fun LoginScreen(state: AppState) {
    val gw = LocalGw.current
    val context = LocalContext.current
    val scope = rememberCoroutineScope()

    var phase by remember { mutableStateOf("idle") }   // idle | starting | waiting | success
    var register by remember { mutableStateOf("") }
    var verificationCode by remember { mutableStateOf("") }
    var errorMessage by remember { mutableStateOf("") }
    var showRegister by remember { mutableStateOf(false) }
    var job by remember { mutableStateOf<Job?>(null) }
    val registerInteraction = remember { MutableInteractionSource() }
    val registerFocused by registerInteraction.collectIsFocusedAsState()

    val registerTyped = register.trim()
    val registerValid = registerTyped.length >= 8

    // eID апп суусан эсэх. `<queries>` блокгүй бол Android 11+ дээр үргэлж
    // null буцаана — тэр тохиолдолд РД push зам нээлттэй тул апп гацахгүй.
    fun eidAppIntent(sessionId: String): Intent? =
        listOf("geregesmartid", "eidmongolia").firstNotNullOfOrNull { scheme ->
            val intent = Intent(Intent.ACTION_VIEW, Uri.parse("$scheme://approve?sessionId=$sessionId"))
            if (intent.resolveActivity(context.packageManager) != null) intent else null
        }

    fun finish(result: AuthResult, typedId: String) {
        val person = result.identity
        if (!result.isComplete || person == null || !person.verified) {
            errorMessage = when (result.state) {
                "EXPIRED" -> context.getString(R.string.Login_Error_Expired)
                "REFUSED" -> context.getString(R.string.Login_Error_Refused)
                else -> context.getString(R.string.Login_Error_Failed)
            }
            phase = "idle"
            return
        }
        // Нэр нь платформоос МОНГОЛООР ирнэ — латин галигийг нөхөх нэмэлт
        // дуудлага (`/api/dashboard`) хэрэггүй болов.
        val identity = StoredIdentity(
            // Платформ нь eID-ийн `documentNumber`-ыг дээшээ гаргадаггүй.
            // Гэрчилгээний сериал нь иргэн ЯМАР гэрчилгээгээр зөвшөөрснийг
            // заадаг цорын ганц бариул тул ID хуудсанд түүнийг харуулна.
            documentNumber = person.certificateSerial.orEmpty(),
            fullName = person.mongolianName.orEmpty(),
            civilId = person.civilId.orEmpty(),
            nationalId = person.regNumber ?: typedId,
            // Нэвтрэлтийн доод хязгаар нь ADVANCED (`EID_CERT_LEVEL`); QUALIFIED
            // гэж бичих нь баталгаагүй зүйлийг батласан болно.
            certificateLevel = "ADVANCED",
            loginAt = AppState.isoNow(),
        )
        phase = "success"
        state.didLogin(identity)
    }

    fun run(start: suspend () -> Pair<String, String>) {
        job?.cancel()
        errorMessage = ""
        phase = "starting"
        job = scope.launch {
            runCatching {
                val (sessionId, typedId) = start()
                phase = "waiting"
                finish(ApiClient.waitForPlatformAuth(sessionId), typedId)
            }.onFailure {
                errorMessage = it.message ?: context.getString(R.string.Login_Error_Failed)
                phase = "idle"
            }
        }
    }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(gw.bg)
            .verticalScroll(rememberScrollState())
            .padding(horizontal = Space.xl, vertical = Space.xxl),
        horizontalAlignment = Alignment.CenterHorizontally,
        // Богино агуулгыг ГОЛЛУУЛНА: `fillMaxSize` нь доод хязгаарыг харагдах
        // өндөрт барьдаг тул `CenterVertically` ажиллана, гар гарч ирэхэд
        // `verticalScroll` нь агуулгыг гүйлгэнэ. iOS дээрх дүрэмтэй ижил.
        verticalArrangement = Arrangement.spacedBy(Space.xl, Alignment.CenterVertically),
    ) {
        Box(
            modifier = Modifier
                .size(76.dp)
                .background(Brush.linearGradient(listOf(gw.brand, gw.brandDeep)),
                            RoundedCornerShape(22.dp)),
            contentAlignment = Alignment.Center,
        ) {
            Icon(Icons.Filled.Badge, null, tint = Color.White, modifier = Modifier.size(34.dp))
        }
        Text(AppConfig.BRAND_NAME, style = MaterialTheme.typography.headlineLarge, color = gw.fg1)
        Text(stringResource(R.string.Login_Subtitle),
             style = MaterialTheme.typography.bodySmall, color = gw.fg3, textAlign = TextAlign.Center)

        EidCard(spacing = Space.lg) {
            when (phase) {
                "waiting", "starting" -> {
                    Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.Center) {
                        CircularProgressIndicator(color = gw.brand, strokeWidth = 2.dp,
                                                  modifier = Modifier.size(28.dp))
                    }
                    Text(
                        stringResource(if (phase == "waiting") R.string.Login_Waiting_Subtitle
                                       else R.string.Login_Initiate_Loading),
                        style = MaterialTheme.typography.bodySmall, color = gw.fg2,
                        textAlign = TextAlign.Center, modifier = Modifier.fillMaxWidth(),
                    )
                    if (verificationCode.isNotEmpty()) {
                        BrandSectionLabel(stringResource(R.string.Login_VerificationCode),
                                          Modifier.align(Alignment.CenterHorizontally))
                        BrandCodeRow(verificationCode, Modifier.align(Alignment.CenterHorizontally))
                    }
                    BrandLinkButton(stringResource(R.string.Login_Cancel),
                                    Modifier.align(Alignment.CenterHorizontally)) {
                        job?.cancel(); phase = "idle"
                    }
                }

                "success" -> Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(Space.sm, Alignment.CenterHorizontally),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Icon(Icons.Filled.Verified, null, tint = gw.credit, modifier = Modifier.size(22.dp))
                    Text(stringResource(R.string.Login_Success_Title),
                         style = MaterialTheme.typography.titleMedium, color = gw.fg1)
                }

                else -> {
                    BrandInfoBanner(stringResource(R.string.Login_AppToApp_Hint))

                    LoadingPrimaryButton(
                        label = stringResource(R.string.Login_AppToApp),
                        leadingIcon = Icons.AutoMirrored.Filled.Launch,
                    ) {
                        run {
                            val session = ApiClient.authStart()
                            verificationCode = session.vc.orEmpty()
                            val intent = eidAppIntent(session.sessionId)
                            if (intent == null) {
                                showRegister = true
                                errorMessage = context.getString(R.string.Login_Error_AppMissing)
                            } else {
                                context.startActivity(intent)
                            }
                            session.sessionId to ""
                        }
                    }

                    if (showRegister) {
                        BrandSectionLabel(stringResource(R.string.Login_NationalId))
                        BrandInputCard(
                            leadingIcon = Icons.Filled.Badge,
                            // Хоосон талбар дээр улаан ✗ анивчуулах нь бичиж
                            // эхлээгүй хүнийг буруутгаж байгаа хэрэг.
                            validation = if (registerTyped.isEmpty()) null else BrandValidationState(
                                label = stringResource(
                                    if (registerValid) R.string.Common_Valid else R.string.Common_TooShort),
                                valid = registerValid,
                            ),
                            isFocused = registerFocused,
                        ) {
                            BasicTextField(
                                value = register,
                                onValueChange = { register = it.uppercase() },
                                singleLine = true,
                                interactionSource = registerInteraction,
                                keyboardOptions = KeyboardOptions(
                                    capitalization = KeyboardCapitalization.Characters),
                                textStyle = LocalTextStyle.current.copy(
                                    fontFamily = FontFamily.Monospace, color = gw.fg1),
                                cursorBrush = SolidColor(gw.brand),
                                modifier = Modifier.fillMaxWidth(),
                                decorationBox = { inner ->
                                    if (register.isEmpty()) {
                                        Text(stringResource(R.string.Login_NationalId_Placeholder),
                                             style = MaterialTheme.typography.bodyMedium.copy(
                                                 fontFamily = FontFamily.Monospace),
                                             color = gw.fg4)
                                    }
                                    inner()
                                },
                            )
                        }
                        SecondaryButton(stringResource(R.string.Login_Push),
                                        enabled = registerValid, tone = gw.brand) {
                            run {
                                val session = ApiClient.authStartById(registerTyped)
                                verificationCode = session.vc.orEmpty()
                                session.sessionId to registerTyped
                            }
                        }
                    } else {
                        BrandLinkButton(stringResource(R.string.Login_OtherDevice),
                                        Modifier.align(Alignment.CenterHorizontally)) {
                            showRegister = true
                        }
                    }
                }
            }
        }

        if (errorMessage.isNotEmpty()) InlineBanner(errorMessage)

        Column(
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(Space.sm),
        ) {
            BrandSecurityFooter(stringResource(R.string.Login_SecurityFooter))
            Text("${AppConfig.host}  ·  ${AppConfig.BRAND_NAME} v1.0.0",
                 style = MaterialTheme.typography.labelSmall.copy(
                     fontFamily = FontFamily.Monospace, fontWeight = FontWeight.Normal),
                 color = gw.fg4)
        }
    }
}
