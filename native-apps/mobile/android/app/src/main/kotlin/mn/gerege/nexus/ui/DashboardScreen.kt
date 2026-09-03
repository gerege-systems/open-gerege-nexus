package mn.gerege.nexus.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowForward
import androidx.compose.material.icons.filled.Badge
import androidx.compose.material.icons.filled.Business
import androidx.compose.material.icons.filled.ChildCare
import androidx.compose.material.icons.filled.Verified
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import mn.gerege.nexus.AppState
import mn.gerege.nexus.R
import mn.gerege.nexus.ui.components.*
import mn.gerege.nexus.ui.theme.LocalGw
import mn.gerege.nexus.ui.theme.Radius
import mn.gerege.nexus.ui.theme.Space

/**
 * Самбар — iOS-ийн `MobileDashboardPage`-тай ижил агуулга, ижил дараалал.
 *
 * Hero нь брэндийн градиент карт: wallet дээр тэр байрыг үлдэгдэл эзэлдэг,
 * энд иргэний өөрийнх нь мөр эзэлнэ — аль ч аппыг нээхэд эхний зүйл нь
 * «би хэн бэ» гэдэг нэг ижил хэлбэрээр угтана.
 */
@Composable
fun DashboardScreen(state: AppState) {
    val gw = LocalGw.current
    EidScreen(title = stringResource(R.string.Nav_Dashboard)) {
        BrandHeroCard {
            Row(verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(Space.md)) {
                Text(
                    initials(state.fullName),
                    style = MaterialTheme.typography.headlineSmall,
                    color = Color.White,
                    modifier = Modifier
                        .size(56.dp)
                        .background(Color.White.copy(alpha = 0.22f), CircleShape)
                        .wrapContentSize(Alignment.Center),
                )
                Column(verticalArrangement = Arrangement.spacedBy(Space.xs)) {
                    Text(state.fullName.ifEmpty { stringResource(R.string.Dashboard_Greeting) },
                         style = MaterialTheme.typography.titleLarge, color = Color.White)
                    if (state.civilId.isNotEmpty()) {
                        Text(state.civilId,
                             style = MaterialTheme.typography.labelSmall.copy(fontFamily = FontFamily.Monospace),
                             color = Color.White.copy(alpha = 0.75f))
                    }
                }
            }
            Row(
                modifier = Modifier
                    .background(Color.White.copy(alpha = 0.18f), RoundedCornerShape(percent = 50))
                    .padding(horizontal = 10.dp, vertical = 5.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(6.dp),
            ) {
                Icon(Icons.Filled.Verified, null, tint = Color.White, modifier = Modifier.size(13.dp))
                Text(stringResource(R.string.Dashboard_StatusBadge),
                     style = MaterialTheme.typography.labelSmall, color = Color.White)
            }
        }

        Row(horizontalArrangement = Arrangement.spacedBy(Space.md), modifier = Modifier.fillMaxWidth()) {
            StatTile(stringResource(R.string.Dashboard_Stats_Logins),
                     state.activity.count { it.sessionType == "AUTH" }.toString(),
                     Icons.AutoMirrored.Filled.ArrowForward, Modifier.weight(1f))
            StatTile(stringResource(R.string.Nav_MyOrganizations),
                     state.organizations.size.toString(),
                     Icons.Filled.Business, Modifier.weight(1f))
        }
        Row(horizontalArrangement = Arrangement.spacedBy(Space.md), modifier = Modifier.fillMaxWidth()) {
            StatTile(stringResource(R.string.Nav_Children), state.children.size.toString(),
                     Icons.Filled.ChildCare, Modifier.weight(1f))
            StatTile(stringResource(R.string.Dashboard_Stats_Certificates),
                     if (state.identity?.documentNumber.isNullOrEmpty()) "0" else "1",
                     Icons.Filled.Badge, Modifier.weight(1f))
        }

        EidCard {
            Text(stringResource(R.string.Dashboard_Activity_Section),
                 style = MaterialTheme.typography.titleMedium, color = gw.fg1)
            val recent = state.activity.take(5)
            if (recent.isEmpty()) {
                Text(stringResource(R.string.Dashboard_Activity_Empty),
                     style = MaterialTheme.typography.bodySmall, color = gw.fg3)
            } else {
                recent.forEach { entry ->
                    Row(verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(Space.md),
                        modifier = Modifier.fillMaxWidth()) {
                        Box(
                            modifier = Modifier.size(30.dp).background(gw.brandSoft, CircleShape),
                            contentAlignment = Alignment.Center,
                        ) {
                            Icon(Icons.AutoMirrored.Filled.ArrowForward, null, tint = gw.brand,
                                 modifier = Modifier.size(14.dp))
                        }
                        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                            Text(entry.rpName,
                                 style = MaterialTheme.typography.bodySmall.copy(fontWeight = FontWeight.SemiBold),
                                 color = gw.fg1)
                            Text(AppState.shortDate(entry.createdAt),
                                 style = MaterialTheme.typography.labelSmall.copy(
                                     fontFamily = FontFamily.Monospace, fontWeight = FontWeight.Normal),
                                 color = gw.fg3)
                        }
                        StatusPill(
                            stringResource(if (entry.result == "OK") R.string.Dashboard_Activity_Success
                                           else R.string.Dashboard_Activity_Failure),
                            if (entry.result == "OK") PillVariant.OK else PillVariant.WARN,
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun StatTile(label: String, value: String, icon: ImageVector, modifier: Modifier = Modifier) {
    val gw = LocalGw.current
    EidCard(modifier = modifier, padding = Space.md, spacing = Space.sm) {
        Box(
            modifier = Modifier.size(32.dp).background(gw.brandSoft, RoundedCornerShape(Radius.sm)),
            contentAlignment = Alignment.Center,
        ) {
            Icon(icon, null, tint = gw.brand, modifier = Modifier.size(16.dp))
        }
        Text(value, style = MaterialTheme.typography.headlineMedium, color = gw.fg1)
        Text(label, style = MaterialTheme.typography.labelSmall.copy(fontWeight = FontWeight.Normal),
             color = gw.fg3)
    }
}

internal fun initials(name: String): String =
    name.split(" ").filter { it.isNotBlank() }.take(2)
        .mapNotNull { it.firstOrNull()?.uppercaseChar() }.joinToString("").ifEmpty { "?" }
