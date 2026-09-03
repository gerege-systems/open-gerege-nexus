package mn.gerege.nexus.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Business
import androidx.compose.material.icons.filled.ChildCare
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import mn.gerege.nexus.AppState
import mn.gerege.nexus.R
import mn.gerege.nexus.ui.components.*
import mn.gerege.nexus.ui.theme.LocalGw
import mn.gerege.nexus.ui.theme.Space

/** Миний ID — iOS-ийн `MobileIdPage`-тай ижил талбарууд, ижил дараалал. */
@Composable
fun IdScreen(state: AppState) {
    val gw = LocalGw.current
    val identity = state.identity
    EidScreen(title = stringResource(R.string.Nav_MyId), subtitle = stringResource(R.string.Id_Subtitle)) {
        if (identity == null) {
            EidCard {
                Text(stringResource(R.string.Dashboard_Activity_Empty),
                     style = MaterialTheme.typography.bodySmall, color = gw.fg3)
            }
            return@EidScreen
        }
        EidCard(spacing = Space.lg) {
            Row(verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(Space.md)) {
                Text(
                    initials(identity.fullName),
                    style = MaterialTheme.typography.titleLarge,
                    color = gw.brand,
                    modifier = Modifier
                        .size(64.dp)
                        .background(gw.brandSoft, CircleShape)
                        .wrapContentSize(Alignment.Center),
                )
                Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(Space.sm)) {
                    Text(identity.fullName.ifEmpty { "—" },
                         style = MaterialTheme.typography.titleLarge, color = gw.fg1)
                    StatusPill(stringResource(R.string.Common_Active), PillVariant.OK)
                }
            }
            HorizontalDivider(color = gw.divider)
            EidField(stringResource(R.string.Id_RegNumber), identity.nationalId, mono = true)
            if (identity.civilId.isNotEmpty()) {
                EidField(stringResource(R.string.Id_CivilId), identity.civilId, mono = true)
            }
        }
        EidCard {
            BrandSectionLabel(stringResource(R.string.Id_Certificate))
            EidField(stringResource(R.string.Id_Level), identity.certificateLevel)
            EidField(stringResource(R.string.Id_DocNumber), identity.documentNumber, mono = true)
        }
        if (state.organizations.isNotEmpty()) {
            EidCard {
                Text(stringResource(R.string.Nav_MyOrganizations),
                     style = MaterialTheme.typography.titleMedium, color = gw.fg1)
                state.organizations.forEach { org ->
                    ListRow(Icons.Filled.Business, org.orgName, org.orgRegister) {
                        StatusPill(org.rightType, PillVariant.ACCENT)
                    }
                }
                Text(stringResource(R.string.Read_Only_Hint),
                     style = MaterialTheme.typography.labelSmall.copy(fontWeight = FontWeight.Normal),
                     color = gw.fg3)
            }
        }
        if (state.children.isNotEmpty()) {
            EidCard {
                Text(stringResource(R.string.Nav_Children),
                     style = MaterialTheme.typography.titleMedium, color = gw.fg1)
                state.children.forEach { child ->
                    ListRow(Icons.Filled.ChildCare, child.name, child.regNo) {
                        StatusPill(
                            stringResource(if (child.registered) R.string.Common_Registered else R.string.Common_Pending),
                            if (child.registered) PillVariant.OK else PillVariant.WARN,
                        )
                    }
                }
                Text(stringResource(R.string.Read_Only_Hint),
                     style = MaterialTheme.typography.labelSmall.copy(fontWeight = FontWeight.Normal),
                     color = gw.fg3)
            }
        }
    }
}

/** Байгууллага ба хүүхдийн мөр нэг л хэлбэртэй — дүрс, нэр, дугаар, капсул. */
@Composable
private fun ListRow(icon: ImageVector, title: String, subtitle: String, trailing: @Composable () -> Unit) {
    val gw = LocalGw.current
    Row(verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(Space.md),
        modifier = Modifier.fillMaxWidth()) {
        Box(
            modifier = Modifier.size(30.dp).background(gw.brandSoft, CircleShape),
            contentAlignment = Alignment.Center,
        ) {
            Icon(icon, null, tint = gw.brand, modifier = Modifier.size(15.dp))
        }
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Text(title, style = MaterialTheme.typography.bodySmall.copy(fontWeight = FontWeight.SemiBold),
                 color = gw.fg1)
            Text(subtitle,
                 style = MaterialTheme.typography.labelSmall.copy(
                     fontFamily = FontFamily.Monospace, fontWeight = FontWeight.Normal),
                 color = gw.fg3)
        }
        trailing()
    }
}
