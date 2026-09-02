// swift-tools-version: 6.2
import PackageDescription

let package = Package(
    name: "GeregeShellKit",
    platforms: [.iOS(.v16), .macOS(.v13)],
    products: [
        .library(name: "GeregeShellKit", targets: ["GeregeShellKit"]),
        .library(name: "GeregeShellUI", targets: ["GeregeShellUI"]),
        .executable(name: "GeregeNexusApp", targets: ["GeregeNexusApp"]),
    ],
    targets: [
        .target(name: "GeregeShellKit"),
        .target(name: "GeregeShellUI", dependencies: ["GeregeShellKit"], resources: [.process("Resources")]),
        .executableTarget(name: "GeregeNexusApp", dependencies: ["GeregeShellKit", "GeregeShellUI"], path: "Examples"),
        .testTarget(name: "GeregeShellKitTests", dependencies: ["GeregeShellKit"]),
    ]
)
