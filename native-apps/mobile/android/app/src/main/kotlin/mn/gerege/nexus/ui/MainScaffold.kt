package mn.gerege.nexus.ui

import androidx.compose.foundation.layout.*
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.res.stringResource
import mn.gerege.nexus.AppState
import mn.gerege.nexus.R
import mn.gerege.nexus.ui.theme.LocalGw

/**
 * Нэвтэрсэн үеийн бүрхүүл — iOS-ийн `MainTabView`-тэй ижил дөрвөн зам.
 *
 * Ширээн дээр эдгээр нь хажуугийн цэсний мөр; гар дээр доод зурвас. Нэр нь
 * ижил `Nav_*` түлхүүрээс ирнэ — гурван клиент нэг зүйлийг нэг нэрээр дуудна.
 */
private enum class Tab(val labelRes: Int, val icon: ImageVector) {
    DASHBOARD(R.string.Nav_Dashboard, Icons.Filled.Home),
    ID(R.string.Nav_MyId, Icons.Filled.Badge),
    LOGS(R.string.Nav_Logs, Icons.Filled.History),
    SETTINGS(R.string.Nav_Settings, Icons.Filled.Settings),

    // Платформын ажлын муж нь ЭНЭ аппын өөрийн дэлгэц биш тул native
    // дэлгэцүүдийн ДАРАА — ширээний sidebar-т ч мөн сүүлд байдаг
    // (`DashboardTab.mainNav`). Байрлал нь хүнд аль нь аль болохыг хэлнэ.
    PLATFORM(R.string.Nav_Platform, Icons.Filled.GridView),
}

@Composable
fun MainScaffold(state: AppState) {
    val gw = LocalGw.current
    var tab by remember { mutableStateOf(Tab.DASHBOARD) }

    Scaffold(
        containerColor = gw.bg,
        bottomBar = {
            NavigationBar(containerColor = gw.surface1) {
                Tab.entries.forEach { entry ->
                    NavigationBarItem(
                        selected = tab == entry,
                        onClick = { tab = entry },
                        icon = { Icon(entry.icon, contentDescription = null) },
                        label = { Text(stringResource(entry.labelRes)) },
                        // Сонгогдсон таб нь БРЭНДИЙН цэнхэр биш, дулаан улбар
                        // шар. Дотор талын карт, товч, холбоос бүр брэндийн
                        // цэнхэр тул табыг мөн цэнхэр болговол «аль нь идэвхтэй
                        // вэ» гэдэг ялгарахаа болино (iOS дээр ижил дүрэм).
                        colors = NavigationBarItemDefaults.colors(
                            selectedIconColor = gw.accent,
                            selectedTextColor = gw.accent,
                            indicatorColor = gw.accentSoft,
                            unselectedIconColor = gw.fg3,
                            unselectedTextColor = gw.fg3,
                        ),
                    )
                }
            }
        },
    ) { padding ->
        Box(Modifier.padding(padding)) {
            when (tab) {
                Tab.DASHBOARD -> DashboardScreen(state)
                Tab.ID -> IdScreen(state)
                Tab.LOGS -> LogsScreen(state)
                Tab.SETTINGS -> SettingsScreen(state)
                Tab.PLATFORM -> PlatformScreen()
            }
        }
    }
}
