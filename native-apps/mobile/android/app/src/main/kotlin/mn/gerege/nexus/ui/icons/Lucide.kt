// The icon set the design handoff names: Lucide, stroke 1.75, 24×24 viewport.
//
// Drawn here rather than imported, for the same reason BrandComponents draws
// its arrow: the compose material-icons artifacts stopped shipping beside
// current BOMs, and a community Lucide port would be a dependency for twenty
// glyphs. Path data is Lucide's own (ISC licensed), embedded as SVG path
// strings and parsed once at class-load. Icon's `tint` colours the stroke, so
// the white here is only what an untinted render would use.
//
// design/README.md § Assets lists which glyph goes where.

package mn.gerege.nexus.ui.icons

import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.graphics.vector.PathParser
import androidx.compose.ui.unit.dp

private fun lucide(name: String, vararg paths: String): ImageVector {
    val builder = ImageVector.Builder(
        name = name,
        defaultWidth = 24.dp,
        defaultHeight = 24.dp,
        viewportWidth = 24f,
        viewportHeight = 24f,
    )
    paths.forEach { data ->
        builder.addPath(
            pathData = PathParser().parsePathString(data).toNodes(),
            fill = null,
            stroke = SolidColor(Color.White),
            strokeLineWidth = 1.75f,
            strokeLineCap = StrokeCap.Round,
            strokeLineJoin = StrokeJoin.Round,
        )
    }
    return builder.build()
}

object Lucide {
    val ShieldCheck = lucide(
        "shield-check",
        "M20 13c0 5-3.5 7.5-7.66 8.95a1 1 0 0 1-.67 0C7.5 20.5 4 18 4 13V6a1 1 0 0 1 1-1c2 0 4.5-1.2 6.24-2.72a1.17 1.17 0 0 1 1.52 0C14.51 3.81 17 5 19 5a1 1 0 0 1 1 1z",
        "m9 12 2 2 4-4",
    )
    val Smartphone = lucide(
        "smartphone",
        "M7 2h10a2 2 0 0 1 2 2v16a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2z",
        "M12 18h.01",
    )
    val Lock = lucide(
        "lock",
        "M5 11h14a2 2 0 0 1 2 2v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-7a2 2 0 0 1 2-2z",
        "M7 11V7a5 5 0 0 1 10 0v4",
    )
    val ArrowRight = lucide("arrow-right", "M5 12h14", "m12 5 7 7-7 7")
    val Check = lucide("check", "M20 6 9 17l-5-5")
    val X = lucide("x", "M18 6 6 18", "m6 6 12 12")
    val Clock = lucide(
        "clock",
        "M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20z",
        "M12 6v6l4 2",
    )
    val TriangleAlert = lucide(
        "triangle-alert",
        "m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3",
        "M12 9v4",
        "M12 17h.01",
    )
    val RefreshCw = lucide(
        "refresh-cw",
        "M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8",
        "M21 3v5h-5",
        "M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16",
        "M8 16H3v5",
    )
    val LayoutGrid = lucide(
        "layout-grid",
        "M4 3h5a1 1 0 0 1 1 1v5a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1z",
        "M15 3h5a1 1 0 0 1 1 1v5a1 1 0 0 1-1 1h-5a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1z",
        "M15 14h5a1 1 0 0 1 1 1v5a1 1 0 0 1-1 1h-5a1 1 0 0 1-1-1v-5a1 1 0 0 1 1-1z",
        "M4 14h5a1 1 0 0 1 1 1v5a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1v-5a1 1 0 0 1 1-1z",
    )
    val FileText = lucide(
        "file-text",
        "M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7z",
        "M14 2v4a2 2 0 0 0 2 2h4",
        "M16 13H8",
        "M16 17H8",
        "M10 9H8",
    )
    val ChartBar = lucide("chart-bar", "M12 20V10", "M18 20V4", "M6 20v-4")
    val Sliders = lucide(
        "sliders-horizontal",
        "M21 4h-7", "M10 4H3", "M21 12h-9", "M8 12H3", "M21 20h-5", "M12 20H3",
        "M14 2v4", "M8 10v4", "M16 18v4",
    )
    val Server = lucide(
        "server",
        "M4 2h16a2 2 0 0 1 2 2v4a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2z",
        "M4 14h16a2 2 0 0 1 2 2v4a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2v-4a2 2 0 0 1 2-2z",
        "M6 6h.01",
        "M6 18h.01",
    )
    val MapPin = lucide(
        "map-pin",
        "M20 10c0 4.993-5.539 10.193-7.399 11.799a1 1 0 0 1-1.202 0C9.539 20.193 4 14.993 4 10a8 8 0 0 1 16 0",
        "M12 7a3 3 0 1 0 0 6 3 3 0 0 0 0-6z",
    )
    val QrCode = lucide(
        "qr-code",
        "M4 3h3a1 1 0 0 1 1 1v3a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1z",
        "M17 3h3a1 1 0 0 1 1 1v3a1 1 0 0 1-1 1h-3a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1z",
        "M4 16h3a1 1 0 0 1 1 1v3a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1v-3a1 1 0 0 1 1-1z",
        "M21 16h-3a2 2 0 0 0-2 2v3",
        "M21 21v.01",
        "M12 7v3a2 2 0 0 1-2 2H7",
        "M3 12h.01",
        "M12 3h.01",
        "M12 16v.01",
        "M16 12h1",
        "M21 12v.01",
        "M12 21v-1",
    )
    val Printer = lucide(
        "printer",
        "M6 9V3a1 1 0 0 1 1-1h10a1 1 0 0 1 1 1v6",
        "M6 18H4a2 2 0 0 1-2-2v-5a2 2 0 0 1 2-2h16a2 2 0 0 1 2 2v5a2 2 0 0 1-2 2h-2",
        "M7 14h10a1 1 0 0 1 1 1v6a1 1 0 0 1-1 1H7a1 1 0 0 1-1-1v-6a1 1 0 0 1 1-1z",
    )
    val Activity = lucide(
        "activity",
        "M22 12h-2.48a2 2 0 0 0-1.93 1.46l-2.35 8.36a.25.25 0 0 1-.48 0L9.24 2.18a.25.25 0 0 0-.48 0l-2.35 8.36A2 2 0 0 1 4.49 12H2",
    )
    val Moon = lucide("moon", "M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9z")
    val Sun = lucide(
        "sun",
        "M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8z",
        "M12 2v2", "M12 20v2", "m4.93 4.93 1.41 1.41", "m17.66 17.66 1.41 1.41",
        "M2 12h2", "M20 12h2", "m6.34 17.66-1.41 1.41", "m19.07 4.93-1.41 1.41",
    )
    val Monitor = lucide(
        "monitor",
        "M4 3h16a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2z",
        "M8 21h8",
        "M12 17v4",
    )
    val ChevronRight = lucide("chevron-right", "m9 18 6-6-6-6")
    val WifiOff = lucide(
        "wifi-off",
        "M12 20h.01",
        "M8.5 16.429a5 5 0 0 1 7 0",
        "M5 12.859a10 10 0 0 1 5.17-2.69",
        "M19 12.859a10 10 0 0 0-2.007-1.523",
        "M2 8.82a15 15 0 0 1 4.177-2.643",
        "M22 8.82a15 15 0 0 0-11.288-3.764",
        "m2 2 20 20",
    )
    val Delete = lucide(
        "delete",
        "M20 5H9l-7 7 7 7h11a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2z",
        "m12 9 6 6",
        "m18 9-6 6",
    )
}
