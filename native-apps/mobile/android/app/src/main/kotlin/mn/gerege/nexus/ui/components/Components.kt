package mn.gerege.nexus.ui.components

import androidx.compose.animation.animateColorAsState
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowForward
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.Lock
import androidx.compose.material.icons.filled.Notifications
import androidx.compose.material.icons.filled.Warning
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import mn.gerege.nexus.ui.theme.LocalGw
import mn.gerege.nexus.ui.theme.Radius
import mn.gerege.nexus.ui.theme.Space

/**
 * Утасны дүрсийн сан — iOS-ийн `Design/BrandComponents.swift`-ийн ХОСОЛ.
 *
 * Gerege Wallet-ийн брэндийн примитивүүдийн порт: дэлгэцийн суурь, карт,
 * оролтын сав, баталгаажуулах капсул, мэдээллийн самбар, аюулгүйн хөл,
 * ачаалалтай CTA. Геометр (52dp оролтын мөр, 56dp CTA, 26dp брэндийн
 * дөрвөлжин, 14dp радиус) нь эх дизайнаас яг тэр чигээрээ — тоог солихын
 * өмнө эх сурвалжаас шинэ хэмжээс аваарай.
 *
 * Өнгө нь `LocalGw`-гээс (`ui/theme/Gw.kt`). Хуучин нэрс (`EidCard`,
 * `StatusPill`, `EidField`, `PrimaryButton`, …) ХЭВЭЭР үлдсэн нь санаатай:
 * харагдац солих ажил дуудлагын бүх талбарыг гар хүрэх шалтгаан болох ёсгүй.
 */

// ── Дэлгэцийн суурь ────────────────────────────────────────────────────

/** Дэлгэц бүрийн нийтлэг хүрээ — гарчиг + гүйлгэх муж + ижил дэвсгэр. */
@Composable
fun EidScreen(title: String, subtitle: String? = null, content: @Composable ColumnScope.() -> Unit) {
    val gw = LocalGw.current
    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(gw.bg)
            .verticalScroll(rememberScrollState())
            .padding(horizontal = Space.lg, vertical = Space.lg),
        verticalArrangement = Arrangement.spacedBy(Space.md),
        horizontalAlignment = Alignment.Start,
    ) {
        Text(title, style = MaterialTheme.typography.headlineLarge, color = gw.fg1)
        if (!subtitle.isNullOrEmpty()) {
            Text(subtitle, style = MaterialTheme.typography.bodySmall, color = gw.fg3)
        }
        content()
        Spacer(Modifier.height(Space.sm))
    }
}

/** Цагаан (харанхуйд өргөгдсөн) бүлэг карт — 1dp хүрээ, зөөлөн сүүдэр. */
@Composable
fun EidCard(
    modifier: Modifier = Modifier,
    padding: Dp = Space.lg,
    spacing: Dp = Space.md,
    content: @Composable ColumnScope.() -> Unit,
) {
    val gw = LocalGw.current
    Column(
        modifier = modifier
            .fillMaxWidth()
            .shadow(2.dp, RoundedCornerShape(Radius.lg), clip = false)
            .background(gw.surface1, RoundedCornerShape(Radius.lg))
            .border(1.dp, gw.border, RoundedCornerShape(Radius.lg))
            .padding(padding),
        verticalArrangement = Arrangement.spacedBy(spacing),
        content = content,
    )
}

/** Брэндийн градиент hero — самбарын дээд карт. */
@Composable
fun BrandHeroCard(modifier: Modifier = Modifier, content: @Composable ColumnScope.() -> Unit) {
    val gw = LocalGw.current
    Column(
        modifier = modifier
            .fillMaxWidth()
            .shadow(6.dp, RoundedCornerShape(Radius.lg), clip = false)
            .clip(RoundedCornerShape(Radius.lg))
            .background(Brush.linearGradient(listOf(gw.brand, gw.brandDeep)))
            .padding(Space.lg),
        verticalArrangement = Arrangement.spacedBy(Space.lg),
        content = content,
    )
}

// ── Бичвэрийн жижиг дүрсүүд ────────────────────────────────────────────

/** Том үсгээр, зай нэмсэн хэсгийн шошго. */
@Composable
fun BrandSectionLabel(text: String, modifier: Modifier = Modifier) {
    val gw = LocalGw.current
    Text(
        text = text.uppercase(),
        modifier = modifier,
        style = MaterialTheme.typography.labelSmall.copy(letterSpacing = 0.9.sp),
        color = gw.fg3,
    )
}

/** Шошго + утга. iOS-ийн `MobileField`-тэй ижил хэмжээс. */
@Composable
fun EidField(label: String, value: String, mono: Boolean = false) {
    val gw = LocalGw.current
    Column(verticalArrangement = Arrangement.spacedBy(Space.xs)) {
        BrandSectionLabel(label)
        Text(
            value.ifEmpty { "—" },
            style = if (mono) {
                MaterialTheme.typography.bodyMedium.copy(fontFamily = FontFamily.Monospace)
            } else {
                MaterialTheme.typography.bodyLarge.copy(fontWeight = FontWeight.SemiBold)
            },
            color = gw.fg1,
        )
    }
}

// ── Төлөвийн капсул ────────────────────────────────────────────────────

enum class PillVariant { OK, WARN, ACCENT }

@Composable
fun StatusPill(text: String, variant: PillVariant = PillVariant.OK) {
    val gw = LocalGw.current
    val (tint, soft) = when (variant) {
        PillVariant.OK -> gw.credit to gw.creditSoft
        PillVariant.WARN -> gw.accent to gw.accentSoft
        PillVariant.ACCENT -> gw.brand to gw.brandSoft
    }
    Text(
        text = text,
        style = MaterialTheme.typography.labelSmall.copy(letterSpacing = 0.4.sp),
        color = tint,
        modifier = Modifier
            .background(soft, RoundedCornerShape(percent = 50))
            .padding(horizontal = 9.dp, vertical = 4.dp),
    )
}

// ── Оролт ──────────────────────────────────────────────────────────────

data class BrandValidationState(val label: String, val valid: Boolean)

@Composable
fun BrandValidationBadge(label: String, valid: Boolean) {
    val gw = LocalGw.current
    val tint = if (valid) gw.credit else gw.debit
    val soft = if (valid) gw.creditSoft else gw.debitSoft
    Row(
        modifier = Modifier
            .background(soft, RoundedCornerShape(percent = 50))
            .padding(horizontal = 9.dp, vertical = 4.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(Space.xs),
    ) {
        Icon(
            imageVector = if (valid) Icons.Filled.Check else Icons.Filled.Close,
            contentDescription = null,
            tint = tint,
            modifier = Modifier.size(12.dp),
        )
        Text(label, color = tint, style = MaterialTheme.typography.labelSmall)
    }
}

/**
 * Оролтын сав — 52dp өндөр, дүрс + талбар + баталгаажуулалтын капсул.
 * Фокус дээр хүрээ нь брэнд рүү, 1.5dp болж зузаарна.
 */
@Composable
fun BrandInputCard(
    leadingIcon: ImageVector? = null,
    validation: BrandValidationState? = null,
    isFocused: Boolean = false,
    modifier: Modifier = Modifier,
    content: @Composable () -> Unit,
) {
    val gw = LocalGw.current
    val borderColor by animateColorAsState(
        targetValue = if (isFocused) gw.brand else gw.border,
        label = "input-border",
    )
    Row(
        modifier = modifier
            .fillMaxWidth()
            .heightIn(min = 52.dp)
            .background(gw.surface1, RoundedCornerShape(14.dp))
            .border(if (isFocused) 1.5.dp else 1.dp, borderColor, RoundedCornerShape(14.dp))
            .padding(horizontal = 14.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        if (leadingIcon != null) {
            Icon(leadingIcon, contentDescription = null, tint = gw.fg3, modifier = Modifier.size(19.dp))
        }
        Box(Modifier.weight(1f)) { content() }
        if (validation != null) BrandValidationBadge(validation.label, validation.valid)
    }
}

// ── Самбарууд ──────────────────────────────────────────────────────────

/** Мэдээллийн самбар — 26dp брэндийн дөрвөлжин дотор цагаан хонх. */
@Composable
fun BrandInfoBanner(text: String, modifier: Modifier = Modifier) {
    val gw = LocalGw.current
    Row(
        modifier = modifier
            .fillMaxWidth()
            .background(gw.brandSoft, RoundedCornerShape(Radius.md))
            .border(1.dp, gw.brandLine, RoundedCornerShape(Radius.md))
            .padding(Space.md),
        verticalAlignment = Alignment.Top,
        horizontalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        Box(
            modifier = Modifier.size(26.dp).background(gw.brand, RoundedCornerShape(Radius.sm)),
            contentAlignment = Alignment.Center,
        ) {
            Icon(Icons.Filled.Notifications, null, tint = Color.White, modifier = Modifier.size(14.dp))
        }
        Text(
            text = text,
            modifier = Modifier.weight(1f),
            style = MaterialTheme.typography.labelMedium.copy(
                fontWeight = FontWeight.Normal, lineHeight = 17.sp,
            ),
            color = gw.fg2,
        )
    }
}

/** Алдаа/амжилтын мөр. */
@Composable
fun InlineBanner(text: String, isError: Boolean = true) {
    val gw = LocalGw.current
    val tint = if (isError) gw.debit else gw.credit
    val soft = if (isError) gw.debitSoft else gw.creditSoft
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .background(soft, RoundedCornerShape(Radius.md))
            .border(1.dp, tint.copy(alpha = 0.35f), RoundedCornerShape(Radius.md))
            .padding(Space.md),
        verticalAlignment = Alignment.Top,
        horizontalArrangement = Arrangement.spacedBy(Space.sm),
    ) {
        Icon(
            if (isError) Icons.Filled.Warning else Icons.Filled.Check,
            contentDescription = null, tint = tint, modifier = Modifier.size(15.dp),
        )
        Text(
            text = text,
            modifier = Modifier.weight(1f),
            style = MaterialTheme.typography.labelMedium.copy(
                fontWeight = FontWeight.Normal, lineHeight = 17.sp,
            ),
            color = gw.fg2,
        )
    }
}

/** Дэлгэцийн доод аюулгүйн мөр. */
@Composable
fun BrandSecurityFooter(text: String, modifier: Modifier = Modifier) {
    val gw = LocalGw.current
    Row(
        modifier = modifier,
        horizontalArrangement = Arrangement.spacedBy(6.dp, Alignment.CenterHorizontally),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(Icons.Filled.Lock, null, tint = gw.fg3, modifier = Modifier.size(13.dp))
        Text(text, style = MaterialTheme.typography.labelSmall, color = gw.fg3, textAlign = TextAlign.Center)
    }
}

// ── Баталгаажуулах код ─────────────────────────────────────────────────
//
// Хоёр аппын хооронд ТУЛГАХ ёстой тоо тул тоо бүр өөрийн нүдэнд, monospace.

@Composable
fun BrandCodeRow(code: String, modifier: Modifier = Modifier) {
    val gw = LocalGw.current
    Row(modifier = modifier, horizontalArrangement = Arrangement.spacedBy(Space.sm)) {
        code.forEach { digit ->
            Box(
                modifier = Modifier
                    .size(width = 40.dp, height = 52.dp)
                    .background(gw.brandSoft, RoundedCornerShape(Radius.md))
                    .border(1.dp, gw.brandLine, RoundedCornerShape(Radius.md)),
                contentAlignment = Alignment.Center,
            ) {
                Text(
                    digit.toString(),
                    fontSize = 26.sp,
                    fontWeight = FontWeight.Bold,
                    fontFamily = FontFamily.Monospace,
                    color = gw.brand,
                )
            }
        }
    }
}

// ── Товчнууд ───────────────────────────────────────────────────────────

/**
 * Үндсэн CTA — 56dp өндөр брэндийн блок. Ачаалж байх үед шошгыг спиннер
 * СОЛИНО: сүлжээний эргэлтийн дунд хоёр дахь даралт давхар session үүсгэхгүй.
 */
@Composable
fun LoadingPrimaryButton(
    label: String,
    isLoading: Boolean = false,
    enabled: Boolean = true,
    leadingIcon: ImageVector? = Icons.AutoMirrored.Filled.ArrowForward,
    modifier: Modifier = Modifier,
    onClick: () -> Unit,
) {
    val gw = LocalGw.current
    val active = enabled && !isLoading
    Box(
        modifier = modifier
            .fillMaxWidth()
            .heightIn(min = 56.dp)
            .clip(RoundedCornerShape(14.dp))
            .background(if (enabled) gw.brand else gw.brand.copy(alpha = 0.4f))
            .then(if (active) Modifier.clickableRow(onClick) else Modifier),
        contentAlignment = Alignment.Center,
    ) {
        if (isLoading) {
            CircularProgressIndicator(color = Color.White, strokeWidth = 2.dp, modifier = Modifier.size(22.dp))
        } else {
            Row(
                horizontalArrangement = Arrangement.spacedBy(10.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                if (leadingIcon != null) {
                    Icon(leadingIcon, null, tint = Color.White, modifier = Modifier.size(20.dp))
                }
                Text(label, style = MaterialTheme.typography.titleMedium, color = Color.White)
            }
        }
    }
}

/** Хуучин нэр — дуудлагын талбарууд хэвээр ажиллана. */
@Composable
fun PrimaryButton(text: String, enabled: Boolean = true, onClick: () -> Unit) =
    LoadingPrimaryButton(label = text, enabled = enabled, onClick = onClick)

/** Хоёрдогч товч — хүрээтэй, дэвсгэргүй. */
@Composable
fun SecondaryButton(
    text: String,
    enabled: Boolean = true,
    tone: Color? = null,
    modifier: Modifier = Modifier,
    onClick: () -> Unit,
) {
    val gw = LocalGw.current
    val fg = if (enabled) (tone ?: gw.fg1) else gw.fg4
    Box(
        modifier = modifier
            .fillMaxWidth()
            .heightIn(min = 48.dp)
            .clip(RoundedCornerShape(14.dp))
            .border(1.dp, if (enabled) gw.borderStrong else gw.border, RoundedCornerShape(14.dp))
            .then(if (enabled) Modifier.clickableRow(onClick) else Modifier),
        contentAlignment = Alignment.Center,
    ) {
        Text(text, style = MaterialTheme.typography.labelLarge, color = fg)
    }
}

/** Тексттэй холбоос маягийн товч. */
@Composable
fun BrandLinkButton(text: String, modifier: Modifier = Modifier, onClick: () -> Unit) {
    val gw = LocalGw.current
    Text(
        text = text,
        style = MaterialTheme.typography.labelMedium,
        color = gw.brand,
        modifier = modifier.clickableRow(onClick).padding(vertical = Space.sm, horizontal = Space.xs),
    )
}

private fun Modifier.clickableRow(onClick: () -> Unit): Modifier = this.clickable(onClick = onClick)
