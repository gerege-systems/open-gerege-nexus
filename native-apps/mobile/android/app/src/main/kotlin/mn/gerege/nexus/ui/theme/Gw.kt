// Утасны дизайны токенууд — iOS-ийн `mobile/ios/Design/Theme.swift`-ийн ХОСОЛ.
//
// Эх сурвалж: Gerege Wallet-ийн «Real App Mirror» токен багц. Гурван гэр бүл:
//   • neutral:  bg, surface1..3, fg1..4, border, borderStrong, divider
//   • brand:    brand, brandSoft, brandLine, brandDeep, brandGlow
//   • semantic: credit (монгол тугны ногоон), debit, accent (улбар шар), gold
//
// **Энэ файл ҮҮСГЭГДДЭГГҮЙ.** Хажуугийн `EidColors.kt` нь ширээний
// `Design/Colors.swift`-ээс `scripts/gen_from_swift.py`-ээр үүсдэг бөгөөд тэр
// гинж ХЭВЭЭР — CI нь ширээ↔Android нийцлийг тэрүүгээр шалгадаг. Утасны
// харагдац тэр шалгалтыг эвдэх ёсгүй тул шинэ палитр энд, гараар амьдарна.
//
// Токены нэрс нь iOS талын `Theme.*`-тэй 1:1: дизайны утга нүүлгэх нь
// механик ажил байх ёстой, орчуулга биш.

package mn.gerege.nexus.ui.theme

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.Typography
import androidx.compose.runtime.Composable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import mn.gerege.nexus.R

// MARK: - Брэнд ба семантик (горимоос үл хамаарна) ------------------------

val GwBrand     = Color(0xFF3178EC)
val GwBrandDeep = Color(0xFF1F5FD0)
val GwBrandGlow = Color(0x593178EC)   // 35%

val GwCredit    = Color(0xFF119D52)   // монгол тугны ногоон — «болсон»
val GwDebitDark = Color(0xFFF6465D)
val GwDebitLite = Color(0xFFE03A50)
val GwAccent    = Color(0xFFFE6401)   // сонгогдсон таб, анхааруулга
val GwGold      = Color(0xFFE0A82E)

// MARK: - Горим бүрийн палитр ---------------------------------------------

data class GwPalette(
    val bg: Color,
    val surface1: Color,
    val surface2: Color,
    val surface3: Color,
    val fg1: Color,
    val fg2: Color,
    val fg3: Color,
    val fg4: Color,
    val border: Color,
    val borderStrong: Color,
    val divider: Color,
    val brandSoft: Color,
    val brandLine: Color,
    val credit: Color = GwCredit,
    val creditSoft: Color,
    val debit: Color,
    val debitSoft: Color,
    val accent: Color = GwAccent,
    val accentSoft: Color,
    val gold: Color = GwGold,
    val goldSoft: Color,
    val brand: Color = GwBrand,
    val brandDeep: Color = GwBrandDeep,
    val onBrand: Color = Color.White,
)

val GwLight = GwPalette(
    bg           = Color(0xFFF4F6FB),
    surface1     = Color(0xFFFFFFFF),
    surface2     = Color(0xFFF4F6FB),
    surface3     = Color(0xFFE9EDF5),
    fg1          = Color(0xFF0E1525),
    fg2          = Color(0xFF4A5468),
    fg3          = Color(0xFF8892A6),
    fg4          = Color(0xFFC2CADA),
    border       = Color(0x140E1525),   // 8%
    borderStrong = Color(0x290E1525),   // 16%
    divider      = Color(0x0F0E1525),   // 6%
    brandSoft    = Color(0x1A3178EC),   // 10%
    brandLine    = Color(0x663178EC),   // 40%
    creditSoft   = Color(0x1A119D52),
    debit        = GwDebitLite,
    debitSoft    = Color(0x1AE03A50),
    accentSoft   = Color(0x1AFE6401),
    goldSoft     = Color(0x1AE0A82E),
)

val GwDark = GwPalette(
    bg           = Color(0xFF0B0E11),
    surface1     = Color(0xFF12161C),
    surface2     = Color(0xFF1A1F27),
    surface3     = Color(0xFF232A34),
    fg1          = Color(0xFFEAECEF),
    fg2          = Color(0xFFB7BDC6),
    fg3          = Color(0xFF707A8A),
    fg4          = Color(0xFF4B5563),
    border       = Color(0x0FFFFFFF),   // 6%
    borderStrong = Color(0x1FFFFFFF),   // 12%
    divider      = Color(0x0AFFFFFF),   // 4%
    brandSoft    = Color(0x243178EC),   // 14%
    brandLine    = Color(0x663178EC),
    creditSoft   = Color(0x24119D52),
    debit        = GwDebitDark,
    debitSoft    = Color(0x1FF6465D),
    accentSoft   = Color(0x24FE6401),
    goldSoft     = Color(0x24E0A82E),
)

val LocalGw = staticCompositionLocalOf { GwLight }

@Composable
fun gwPalette(dark: Boolean = isSystemInDarkTheme()): GwPalette = if (dark) GwDark else GwLight

// MARK: - Зай, радиус ------------------------------------------------------
//
// iOS-ийн `Theme.Space` / `Theme.Radius`-тай ижил утга. 4dp суурь, ~25% алхам.

object Space {
    val xxs  = 2.dp
    val xs   = 4.dp
    val sm   = 8.dp
    val md   = 12.dp
    val lg   = 16.dp
    val xl   = 24.dp
    val xxl  = 32.dp
    val xxxl = 48.dp
}

object Radius {
    val sm  = 8.dp
    val md  = 12.dp
    val lg  = 16.dp
    val xl  = 20.dp
    val xxl = 24.dp
}

// MARK: - Фонт -------------------------------------------------------------
//
// Montserrat нь `res/font/`-д — iOS-ийн багцалсан ЯГ ТЭР дөрвөн .ttf. Тоо
// (регистр, баримтын дугаар, код) нь `FontFamily.Monospace` хэвээр: Montserrat
// tabular figure-гүй тул баганаар эгнэхгүй.

val Montserrat = FontFamily(
    Font(R.font.montserrat_regular,  FontWeight.Normal),
    Font(R.font.montserrat_medium,   FontWeight.Medium),
    Font(R.font.montserrat_semibold, FontWeight.SemiBold),
    Font(R.font.montserrat_bold,     FontWeight.Bold),
)

/** iOS-ийн `Theme.TypeScale`-тай ижил хэмжээс, ижил жин. */
val GwTypography = Typography(
    displayLarge   = TextStyle(fontFamily = Montserrat, fontWeight = FontWeight.Bold,     fontSize = 34.sp),
    headlineLarge  = TextStyle(fontFamily = Montserrat, fontWeight = FontWeight.Bold,     fontSize = 28.sp),
    headlineMedium = TextStyle(fontFamily = Montserrat, fontWeight = FontWeight.Bold,     fontSize = 22.sp),
    headlineSmall  = TextStyle(fontFamily = Montserrat, fontWeight = FontWeight.SemiBold, fontSize = 20.sp),
    titleLarge     = TextStyle(fontFamily = Montserrat, fontWeight = FontWeight.SemiBold, fontSize = 20.sp),
    titleMedium    = TextStyle(fontFamily = Montserrat, fontWeight = FontWeight.SemiBold, fontSize = 17.sp),
    bodyLarge      = TextStyle(fontFamily = Montserrat, fontWeight = FontWeight.Normal,   fontSize = 15.sp),
    bodyMedium     = TextStyle(fontFamily = Montserrat, fontWeight = FontWeight.Normal,   fontSize = 14.sp),
    bodySmall      = TextStyle(fontFamily = Montserrat, fontWeight = FontWeight.Normal,   fontSize = 13.sp),
    labelLarge     = TextStyle(fontFamily = Montserrat, fontWeight = FontWeight.SemiBold, fontSize = 15.sp),
    labelMedium    = TextStyle(fontFamily = Montserrat, fontWeight = FontWeight.SemiBold, fontSize = 12.sp),
    labelSmall     = TextStyle(fontFamily = Montserrat, fontWeight = FontWeight.SemiBold, fontSize = 11.sp),
)
