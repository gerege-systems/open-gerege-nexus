import SwiftUI

/// Windows `DashboardPage` port — citizen overview hero + 4-stat grid +
/// trusted devices repeater + activity history. Pulls `/dashboard` from
/// the backend (already loaded by AppState).
struct DashboardPageView: View {
    @EnvironmentObject private var appState: AppState
    @ObservedObject private var loc = LocalizationService.shared

    @State private var isRefreshing = false

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 20) {
                heroCard
                statsGrid
                devicesCard
                activityCard
            }
            .padding(.horizontal, Space.pageHoriz)
            .padding(.top, Space.pageTop)
            .padding(.bottom, Space.pageBottom)
            .frame(maxWidth: .infinity, alignment: .topLeading)
        }
        .background(Color.eidSurface)
    }

    // MARK: - Hero (avatar + greeting + status badge + refresh)

    private var heroCard: some View {
        AppCard {
            HStack(alignment: .top, spacing: 16) {
                UserAvatar(photo: appState.dashboardData?.user.photo,
                           initials: initials, size: 64)

                VStack(alignment: .leading, spacing: 8) {
                    HStack(spacing: 12) {
                        VStack(alignment: .leading, spacing: 2) {
                            Text(loc.t("Dashboard_Wordmark"))
                                .font(.system(size: 11, weight: .bold))
                                .foregroundStyle(Color.eidAccent)
                                .kerning(0.8)
                            Text("Digital Sovereignty")
                                .font(.system(size: 10))
                                .foregroundStyle(Color.eidMuted)
                        }
                        HStack(spacing: 6) {
                            StatusDot(color: Color.eidSuccess, size: 8)
                            Text(loc.t("Dashboard_StatusBadge"))
                                .font(.system(size: 12, weight: .semibold))
                                .foregroundStyle(Color.eidAccent)
                        }
                        .padding(.horizontal, 10)
                        .padding(.vertical, 4)
                        .background(Color.eidAccentSubtle)
                        .clipShape(RoundedRectangle(cornerRadius: 16, style: .continuous))
                        .overlay(
                            RoundedRectangle(cornerRadius: 16, style: .continuous)
                                .strokeBorder(Color.eidAccent.opacity(0.35), lineWidth: 1)
                        )
                    }

                    Text("\(loc.t("Dashboard_Greeting")) \(appState.fullName.isEmpty ? loc.t("Common_User") : appState.fullName)!")
                        .font(.eidPageTitle)
                        .foregroundStyle(Color.textPrimary)

                    Text(loc.t("Dashboard_Subtitle"))
                        .subtleSubtitleStyle()
                }

                Spacer(minLength: 0)

                Button {
                    refresh()
                } label: {
                    HStack(spacing: 6) {
                        if isRefreshing {
                            ProgressView().controlSize(.small)
                        } else {
                            Image(systemName: "arrow.clockwise")
                                .font(.system(size: 12))
                        }
                        Text(loc.t("Dashboard_Refresh"))
                    }
                }
                .buttonStyle(.secondary)
            }
        }
    }

    // MARK: - 4 stat cards

    private var statsGrid: some View {
        let cols = [
            GridItem(.flexible(), spacing: 16),
            GridItem(.flexible(), spacing: 16),
            GridItem(.flexible(), spacing: 16),
            GridItem(.flexible(), spacing: 16),
        ]
        return LazyVGrid(columns: cols, spacing: 16) {
            statCard(title: loc.t("Dashboard_Stats_SecurityScore"), value: "98%",
                     accent: false)
            statCard(title: loc.t("Dashboard_Stats_Certificates"),
                     value: "\(appState.dashboardData?.certificates ?? 0)",
                     accent: false)
            statCard(title: loc.t("Dashboard_Stats_Logins"),
                     value: "\(appState.dashboardData?.totalLogins ?? 0)",
                     accent: false)
            statCard(title: loc.t("Dashboard_Stats_LastLogin"),
                     value: lastLoginValue,
                     accent: true)
        }
    }

    private func statCard(title: String, value: String, accent: Bool) -> some View {
        AppCard(padding: 18) {
            VStack(alignment: .leading, spacing: 8) {
                Text(title)
                    .font(.eidStatLabel)
                    .foregroundStyle(Color.eidMuted)
                    .textCase(.uppercase)
                    .kerning(0.6)
                Text(value)
                    .font(.eidStatValue)
                    .foregroundStyle(accent ? Color.eidAccent : Color.textPrimary)
                    .lineLimit(1)
                    .minimumScaleFactor(0.6)
            }
        }
    }

    private var lastLoginValue: String {
        guard let session = appState.dashboardData?.sessions.first else { return "—" }
        return String(session.createdAt.prefix(16))
    }

    // MARK: - Devices

    private var devicesCard: some View {
        AppCard {
            VStack(alignment: .leading, spacing: 12) {
                Text(loc.t("Dashboard_Devices_Section"))
                    .font(.eidSectionTitle)
                    .foregroundStyle(Color.textPrimary)

                let devices = appState.dashboardData?.devices ?? []

                if devices.isEmpty {
                    Text(loc.t("Dashboard_Devices_Empty"))
                        .labelStyle()
                } else {
                    ForEach(devices) { device in
                        deviceRow(device)
                    }
                }
            }
        }
    }

    private func deviceRow(_ device: DashboardDevice) -> some View {
        let isActive = device.status.uppercased() == "ACTIVE"
        let isIOS = device.platform.uppercased() == "IOS"
        let platformLabel: String = {
            switch device.platform.uppercased() {
            case "IOS": return "iOS"
            case "ANDROID": return "Android"
            default: return device.platform
            }
        }()

        return HStack(spacing: 12) {
            ZStack {
                RoundedRectangle(cornerRadius: 10)
                    .fill((isIOS ? Color.eidAccent : Color.eidMuted).opacity(0.12))
                    .frame(width: 38, height: 38)
                Image(systemName: "iphone")
                    .font(.system(size: 16))
                    .foregroundStyle(isIOS ? Color.eidAccent : Color.eidMuted)
            }
            VStack(alignment: .leading, spacing: 2) {
                Text("\(platformLabel) · \(String(device.deviceId.prefix(8)))…")
                    .font(.eidBody)
                    .foregroundStyle(Color.textPrimary)
                Text("\(loc.t("Dashboard_Device_Registered")): \(String(device.createdAt.prefix(10)))")
                    .font(.eidLabel)
                    .foregroundStyle(Color.eidMuted)
            }
            Spacer()
            StatusPill(isActive ? loc.t("Dashboard_Device_Active") : loc.t("Dashboard_Device_Inactive"),
                       variant: isActive ? .ok : .neutral)
        }
        .padding(12)
        .background(Color.eidCardBackground)
        .clipShape(RoundedRectangle(cornerRadius: Radius.md, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: Radius.md, style: .continuous)
                .strokeBorder(Color.eidCardStroke, lineWidth: 1)
        )
    }

    // MARK: - Activity

    private var activityCard: some View {
        AppCard {
            VStack(alignment: .leading, spacing: 12) {
                Text(loc.t("Dashboard_Activity_Section"))
                    .font(.eidSectionTitle)
                    .foregroundStyle(Color.textPrimary)

                let sessions = appState.dashboardData?.sessions ?? []

                if sessions.isEmpty {
                    Text(loc.t("Dashboard_Activity_Empty"))
                        .labelStyle()
                } else {
                    ForEach(Array(sessions.prefix(8).enumerated()), id: \.element.id) { idx, session in
                        activityRow(session)
                        if idx < min(sessions.count, 8) - 1 {
                            Divider().background(Color.eidCardStroke)
                        }
                    }
                }
            }
        }
    }

    private func activityRow(_ session: DashboardSession) -> some View {
        let kind = session.sessionType.uppercased() == "AUTH" ? loc.t("Dashboard_Activity_Auth") : loc.t("Dashboard_Activity_Sign")
        let ok = session.result.uppercased() == "OK"
        return HStack(alignment: .top, spacing: 12) {
            VStack(alignment: .leading, spacing: 4) {
                HStack(spacing: 8) {
                    Text(kind)
                        .font(.eidBody)
                        .foregroundStyle(Color.textPrimary)
                        .fontWeight(.semibold)
                    if !session.rpName.isEmpty {
                        Text("· \(session.rpName)")
                            .font(.eidLabel)
                            .foregroundStyle(Color.eidMuted)
                    }
                }
                Text(String(session.createdAt.prefix(16)))
                    .font(.eidLabel)
                    .foregroundStyle(Color.eidMuted)
            }
            Spacer()
            StatusPill(ok ? loc.t("Dashboard_Activity_Success") : loc.t("Dashboard_Activity_Failure"),
                       variant: ok ? .ok : .bad)
        }
        .padding(.vertical, 8)
    }

    // MARK: - Helpers

    private var initials: String {
        appState.fullName
            .split(separator: " ")
            .compactMap { $0.first.map(String.init) }
            .prefix(2)
            .joined()
            .uppercased()
    }

    private func refresh() {
        // v3: dashboard нь локал өгөгдөл (identity + activity log) тул зөвхөн
        // товч анимац — өгөгдөл нь session бүртгэгдэх бүрт шинэчлэгддэг.
        isRefreshing = true
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.8) {
            isRefreshing = false
        }
    }
}
