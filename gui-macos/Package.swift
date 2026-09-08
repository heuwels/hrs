// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "HrsGUI",
    platforms: [
        .macOS(.v14),
    ],
    products: [
        .executable(name: "HrsGUI", targets: ["HrsGUI"]),
        .library(name: "HrsGUICore", targets: ["HrsGUICore"]),
    ],
    targets: [
        .target(
            name: "HrsGUICore",
            path: "Sources/HrsGUICore"
        ),
        .executableTarget(
            name: "HrsGUI",
            dependencies: ["HrsGUICore"],
            path: "Sources/HrsGUI"
        ),
        .testTarget(
            name: "HrsGUITests",
            dependencies: ["HrsGUICore"],
            path: "Tests/HrsGUITests"
        ),
    ]
)
