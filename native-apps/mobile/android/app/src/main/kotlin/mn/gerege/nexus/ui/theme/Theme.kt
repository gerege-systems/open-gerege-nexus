package mn.gerege.nexus.ui.theme

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color

/**
 * eID клиентийн дизайны систем.
 *
 * Хоёр палитр зэрэгцэн явна:
 *
 * • `LocalGw` (`Gw.kt`) — **утасны харагдацын палитр**. Gerege Wallet-ийн
 *   токен багцаас гаралтай, iOS-ийн `Design/Theme.swift`-тэй 1:1. Дэлгэц бүр
 *   ҮҮНИЙГ уншина.
 * • `LocalEidColors` (`EidColors.kt`) — ширээний `Design/Colors.swift`-ээс
 *   ҮҮСГЭГДСЭН палитр. Гараар бүү зас; CI нь ширээ↔Android нийцлийг
 *   тэрүүгээр шалгадаг тул гинжийг таслахгүй, зөвхөн шинэ дэлгэц түүнийг
 *   уншихаа больсон.
 *
 * Хоёуланг нь өгч байгаа нь дундын байдал биш: ширээ, Windows-той нийцэл нь
 * үүсгэсэн файлын ажил, харин утасны харагдац нь Gw-гийнх. Хоёр асуулт, хоёр
 * хариулт.
 */
val LocalEidColors = staticCompositionLocalOf { EidLightColors }

@Composable
fun EidTheme(darkTheme: Boolean = isSystemInDarkTheme(), content: @Composable () -> Unit) {
    val colors = if (darkTheme) EidDarkColors else EidLightColors
    val gw = if (darkTheme) GwDark else GwLight

    CompositionLocalProvider(
        LocalEidColors provides colors,
        LocalGw provides gw,
    ) {
        MaterialTheme(
            colorScheme = if (darkTheme) {
                darkColorScheme(
                    primary = gw.brand,
                    onPrimary = Color.White,
                    primaryContainer = gw.brandDeep,
                    secondary = gw.credit,
                    background = gw.bg,
                    onBackground = gw.fg1,
                    surface = gw.surface1,
                    onSurface = gw.fg1,
                    surfaceVariant = gw.surface2,
                    onSurfaceVariant = gw.fg2,
                    outline = gw.border,
                    outlineVariant = gw.divider,
                    error = gw.debit,
                )
            } else {
                lightColorScheme(
                    primary = gw.brand,
                    onPrimary = Color.White,
                    primaryContainer = gw.brandDeep,
                    secondary = gw.credit,
                    background = gw.bg,
                    onBackground = gw.fg1,
                    surface = gw.surface1,
                    onSurface = gw.fg1,
                    surfaceVariant = gw.surface2,
                    onSurfaceVariant = gw.fg2,
                    outline = gw.border,
                    outlineVariant = gw.divider,
                    error = gw.debit,
                )
            },
            typography = GwTypography,
            content = content,
        )
    }
}
