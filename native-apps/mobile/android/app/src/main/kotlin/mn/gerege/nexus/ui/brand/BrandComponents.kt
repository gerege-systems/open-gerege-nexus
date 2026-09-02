// The wallet's brand components, ported.
//
// From gerege-line/gerege-core/wallet-gerege-mn — android/app/.../ui/brand/
// BrandComponents.kt. Their measurements are kept exactly: the 52dp input
// card, its 14dp corner and 1.5dp focused border, the 56dp primary pill, the
// 11sp section label at 0.08em. These are the pieces every screen is assembled
// from over there, so they arrive as pieces here too rather than as properties
// pasted into each screen.
//
// BrandWordmark paints the brand mark the web app uses, converted into a
// drawable for this client.

package mn.gerege.nexus.ui.brand

import androidx.compose.animation.animateColorAsState
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.graphics.vector.path
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import mn.gerege.nexus.ui.theme.LocalGw

/**
 * The primary button's leading arrow, drawn here rather than imported.
 *
 * Their app takes it from `androidx.compose.material.icons`, and this one
 * cannot: material3 used to pull `material-icons-core` in transitively and no
 * longer does, and that artifact stopped at 1.7.8 — so reaching for it would
 * mean pinning a discontinued library beside a 2026 BOM to obtain one glyph.
 * This is Material's own arrow_forward path, which keeps the design exact
 * without the dependency.
 */
private val ArrowForwardVector: ImageVector = ImageVector.Builder(
    name = "ArrowForward",
    defaultWidth = 24.dp,
    defaultHeight = 24.dp,
    viewportWidth = 24f,
    viewportHeight = 24f,
    autoMirror = true,
).apply {
    // Material's arrow_forward, 24x24:
    //   M12,4 l-1.41,1.41 L16.17,11 H4 v2 h12.17 l-5.58,5.59 L12,20 l8,-8 z
    // The tint comes from Icon's `tint`, so the fill here is only what an
    // untinted render would use.
    path(fill = SolidColor(Color.White)) {
        moveTo(12f, 4f)
        lineToRelative(-1.41f, 1.41f)
        lineTo(16.17f, 11f)
        horizontalLineTo(4f)
        verticalLineToRelative(2f)
        horizontalLineToRelative(12.17f)
        lineToRelative(-5.58f, 5.59f)
        lineTo(12f, 20f)
        lineToRelative(8f, -8f)
        close()
    }
}.build()

/** Full-bleed canvas tinted with the `bg` token. */
@Composable
fun BrandScreen(modifier: Modifier = Modifier, content: @Composable () -> Unit) {
    val gw = LocalGw.current
    Box(modifier = modifier.fillMaxSize().background(gw.bg)) { content() }
}

/** Section label — uppercase, letter-spaced, fg-3. */
@Composable
fun BrandSectionLabel(text: String, modifier: Modifier = Modifier) {
    val gw = LocalGw.current
    Text(
        text = text,
        modifier = modifier.padding(start = 4.dp),
        style = MaterialTheme.typography.labelSmall.copy(
            fontSize = 11.sp,
            fontWeight = FontWeight.SemiBold,
            letterSpacing = 0.88.sp, // 0.08em ≈ 11sp × .08
        ),
        color = gw.fg3,
    )
}

/**
 * A 52dp surface-1 card holding a field. The border steps from divider to a
 * 1.5dp brand line on focus, which is what tells somebody where they are
 * typing without a label having to shout it.
 */
@Composable
fun BrandInputCard(
    leadingIcon: ImageVector? = null,
    isFocused: Boolean = false,
    modifier: Modifier = Modifier,
    content: @Composable () -> Unit,
) {
    val gw = LocalGw.current
    val borderColor by animateColorAsState(
        targetValue = if (isFocused) gw.brand else gw.border,
        label = "input-border",
    )
    Surface(
        modifier = modifier.fillMaxWidth().heightIn(min = 52.dp),
        shape = RoundedCornerShape(14.dp),
        color = gw.surface1,
        border = BorderStroke(if (isFocused) 1.5.dp else 1.dp, borderColor),
        tonalElevation = 0.dp,
    ) {
        Row(
            modifier = Modifier.padding(horizontal = 14.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            if (leadingIcon != null) {
                Icon(
                    imageVector = leadingIcon,
                    contentDescription = null,
                    tint = gw.fg3,
                    modifier = Modifier.size(19.dp),
                )
            }
            Box(modifier = Modifier.weight(1f)) { content() }
        }
    }
}

/** A 56dp brand pill with a leading arrow, and a spinner in place of the label
 *  while something is in flight. */
@Composable
fun LoadingPrimaryButton(
    label: String,
    isLoading: Boolean,
    isEnabled: Boolean,
    leadingIcon: ImageVector? = ArrowForwardVector,
    modifier: Modifier = Modifier,
    onClick: () -> Unit,
) {
    val gw = LocalGw.current
    Button(
        onClick = { if (!isLoading && isEnabled) onClick() },
        modifier = modifier.fillMaxWidth().heightIn(min = 56.dp),
        enabled = isEnabled && !isLoading,
        shape = RoundedCornerShape(14.dp),
        colors = ButtonDefaults.buttonColors(
            containerColor = gw.brand,
            contentColor = Color.White,
            disabledContainerColor = gw.brand.copy(alpha = 0.4f),
            disabledContentColor = Color.White,
        ),
        contentPadding = PaddingValues(horizontal = 16.dp, vertical = 12.dp),
    ) {
        if (isLoading) {
            CircularProgressIndicator(
                color = Color.White,
                strokeWidth = 2.dp,
                modifier = Modifier.size(22.dp),
            )
        } else {
            Row(
                horizontalArrangement = Arrangement.spacedBy(10.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                if (leadingIcon != null) {
                    Icon(imageVector = leadingIcon, contentDescription = null, modifier = Modifier.size(20.dp))
                }
                Text(
                    text = label,
                    style = MaterialTheme.typography.titleMedium.copy(
                        fontSize = 17.sp,
                        fontWeight = FontWeight.Bold,
                    ),
                )
            }
        }
    }
}

/**
 * The brand mark, at the size their login screen gives it.
 *
 * Not tinted: the mark carries its own colour — a silver medallion on the
 * brand blue — and flattening it to a single tint would throw away the thing
 * that makes it recognisable.
 */
@Composable
fun BrandWordmark(size: Dp = 56.dp, modifier: Modifier = Modifier) {
    Image(
        painter = painterResource(id = mn.gerege.nexus.R.drawable.brand),
        contentDescription = "Gerege Nexus",
        modifier = modifier.size(size).clip(RoundedCornerShape(14.dp)),
    )
}

/** The quiet line at the foot of an auth screen. */
@Composable
fun BrandSecurityFooter(text: String, modifier: Modifier = Modifier) {
    val gw = LocalGw.current
    Row(
        modifier = modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.Center,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            text = text,
            style = MaterialTheme.typography.labelSmall.copy(fontSize = 12.sp),
            color = gw.fg3,
            textAlign = TextAlign.Center,
        )
    }
}
