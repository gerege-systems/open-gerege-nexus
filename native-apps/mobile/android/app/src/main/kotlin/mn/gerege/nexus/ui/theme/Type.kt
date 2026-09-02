// The wallet's type scale. Their file names Montserrat and falls back to the
// platform family while the .ttf assets are absent; this client has no font
// assets of its own, so the fallback is what runs — the sizes and weights are
// what carry the design, and they are theirs exactly.

package mn.gerege.nexus.ui.theme

import androidx.compose.material3.Typography
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp

private val BrandSans: FontFamily = FontFamily.Default

val NexusTypography = Typography(
    displayLarge = TextStyle(fontFamily = BrandSans, fontWeight = FontWeight.Bold, fontSize = 34.sp),
    headlineLarge = TextStyle(fontFamily = BrandSans, fontWeight = FontWeight.Bold, fontSize = 28.sp),
    headlineMedium = TextStyle(fontFamily = BrandSans, fontWeight = FontWeight.Bold, fontSize = 22.sp),
    titleLarge = TextStyle(fontFamily = BrandSans, fontWeight = FontWeight.SemiBold, fontSize = 20.sp),
    titleMedium = TextStyle(fontFamily = BrandSans, fontWeight = FontWeight.SemiBold, fontSize = 17.sp),
    bodyLarge = TextStyle(fontFamily = BrandSans, fontWeight = FontWeight.Normal, fontSize = 17.sp),
    bodyMedium = TextStyle(fontFamily = BrandSans, fontWeight = FontWeight.Normal, fontSize = 15.sp),
    labelLarge = TextStyle(fontFamily = BrandSans, fontWeight = FontWeight.SemiBold, fontSize = 16.sp),
    labelMedium = TextStyle(fontFamily = BrandSans, fontWeight = FontWeight.SemiBold, fontSize = 13.sp),
    labelSmall = TextStyle(fontFamily = BrandSans, fontWeight = FontWeight.SemiBold, fontSize = 12.sp),
)
