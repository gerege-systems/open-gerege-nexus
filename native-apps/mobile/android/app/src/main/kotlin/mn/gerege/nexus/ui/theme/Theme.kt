// The wallet's Compose theme, ported whole.
//
// Two layers, as they have it. `MaterialTheme` carries what Material's own
// components read, and `LocalGw` carries the semantic palette those components
// have no slot for — surface2, fg3, brandSoft, the credit/debit pair. A screen
// reaches for whichever answers its question; neither is a shadow of the other.

package mn.gerege.nexus.ui.theme

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color

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
    val brand: Color = Brand,
    val brandSoft: Color,
    val brandLine: Color,
    val brandDeep: Color = BrandDeep,
    val credit: Color = Credit,
    val creditSoft: Color,
    val debit: Color,
    val debitSoft: Color,
    val accent: Color = Accent,
    val accentSoft: Color,
    val gold: Color = GoldFlag,
    val goldSoft: Color,
    val onBrand: Color = Color.White,
)

private val DarkGw = GwPalette(
    bg = DarkPalette.bg,
    surface1 = DarkPalette.surface1,
    surface2 = DarkPalette.surface2,
    surface3 = DarkPalette.surface3,
    fg1 = DarkPalette.fg1,
    fg2 = DarkPalette.fg2,
    fg3 = DarkPalette.fg3,
    fg4 = DarkPalette.fg4,
    border = DarkPalette.border,
    borderStrong = DarkPalette.borderStrong,
    divider = DarkPalette.divider,
    brandSoft = DarkPalette.brandSoft,
    brandLine = DarkPalette.brandLine,
    creditSoft = DarkPalette.creditSoft,
    debit = DebitDark,
    debitSoft = DarkPalette.debitSoft,
    accentSoft = DarkPalette.accentSoft,
    goldSoft = DarkPalette.goldSoft,
)

private val LightGw = GwPalette(
    bg = LightPalette.bg,
    surface1 = LightPalette.surface1,
    surface2 = LightPalette.surface2,
    surface3 = LightPalette.surface3,
    fg1 = LightPalette.fg1,
    fg2 = LightPalette.fg2,
    fg3 = LightPalette.fg3,
    fg4 = LightPalette.fg4,
    border = LightPalette.border,
    borderStrong = LightPalette.borderStrong,
    divider = LightPalette.divider,
    brandSoft = LightPalette.brandSoft,
    brandLine = LightPalette.brandLine,
    creditSoft = LightPalette.creditSoft,
    debit = DebitLight,
    debitSoft = LightPalette.debitSoft,
    accentSoft = LightPalette.accentSoft,
    goldSoft = LightPalette.goldSoft,
)

/** Dark is the default, as it is in the wallet. */
val LocalGw = staticCompositionLocalOf { DarkGw }

private val DarkScheme = darkColorScheme(
    primary = Brand,
    onPrimary = Color.White,
    primaryContainer = BrandDeep,
    secondary = Credit,
    onSecondary = Color.White,
    background = DarkPalette.bg,
    onBackground = DarkPalette.fg1,
    surface = DarkPalette.surface1,
    onSurface = DarkPalette.fg1,
    surfaceVariant = DarkPalette.surface2,
    onSurfaceVariant = DarkPalette.fg2,
    error = DebitDark,
    outline = DarkPalette.border,
    outlineVariant = DarkPalette.divider,
)

private val LightScheme = lightColorScheme(
    primary = Brand,
    onPrimary = Color.White,
    primaryContainer = BrandDeep,
    secondary = Credit,
    onSecondary = Color.White,
    background = LightPalette.bg,
    onBackground = LightPalette.fg1,
    surface = LightPalette.surface1,
    onSurface = LightPalette.fg1,
    surfaceVariant = LightPalette.surface2,
    onSurfaceVariant = LightPalette.fg2,
    error = DebitLight,
    outline = LightPalette.border,
    outlineVariant = LightPalette.divider,
)

@Composable
fun GeregeNexusTheme(
    darkTheme: Boolean = isSystemInDarkTheme(),
    content: @Composable () -> Unit,
) {
    val gw = if (darkTheme) DarkGw else LightGw
    CompositionLocalProvider(LocalGw provides gw) {
        MaterialTheme(
            colorScheme = if (darkTheme) DarkScheme else LightScheme,
            typography = NexusTypography,
            content = content,
        )
    }
}
