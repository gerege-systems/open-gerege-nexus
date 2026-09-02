// The wallet's design tokens, for the Android client.
//
// Ported from gerege-line/gerege-core/wallet-gerege-mn — android/app/.../
// ui/theme/Color.kt. Both apps are Compose, so this is a straight copy: the
// names mirror the CSS variables of the prototype they came from, one for one,
// which is what makes looking a value up mechanical rather than a hunt.
//
// Three families: neutrals (bg, surface1..3, fg1..4, border, divider), brand,
// and semantic (credit — the Mongolian-flag green — debit, accent, gold).
//
// Note this is a different palette from the one the desktop clients wear:
// those took the eID platform's ramp (#3A6DFF, light-first), this is the
// wallet's (#3178EC, dark-first). Two product lines, two identities. A change
// to one is not a change to the other.

package mn.gerege.nexus.ui.theme

import androidx.compose.ui.graphics.Color

// Brand, shared across modes
val Brand = Color(0xFF3178EC)
val BrandDeep = Color(0xFF1F5FD0)
val BrandGlow = Color(0x593178EC) // 35% alpha

// Semantic foundation
val Credit = Color(0xFF119D52)
val CreditDeep = Color(0xFF0E7E42)
val DebitDark = Color(0xFFF6465D)
val DebitLight = Color(0xFFE03A50)
val Accent = Color(0xFFFE6401)
val AccentDeep = Color(0xFFD85400)
val GoldFlag = Color(0xFFE0A82E)

/** Dark is the default here, as it is in the wallet. */
object DarkPalette {
    val bg = Color(0xFF0B0E11)
    val surface1 = Color(0xFF12161C)
    val surface2 = Color(0xFF1A1F27)
    val surface3 = Color(0xFF232A34)
    val fg1 = Color(0xFFEAECEF)
    val fg2 = Color(0xFFB7BDC6)
    val fg3 = Color(0xFF707A8A)
    val fg4 = Color(0xFF4B5563)
    val border = Color(0x0FFFFFFF)       // rgba(255,255,255,0.06)
    val borderStrong = Color(0x1FFFFFFF) // 0.12
    val divider = Color(0x0AFFFFFF)      // 0.04
    val brandSoft = Color(0x243178EC)    // rgba(49,120,236,0.14)
    val brandLine = Color(0x663178EC)    // 0.40
    val creditSoft = Color(0x24119D52)
    val debitSoft = Color(0x1FF6465D)
    val accentSoft = Color(0x24FE6401)
    val goldSoft = Color(0x24E0A82E)
}

object LightPalette {
    val bg = Color(0xFFF4F6FB)
    val surface1 = Color(0xFFFFFFFF)
    val surface2 = Color(0xFFF4F6FB)
    val surface3 = Color(0xFFE9EDF5)
    val fg1 = Color(0xFF0E1525)
    val fg2 = Color(0xFF4A5468)
    val fg3 = Color(0xFF8892A6)
    val fg4 = Color(0xFFC2CADA)
    val border = Color(0x140E1525)       // rgba(14,21,37,0.08)
    val borderStrong = Color(0x290E1525) // 0.16
    val divider = Color(0x0F0E1525)      // 0.06
    val brandSoft = Color(0x1A3178EC)    // 0.10 in light
    val brandLine = Color(0x663178EC)
    val creditSoft = Color(0x1A119D52)
    val debitSoft = Color(0x1AE03A50)
    val accentSoft = Color(0x1AFE6401)
    val goldSoft = Color(0x1AE0A82E)
}
