package mn.gerege.nexus.ui

import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsFocusedAsState
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Public
import androidx.compose.material3.LocalTextStyle
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import mn.gerege.nexus.AppConfig
import mn.gerege.nexus.AppState
import mn.gerege.nexus.R
import mn.gerege.nexus.ui.components.*
import mn.gerege.nexus.ui.theme.LocalGw

/**
 * Тохиргоо — iOS-ийн `MobileSettingsPage`-тай ижил хүрээ.
 *
 * Хэлийг Android дээр СИСТЕМ шийднэ (`res/values-*`): аппын дотор хэлний
 * сонголт тавихгүй байгаа нь ялгаа биш, платформын зөв зан. iOS дээр
 * ширээнээс өвлөсөн `LocalizationService` нь өөрөө хэл сольдог.
 */
@Composable
fun SettingsScreen(state: AppState) {
    val gw = LocalGw.current
    var server by remember { mutableStateOf(AppConfig.prefs().getString(AppConfig.BASE_URL_KEY, "").orEmpty()) }
    val interaction = remember { MutableInteractionSource() }
    val focused by interaction.collectIsFocusedAsState()

    EidScreen(title = stringResource(R.string.Nav_Settings)) {
        EidCard {
            Text(stringResource(R.string.Settings_Server),
                 style = MaterialTheme.typography.titleMedium, color = gw.fg1)
            BrandInputCard(leadingIcon = Icons.Filled.Public, isFocused = focused) {
                BasicTextField(
                    value = server,
                    onValueChange = { server = it; AppConfig.baseUrl = it },
                    singleLine = true,
                    interactionSource = interaction,
                    textStyle = LocalTextStyle.current.copy(
                        fontFamily = FontFamily.Monospace, color = gw.fg1),
                    cursorBrush = androidx.compose.ui.graphics.SolidColor(gw.brand),
                    modifier = Modifier.fillMaxWidth(),
                    decorationBox = { inner ->
                        if (server.isEmpty()) {
                            Text(AppConfig.DEFAULT_BASE_URL,
                                 style = MaterialTheme.typography.bodyMedium.copy(fontFamily = FontFamily.Monospace),
                                 color = gw.fg4)
                        }
                        inner()
                    },
                )
            }
            Text(AppConfig.DEFAULT_BASE_URL,
                 style = MaterialTheme.typography.labelSmall.copy(fontWeight = FontWeight.Normal),
                 color = gw.fg3)
        }
        EidCard {
            Text(stringResource(R.string.Settings_About),
                 style = MaterialTheme.typography.titleMedium, color = gw.fg1)
            EidField(stringResource(R.string.App_VersionLabel), "${AppConfig.BRAND_NAME} v1.0.0", mono = true)
            EidField(stringResource(R.string.Settings_Host), AppConfig.host, mono = true)
        }
        SecondaryButton(stringResource(R.string.Nav_Logout), tone = gw.debit) { state.logout() }
    }
}
