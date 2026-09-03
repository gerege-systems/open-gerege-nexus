package mn.gerege.nexus.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowForward
import androidx.compose.material.icons.filled.Draw
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
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
 * Лог түүх — ЭНЭ төхөөрөмжийн локал бүртгэл.
 *
 * v3 RP-API нь иргэний бүх session-ий түүхийг өгдөггүй тул утас, мак хоёр
 * өөр өөр жагсаалт харуулна — тэр нь алдаа биш, өөр өөр төхөөрөмжийн түүх.
 */
@Composable
fun LogsScreen(state: AppState) {
    val gw = LocalGw.current
    EidScreen(title = stringResource(R.string.Nav_Logs), subtitle = stringResource(R.string.Logs_Subtitle)) {
        if (state.activity.isEmpty()) {
            EidCard {
                Text(stringResource(R.string.Dashboard_Activity_Empty),
                     style = MaterialTheme.typography.bodySmall, color = gw.fg3)
            }
        } else {
            state.activity.forEach { entry ->
                EidCard(padding = Space.md) {
                    Row(verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(Space.md),
                        modifier = Modifier.fillMaxWidth()) {
                        Box(
                            modifier = Modifier
                                .size(36.dp)
                                .background(gw.brandSoft, RoundedCornerShape(Radius.md)),
                            contentAlignment = Alignment.Center,
                        ) {
                            Icon(
                                if (entry.sessionType == "AUTH") Icons.AutoMirrored.Filled.ArrowForward else Icons.Filled.Draw,
                                contentDescription = null, tint = gw.brand, modifier = Modifier.size(17.dp),
                            )
                        }
                        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(3.dp)) {
                            Text(
                                stringResource(if (entry.sessionType == "AUTH") R.string.Dashboard_Activity_Auth
                                               else R.string.Dashboard_Activity_Sign),
                                style = MaterialTheme.typography.bodyMedium.copy(fontWeight = FontWeight.SemiBold),
                                color = gw.fg1,
                            )
                            Text(entry.rpName,
                                 style = MaterialTheme.typography.labelMedium.copy(fontWeight = FontWeight.Normal),
                                 color = gw.fg2)
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
