#!/bin/zsh
# Gerege macOS — Obfuscated Release Build
# View/UI нэрүүдийг short string-ээр солиод Release build хийнэ.
set -e

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
MONO_ROOT="$(cd "$PROJECT_DIR/../.." && pwd)"
BUILD_DIR="/tmp/gerege-obfuscated"
TEAM="CQTHTD6YJQ"
SIGN_ID="Developer ID Application: Gerege Systems LLC (CQTHTD6YJQ)"

echo "=== Gerege Obfuscated Build ==="

# 1. Copy source
rm -rf "$BUILD_DIR"
cp -R "$PROJECT_DIR" "$BUILD_DIR"
cd "$BUILD_DIR"

# 2. Fix GeregeTokenKit path
sed -i '' "s|path: ../../gerege-token-kit|path: $MONO_ROOT/gerege-token-kit|" "$BUILD_DIR/project.yml"

# 3. Obfuscate — зөвхөн app-ийн өөрийн нэрүүд, word boundary-тай
# Framework нэрүүдтэй давхцахгүй нэрүүдийг л солино
echo "Obfuscating UI names..."

# View names (UI layer)
SUBS=(
    "s/[[:<:]]ContentView[[:>:]]/G1v/g"
    "s/[[:<:]]LoginView[[:>:]]/G2v/g"
    "s/[[:<:]]HomeView[[:>:]]/G3v/g"
    "s/[[:<:]]DashboardView[[:>:]]/G4v/g"
    "s/[[:<:]]IDCardView[[:>:]]/G5v/g"
    "s/[[:<:]]LogsView[[:>:]]/G6v/g"
    "s/[[:<:]]DevicesView[[:>:]]/G7v/g"
    "s/[[:<:]]SecurityView[[:>:]]/G8v/g"
    "s/[[:<:]]ChildrenView[[:>:]]/G9v/g"
    "s/[[:<:]]TokenTabView[[:>:]]/GAv/g"
    "s/[[:<:]]SignView[[:>:]]/GBv/g"
    "s/[[:<:]]SettingsView[[:>:]]/GCv/g"
    "s/[[:<:]]OrgView[[:>:]]/GDv/g"
    "s/[[:<:]]CloudSignView[[:>:]]/GEv/g"
    "s/[[:<:]]UpdateInfoView[[:>:]]/GFv/g"
    "s/[[:<:]]CheckForUpdatesView[[:>:]]/GGv/g"
    "s/[[:<:]]UserAvatar[[:>:]]/GHv/g"
    "s/[[:<:]]StatusBadge[[:>:]]/GIv/g"
    # State/Model (app-specific, framework-д байхгүй нэрүүд)
    "s/[[:<:]]AppState[[:>:]]/G1s/g"
    "s/[[:<:]]AppScreen[[:>:]]/G2s/g"
    "s/[[:<:]]DashboardTab[[:>:]]/G3s/g"
    "s/[[:<:]]DashboardData[[:>:]]/G1d/g"
    "s/[[:<:]]DashboardUser[[:>:]]/G2d/g"
    "s/[[:<:]]DashboardDevice[[:>:]]/G3d/g"
    "s/[[:<:]]DashboardSession[[:>:]]/G4d/g"
    "s/[[:<:]]AuthInitResponse[[:>:]]/G1r/g"
    "s/[[:<:]]AuthStatusResponse[[:>:]]/G2r/g"
    "s/[[:<:]]QRInitResponse[[:>:]]/G3r/g"
    "s/[[:<:]]QRStatusResponse[[:>:]]/G4r/g"
    "s/[[:<:]]CheckForUpdatesViewModel[[:>:]]/G1m/g"
    "s/[[:<:]]UpdateViewModel[[:>:]]/G2m/g"
    "s/[[:<:]]OrgItem[[:>:]]/G1o/g"
    "s/[[:<:]]OrgMemberItem[[:>:]]/G2o/g"
    "s/[[:<:]]OrgLookupResult[[:>:]]/G3o/g"
    "s/[[:<:]]OrgListResponse[[:>:]]/G4o/g"
    "s/[[:<:]]OrgLookupResponse[[:>:]]/G5o/g"
    "s/[[:<:]]OrgRegisterResponse[[:>:]]/G6o/g"
    "s/[[:<:]]OrgMembersResponse[[:>:]]/G7o/g"
    "s/[[:<:]]ProvisionStep[[:>:]]/G1e/g"
    "s/[[:<:]]LoginTab[[:>:]]/G2e/g"
    "s/[[:<:]]AuthState[[:>:]]/G3e/g"
    "s/[[:<:]]KeychainKey[[:>:]]/G4e/g"
    "s/[[:<:]]KeychainError[[:>:]]/G5e/g"
)

# Apply to Swift files (skip GeregeTokenKit)
for sub in "${SUBS[@]}"; do
    find "$BUILD_DIR" -name "*.swift" -not -path "*/gerege-token-kit/*" -exec sed -i '' "$sub" {} +
done

# 4. Build
echo "Building Release..."
xcodegen generate 2>&1 | tail -1
xcodebuild archive \
    -project Gerege.xcodeproj -scheme Gerege -configuration Release \
    -archivePath /tmp/Gerege-obf.xcarchive \
    CODE_SIGN_IDENTITY="$SIGN_ID" \
    DEVELOPMENT_TEAM="$TEAM" \
    CODE_SIGN_STYLE=Manual \
    -allowProvisioningUpdates 2>&1 | tail -3

echo ""
BIN="/tmp/Gerege-obf.xcarchive/Products/Applications/Gerege.app/Contents/MacOS/Gerege"
if [ -f "$BIN" ]; then
    echo "=== Original names still visible? ==="
    FOUND=$(strings "$BIN" | grep -oE '(AppState|LoginView|TokenManager|OrgView|DashboardView|HomeView|SecurityView|APIClient)' | sort -u)
    if [ -z "$FOUND" ]; then
        echo "✓ None! Бүгд obfuscate хийгдсэн."
    else
        echo "$FOUND"
    fi
    echo ""
    echo "=== Obfuscated names ==="
    strings "$BIN" | grep -oE 'G[0-9A-H][a-z]' | sort -u
    echo ""
    echo "✓ Archive: /tmp/Gerege-obf.xcarchive"
else
    echo "✗ Build failed"
fi

rm -rf "$BUILD_DIR"
