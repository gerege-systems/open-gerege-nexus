package mn.gerege.nexus

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.viewModels
import androidx.compose.runtime.getValue
import mn.gerege.nexus.ui.LoginScreen
import mn.gerege.nexus.ui.MainScaffold
import mn.gerege.nexus.ui.theme.EidTheme

/**
 * Ганц Activity — ширээний «нэг цонх» дүрмийн гар дээрх хэлбэр. Нэвтрэлт,
 * ажлын муж, тохиргоо бүгд НЭГ хүрээн дотор солигдоно, шинэ цонх нээгдэхгүй.
 * Гадагш гардаг цорын ганц зүйл нь eID Mongolia апп руу үсрэх app-to-app.
 */
class MainActivity : ComponentActivity() {
    private val state: AppState by viewModels()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        AppConfig.init(this)
        setContent {
            EidTheme {
                when (state.screen) {
                    AppState.Screen.LOGIN -> LoginScreen(state)
                    AppState.Screen.DASHBOARD -> MainScaffold(state)
                }
            }
        }
    }
}
