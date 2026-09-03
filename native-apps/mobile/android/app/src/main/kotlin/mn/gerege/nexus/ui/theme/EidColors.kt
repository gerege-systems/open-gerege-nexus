// ҮҮСГЭСЭН ФАЙЛ — scripts/gen_from_swift.py. Гараар бүү зас.
// Эх сурвалж: native-apps/desktop/macos/Design/Colors.swift

package mn.gerege.nexus.ui.theme

import androidx.compose.ui.graphics.Color

/** Горимоос үл хамаарах брэндийн ramp. */
val brand50 = Color(0xFFEEF4FF)
val brand100 = Color(0xFFD8E6FF)
val brand200 = Color(0xFFB3CCFF)
val brand300 = Color(0xFF84A8FF)
val brand400 = Color(0xFF5B8AFF)
val brand500 = Color(0xFF3A6DFF)
val brand600 = Color(0xFF2A5BD7)
val brand700 = Color(0xFF1D47B3)
val brand800 = Color(0xFF14358A)
val brand900 = Color(0xFF0A2466)
val brand950 = Color(0xFF001F5E)
val bannerErrorBG = Color(0xFFFEE2E2)
val bannerErrorBorder = Color(0xFFFCA5A5)
val bannerErrorText = Color(0xFF7F1D1D)
val bannerErrorIcon = Color(0xFFB91C1C)
val bannerSuccessBG = Color(0xFFDCFCE7)
val bannerSuccessIcon = Color(0xFF16A34A)
val accentGold = Color(0xFFD4A017)
val warningBG = Color(0xFFFEF3C7)
val sidebarBackground = Color(0xFF0F172A)
val sidebarHover = Color(0xFF1E293B)
val sidebarMutedText = Color(0xFF94A3B8)

/** Гэрэл/харанхуйд өөр өөр утгатай токенууд. */
data class EidColors(
    val eidAccent: Color,
    val eidAccentStrong: Color,
    val eidAccentSubtle: Color,
    val eidAccentMuted: Color,
    val eidSuccess: Color,
    val eidWarning: Color,
    val eidDanger: Color,
    val eidCardBackground: Color,
    val eidCardStroke: Color,
    val eidSurface: Color,
    val eidMuted: Color,
    val backgroundSecondary: Color,
    val textPrimary: Color,
)

val EidLightColors = EidColors(
    eidAccent = Color(0xFF3A6DFF),
    eidAccentStrong = Color(0xFF1D47B3),
    eidAccentSubtle = Color(0xFFD8E6FF),
    eidAccentMuted = Color(0xFFEEF4FF),
    eidSuccess = Color(0xFF107C10),
    eidWarning = Color(0xFF9D5D00),
    eidDanger = Color(0xFFBA1A1A),
    eidCardBackground = Color(0xFFFFFFFF),
    eidCardStroke = Color(0xFFE5E7EB),
    eidSurface = Color(0xFFF9F9FC),
    eidMuted = Color(0xFF6B7280),
    backgroundSecondary = Color(0xFFF1F5F9),
    textPrimary = Color(0xFF0F172A),
)

val EidDarkColors = EidColors(
    eidAccent = Color(0xFF84A8FF),
    eidAccentStrong = Color(0xFFB3CCFF),
    eidAccentSubtle = Color(0xFF0A2466),
    eidAccentMuted = Color(0xFF001F5E),
    eidSuccess = Color(0xFF6FCF6F),
    eidWarning = Color(0xFFE8B66A),
    eidDanger = Color(0xFFFFB4AB),
    eidCardBackground = Color(0xFF1F1F23),
    eidCardStroke = Color(0xFF34343A),
    eidSurface = Color(0xFF0B0E12),
    eidMuted = Color(0xFF9CA3AF),
    backgroundSecondary = Color(0xFF111827),
    textPrimary = Color(0xFFF8FAFC),
)
