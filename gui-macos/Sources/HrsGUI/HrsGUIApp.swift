import AppKit
import HrsGUICore
import SwiftUI

@main
struct HrsGUIApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) private var appDelegate
    @State private var store = GraphStore()

    var body: some Scene {
        WindowGroup("hrs") {
            ContentView(store: store)
        }
        .defaultSize(width: 1440, height: 900)
        .commands {
            CommandGroup(after: .sidebar) {
                Button("Refresh") {
                    Task { await store.refresh() }
                }
                .keyboardShortcut("r", modifiers: .command)
            }
        }
    }
}

final class AppDelegate: NSObject, NSApplicationDelegate {
    func applicationDidFinishLaunching(_ notification: Notification) {
        NSApp.setActivationPolicy(.regular)
        NSApp.activate(ignoringOtherApps: true)
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
        true
    }
}
