import SwiftUI

/// Windows `HomePage` port — backend health monitor.
/// Hero (logo + title) + status card (variant pill + version + last-checked + refresh).
struct HomeView: View {
    @EnvironmentObject private var appState: AppState
    @ObservedObject private var loc = LocalizationService.shared

    @State private var statusVariant: StatusPill.Variant = .neutral
    @State private var statusKey: String = "Home_Status_Checking"
    @State private var version: String = "—"
    @State private var lastChecked: String = "—"
    @State private var isLoading = false
    @State private var refreshTask: Task<Void, Never>?

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 24) {
                heroHeader

                backendStatusCard

                // Дэлгэцийн доод хэсэгт quick-card-ууд — Windows HomePage дээр байхгүй,
                // гэхдээ macOS-д хэрэгтэй гэж үзвэл DashboardPageView дотроо харагдана.
            }
            .padding(.horizontal, Space.pageHoriz)
            .padding(.top, Space.pageTop)
            .padding(.bottom, Space.pageBottom)
            .frame(maxWidth: .infinity, alignment: .topLeading)
        }
        .background(Color.eidSurface)
        .onAppear { refresh() }
        .onDisappear { refreshTask?.cancel() }
    }

    // MARK: - Hero (logo + title + subtitle)

    private var heroHeader: some View {
        HStack(spacing: 20) {
            Image("Logo")
                .resizable()
                .aspectRatio(contentMode: .fit)
                .frame(width: 72, height: 72)
                .clipShape(RoundedRectangle(cornerRadius: 14, style: .continuous))
                .overlay(
                    RoundedRectangle(cornerRadius: 14, style: .continuous)
                        .strokeBorder(Color.eidCardStroke, lineWidth: 1)
                )
            VStack(alignment: .leading, spacing: 8) {
                Text(loc.t("Home_Hero_Title"))
                    .heroTitleStyle()
                Text(loc.t("Home_Subtitle"))
                    .subtleSubtitleStyle()
            }
            Spacer(minLength: 0)
        }
    }

    // MARK: - Backend status (Windows HomePage AppCard)

    private var backendStatusCard: some View {
        AppCard {
            VStack(alignment: .leading, spacing: 8) {
                HStack(alignment: .center, spacing: 12) {
                    StatusDot(color: variantColor(statusVariant), size: 14)
                    Text(loc.t("Home_Backend_Title"))
                        .font(.eidSectionTitle)
                        .foregroundStyle(Color.textPrimary)
                    Spacer()
                    Button {
                        refresh()
                    } label: {
                        HStack(spacing: 6) {
                            if isLoading { ProgressView().controlSize(.small) }
                            Image(systemName: "arrow.clockwise")
                                .font(.system(size: 12))
                            Text(loc.t("Home_Backend_Refresh"))
                        }
                    }
                    .buttonStyle(.secondary)
                }

                Text(loc.t(statusKey))
                    .pageTitleStyle()
                    .padding(.leading, 26)

                Text("\(loc.t("Home_Backend_Version")): \(version)")
                    .labelStyle()
                    .padding(.leading, 26)

                Text("\(loc.t("Home_Backend_LastChecked")): \(lastChecked)")
                    .labelStyle()
                    .padding(.leading, 26)
            }
        }
    }

    private func variantColor(_ variant: StatusPill.Variant) -> Color {
        switch variant {
        case .ok:      return Color.eidSuccess
        case .warn:    return Color.eidWarning
        case .bad:     return Color.eidDanger
        case .accent:  return Color.eidAccent
        case .neutral: return Color.eidMuted
        }
    }

    // MARK: - Backend health probe

    private func refresh() {
        refreshTask?.cancel()
        isLoading = true
        statusVariant = .neutral
        statusKey = "Home_Status_Checking"

        refreshTask = Task {
            let snapshot = await probeBackend()
            await MainActor.run {
                isLoading = false
                statusVariant = snapshot.variant
                statusKey     = snapshot.key
                version       = snapshot.version
                let formatter = DateFormatter()
                formatter.dateFormat = "HH:mm:ss"
                lastChecked = formatter.string(from: Date())
            }
        }
    }

    private struct HealthSnapshot {
        let variant: StatusPill.Variant
        let key: String
        let version: String
    }

    /// v3 backend health probe — `GET /actuator/health` (нээлттэй, Bearer
    /// шаардахгүй). 200 {"status":"UP"} → эрүүл.
    private func probeBackend() async -> HealthSnapshot {
        let ok = await APIClient.shared.healthOK()
        return ok
            ? HealthSnapshot(variant: .ok, key: "Home_Status_Healthy", version: "—")
            : HealthSnapshot(variant: .bad, key: "Home_Status_Unreachable", version: "—")
    }
}
