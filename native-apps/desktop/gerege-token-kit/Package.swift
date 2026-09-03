// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "GeregeTokenKit",
    platforms: [
        .macOS(.v14),
        .iOS(.v17)
    ],
    products: [
        .library(name: "GeregeTokenKit", targets: ["GeregeTokenKit"])
    ],
    targets: [
        .target(
            name: "GeregeTokenKit",
            path: "Sources/GeregeTokenKit"
        ),
        .testTarget(
            name: "GeregeTokenKitTests",
            dependencies: ["GeregeTokenKit"],
            path: "Tests"
        )
    ]
)
