#!/bin/bash
set -e

echo "========================================================"
echo " Building Pure Native macOS App (Swift + AppKit) "
echo "========================================================"

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
OUTPUT_BIN="${SCRIPT_DIR}/GeregeNexusNativeMac"

SDK_PATH=$(xcrun --show-sdk-path)

echo "[1/2] Compiling Swift source files..."
swiftc \
  -sdk "${SDK_PATH}" \
  -target arm64-apple-macosx12.0 \
  -framework AppKit \
  -framework WebKit \
  -framework Security \
  -framework LocalAuthentication \
  -framework CoreImage \
  -O \
  "${SCRIPT_DIR}/DesignSystem.swift" \
  "${SCRIPT_DIR}/NativeIPC.swift" \
  "${SCRIPT_DIR}/NativeAuth.swift" \
  "${SCRIPT_DIR}/NativeLoginViewController.swift" \
  "${SCRIPT_DIR}/NativeSettings.swift" \
  "${SCRIPT_DIR}/DeviceEnrollment.swift" \
  "${SCRIPT_DIR}/SettingsPaneViewController.swift" \
  "${SCRIPT_DIR}/MainWindowController.swift" \
  "${SCRIPT_DIR}/AppDelegate.swift" \
  "${SCRIPT_DIR}/main.swift" \
  -o "${OUTPUT_BIN}"

# No copy step for brand.png: the build writes the binary into this same
# directory, so the mark is already beside it. The app loads it from there and
# draws without it if it is missing, which is what happens when the binary is
# moved somewhere on its own.

echo "[2/2] Native binary compiled successfully at:"
echo "      ${OUTPUT_BIN}"

echo "Done! You can run: ${OUTPUT_BIN}"
