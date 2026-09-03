// The design handoff's component vocabulary (design/README.md).
//
// Wallet-ported tokens stay where they were (ui/theme); what changed with the
// 2026-09 handoff is the pieces screens are assembled from: a 56dp filled
// primary with a trailing arrow and a #1F5FD0 pressed step, a tonal twin on
// brandSoft, a 48dp quiet outline for the demoted admin zone, fields drawn on
// surface2 with a 12% border that steps to brand on focus, and the eyebrow /
// badge / security-footer trio. Sizes come from the handoff verbatim — pick
// from Space/Radius, do not invent.

package mn.gerege.nexus.ui.brand

import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsPressedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.luminance
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.TextUnit
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import mn.gerege.nexus.ui.icons.Lucide
import mn.gerege.nexus.ui.theme.LocalGw
import mn.gerege.nexus.ui.theme.Radius
import mn.gerege.nexus.ui.theme.Space

/** Full-bleed canvas tinted with the `bg` token. */
@Composable
fun BrandScreen(modifier: Modifier = Modifier, content: @Composable () -> Unit) {
    val gw = LocalGw.current
    Box(modifier = modifier.fillMaxSize().background(gw.bg)) { content() }
}

/** Light mode is the one that carries card shadows; dark carries borders. */
@Composable
fun isLightTheme(): Boolean = LocalGw.current.bg.luminance() > 0.5f

/**
 * Eyebrow — uppercase, letter-spaced, fg-3. The handoff has two sizes: 13sp at
 * 0.08em for section eyebrows, 12sp at 0.16em for the wordmark line.
 */
@Composable
fun BrandSectionLabel(
    text: String,
    modifier: Modifier = Modifier,
    fontSize: TextUnit = 13.sp,
    trackingEm: Float = 0.08f,
    color: Color = LocalGw.current.fg3,
) {
    Text(
        text = text,
        modifier = modifier,
        fontSize = fontSize,
        fontWeight = FontWeight.SemiBold,
        letterSpacing = fontSize * trackingEm,
        color = color,
    )
}

/** «ҮНДСЭН»-style pill: 12sp semibold caps on a soft fill, radius-sm. */
@Composable
fun BrandBadge(
    text: String,
    modifier: Modifier = Modifier,
    color: Color = LocalGw.current.brand,
    container: Color = LocalGw.current.brandSoft,
) {
    Surface(color = container, shape = RoundedCornerShape(Radius.sm), modifier = modifier) {
        Text(
            text = text,
            modifier = Modifier.padding(horizontal = Space.sm, vertical = Space.xs),
            color = color,
            fontSize = 12.sp,
            fontWeight = FontWeight.SemiBold,
            letterSpacing = 0.5.sp,
        )
    }
}

/** The handoff's pressed feel: scale 0.97 over 120ms. Colour steps live in
 *  each button, since filled and tonal step differently. */
private fun Modifier.pressedScale(scale: Float): Modifier =
    graphicsLayer { scaleX = scale; scaleY = scale }

@Composable
private fun rememberPressScale(interaction: MutableInteractionSource): Float {
    val pressed by interaction.collectIsPressedAsState()
    val scale by animateFloatAsState(
        targetValue = if (pressed) 0.97f else 1f,
        animationSpec = tween(120),
        label = "press-scale",
    )
    return scale
}

/**
 * Field on surface-2 with the 12% border that steps to a 1.5dp brand line on
 * focus. `filled = false` is the admin variant: transparent, 48dp, 15sp.
 */
@Composable
fun BrandField(
    value: String,
    onValueChange: (String) -> Unit,
    placeholder: String,
    modifier: Modifier = Modifier,
    height: Dp = 56.dp,
    fontSize: TextUnit = 17.sp,
    filled: Boolean = true,
    enabled: Boolean = true,
    keyboardOptions: KeyboardOptions = KeyboardOptions.Default,
    visualTransformation: VisualTransformation = VisualTransformation.None,
) {
    val gw = LocalGw.current
    var focused by remember { mutableStateOf(false) }
    val borderColor by animateColorAsState(
        targetValue = if (focused) gw.brand else gw.borderStrong,
        label = "field-border",
    )
    Surface(
        modifier = modifier.fillMaxWidth().height(height),
        shape = RoundedCornerShape(Radius.md),
        color = if (filled) gw.surface2 else Color.Transparent,
        border = BorderStroke(if (focused) 1.5.dp else 1.dp, borderColor),
    ) {
        Box(Modifier.padding(horizontal = Space.lg), contentAlignment = Alignment.CenterStart) {
            BasicTextField(
                value = value,
                onValueChange = onValueChange,
                modifier = Modifier.fillMaxWidth().onFocusChanged { focused = it.isFocused },
                textStyle = TextStyle(color = gw.fg1, fontSize = fontSize),
                singleLine = true,
                enabled = enabled,
                keyboardOptions = keyboardOptions,
                visualTransformation = visualTransformation,
                cursorBrush = SolidColor(gw.brand),
            )
            if (value.isEmpty()) Text(placeholder, color = gw.fg3, fontSize = fontSize)
        }
    }
}

/** 56dp filled brand button: trailing arrow, #1F5FD0 pressed, spinner while in
 *  flight. */
@Composable
fun LoadingPrimaryButton(
    label: String,
    isLoading: Boolean,
    isEnabled: Boolean,
    modifier: Modifier = Modifier,
    height: Dp = 56.dp,
    fontSize: TextUnit = 17.sp,
    trailingIcon: ImageVector? = Lucide.ArrowRight,
    onClick: () -> Unit,
) {
    val gw = LocalGw.current
    val interaction = remember { MutableInteractionSource() }
    val pressed by interaction.collectIsPressedAsState()
    Button(
        onClick = { if (!isLoading && isEnabled) onClick() },
        modifier = modifier.fillMaxWidth().heightIn(min = height).pressedScale(rememberPressScale(interaction)),
        enabled = isEnabled && !isLoading,
        shape = RoundedCornerShape(Radius.md),
        interactionSource = interaction,
        colors = ButtonDefaults.buttonColors(
            containerColor = if (pressed) gw.brandDeep else gw.brand,
            contentColor = gw.onBrand,
            // The handoff's loading step: #1F5FD0 with the label at 60%.
            disabledContainerColor = if (isLoading) gw.brandDeep else gw.brand.copy(alpha = 0.4f),
            disabledContentColor = gw.onBrand.copy(alpha = if (isLoading) 0.6f else 1f),
        ),
        contentPadding = PaddingValues(horizontal = Space.lg, vertical = Space.md),
    ) {
        if (isLoading) {
            CircularProgressIndicator(
                color = gw.onBrand,
                strokeWidth = 2.dp,
                modifier = Modifier.size(20.dp),
            )
        } else {
            Row(
                horizontalArrangement = Arrangement.spacedBy(Space.sm + Space.xxs),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(label, fontSize = fontSize, fontWeight = FontWeight.SemiBold)
                if (trailingIcon != null) {
                    Icon(imageVector = trailingIcon, contentDescription = null, modifier = Modifier.size(20.dp))
                }
            }
        }
    }
}

/** 56dp tonal twin — brandSoft fill, brand label, leading icon. */
@Composable
fun TonalButton(
    label: String,
    modifier: Modifier = Modifier,
    isEnabled: Boolean = true,
    height: Dp = 56.dp,
    fontSize: TextUnit = 17.sp,
    leadingIcon: ImageVector? = null,
    onClick: () -> Unit,
) {
    val gw = LocalGw.current
    val interaction = remember { MutableInteractionSource() }
    val labelColor = if (isLightTheme()) gw.brandDeep else gw.brand
    Button(
        onClick = onClick,
        modifier = modifier.fillMaxWidth().heightIn(min = height).pressedScale(rememberPressScale(interaction)),
        enabled = isEnabled,
        shape = RoundedCornerShape(Radius.md),
        interactionSource = interaction,
        colors = ButtonDefaults.buttonColors(
            containerColor = gw.brandSoft,
            contentColor = labelColor,
            disabledContainerColor = gw.brandSoft.copy(alpha = 0.5f),
            disabledContentColor = labelColor.copy(alpha = 0.5f),
        ),
        contentPadding = PaddingValues(horizontal = Space.lg, vertical = Space.md),
    ) {
        Row(
            horizontalArrangement = Arrangement.spacedBy(Space.sm + Space.xxs),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            if (leadingIcon != null) {
                Icon(imageVector = leadingIcon, contentDescription = null, modifier = Modifier.size(20.dp))
            }
            Text(label, fontSize = fontSize, fontWeight = FontWeight.SemiBold)
        }
    }
}

/** 48dp quiet outline — the admin zone's register: 12% border, fg-2 label. */
@Composable
fun QuietButton(
    label: String,
    modifier: Modifier = Modifier,
    isEnabled: Boolean = true,
    height: Dp = 48.dp,
    fontSize: TextUnit = 15.sp,
    leadingIcon: ImageVector? = null,
    onClick: () -> Unit,
) {
    val gw = LocalGw.current
    val interaction = remember { MutableInteractionSource() }
    OutlinedButton(
        onClick = onClick,
        modifier = modifier.fillMaxWidth().heightIn(min = height).pressedScale(rememberPressScale(interaction)),
        enabled = isEnabled,
        shape = RoundedCornerShape(Radius.md),
        interactionSource = interaction,
        border = BorderStroke(1.dp, gw.borderStrong),
        colors = ButtonDefaults.outlinedButtonColors(
            contentColor = gw.fg2,
            disabledContentColor = gw.fg4,
        ),
        contentPadding = PaddingValues(horizontal = Space.lg, vertical = Space.sm),
    ) {
        Row(
            horizontalArrangement = Arrangement.spacedBy(Space.sm),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            if (leadingIcon != null) {
                Icon(imageVector = leadingIcon, contentDescription = null, modifier = Modifier.size(16.dp))
            }
            Text(label, fontSize = fontSize, fontWeight = FontWeight.SemiBold)
        }
    }
}

/**
 * The brand mark — the handoff's logo-mark asset. Not tinted: the mark carries
 * its own colour, a silver medallion on the brand blue.
 */
@Composable
fun BrandWordmark(size: Dp = 48.dp, radius: Dp = Radius.md, modifier: Modifier = Modifier) {
    Image(
        painter = painterResource(id = mn.gerege.nexus.R.drawable.logo_mark),
        contentDescription = "Gerege Nexus",
        modifier = modifier.size(size).clip(RoundedCornerShape(radius)),
    )
}

/** The quiet line at the foot of an auth screen: 14dp lock + 12sp fg-3. */
@Composable
fun BrandSecurityFooter(text: String, modifier: Modifier = Modifier) {
    val gw = LocalGw.current
    Row(
        modifier = modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.Center,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(
            imageVector = Lucide.Lock,
            contentDescription = null,
            tint = gw.fg3,
            modifier = Modifier.size(14.dp),
        )
        Text(
            text = text,
            modifier = Modifier.padding(start = Space.sm),
            style = MaterialTheme.typography.labelSmall.copy(fontSize = 12.sp),
            color = gw.fg3,
            textAlign = TextAlign.Center,
        )
    }
}
