// Spacing and radius scales — the wallet's, which are in turn a port of its
// iOS Theme.Space and Theme.Radius. A 4dp base with roughly 25% steps: pick
// from the menu, do not invent. The same numbers are in the iOS client's
// DesignSystem.swift, so a measurement means the same thing on both.

package mn.gerege.nexus.ui.theme

import androidx.compose.ui.unit.dp

object Space {
    val xxs = 2.dp
    val xs = 4.dp
    val sm = 8.dp
    val md = 12.dp
    val lg = 16.dp
    val xl = 24.dp
    val xxl = 32.dp
    val xxxl = 48.dp
    val huge = 64.dp
}

object Radius {
    val sm = 8.dp
    val md = 12.dp
    val lg = 16.dp
    val xl = 20.dp
    val xxl = 24.dp
}
