import HrsGUICore
import SwiftUI

struct ContentView: View {
    @Bindable var store: GraphStore

    var body: some View {
        VStack(spacing: 0) {
            HeaderBar(store: store)
            if let message = store.errorMessage {
                Banner(text: message, tint: .red) {
                    store.errorMessage = nil
                }
            } else if !store.connected && !store.loading {
                Banner(
                    text: "Cannot reach hrs serve at \(store.baseURLString). Start it with `hrs serve`.",
                    tint: .red
                ) {
                    Task { await store.refresh() }
                }
            } else if store.unalignedCount > 0 {
                Banner(
                    text: "\(store.unalignedCount) of \(store.openCount) open tactical goals have no strategic parent. Drag a goal onto a strategy to align it.",
                    tint: .orange
                )
            }
            HSplitView {
                VStack(alignment: .leading, spacing: 0) {
                    ColumnLegend(layout: store.layout, unalignedCount: store.unalignedCount)
                    AlignmentGraphView(store: store)
                }
                .frame(minWidth: 720)
                InspectorPane(store: store)
                    .frame(minWidth: 280, idealWidth: 320, maxWidth: 420)
            }
        }
        .background(Color(nsColor: .windowBackgroundColor))
        .task { await store.refresh() }
        .onChange(of: store.showCompletedGoals) { _, _ in
            Task { await store.refresh() }
        }
    }
}

private struct HeaderBar: View {
    @Bindable var store: GraphStore

    var body: some View {
        HStack(spacing: 12) {
            Text("hrs")
                .font(.title2.weight(.semibold))
            Text("alignment")
                .foregroundStyle(.secondary)
            Circle()
                .fill(store.connected ? Color.green : Color.red)
                .frame(width: 8, height: 8)
                .help(store.connected ? "hrs serve connected" : "hrs serve unreachable")
            Spacer()
            Toggle("Completed strategies", isOn: $store.showCompletedStrategies)
                .toggleStyle(.checkbox)
            Toggle("Completed goals", isOn: $store.showCompletedGoals)
                .toggleStyle(.checkbox)
            TextField("Filter", text: $store.query)
                .textFieldStyle(.roundedBorder)
                .frame(width: 180)
            Button {
                Task { await store.refresh() }
            } label: {
                if store.loading {
                    ProgressView()
                        .controlSize(.small)
                        .frame(width: 18, height: 18)
                } else {
                    Image(systemName: "arrow.clockwise")
                }
            }
            .buttonStyle(.borderless)
            .help("Refresh")
        }
        .padding(.horizontal, 16)
        .padding(.vertical, 10)
    }
}

private struct ColumnLegend: View {
    var layout: GraphLayout
    var unalignedCount: Int

    var body: some View {
        HStack(spacing: 0) {
            legend("Strategic", detail: "\(layout.lanes.count)", color: .accentColor)
                .frame(width: layout.metrics.strategySize.width)
                .padding(.leading, layout.metrics.padding)
            Spacer().frame(width: layout.metrics.edgeGap)
            legend("Tactical", detail: "aligned", color: .accentColor.opacity(0.7))
                .frame(width: layout.metrics.goalSize.width)
            Spacer().frame(width: layout.metrics.columnGap)
            legend("Unaligned", detail: "\(unalignedCount)", color: .orange)
                .frame(width: layout.metrics.goalSize.width)
            Spacer()
        }
        .padding(.vertical, 8)
        .padding(.trailing, 16)
        .background(.bar)
    }

    private func legend(_ title: String, detail: String, color: Color) -> some View {
        HStack(spacing: 6) {
            Circle().fill(color).frame(width: 7, height: 7)
            Text(title).font(.headline)
            Text(detail).foregroundStyle(.secondary).font(.subheadline)
        }
    }
}

private struct Banner: View {
    var text: String
    var tint: Color
    var action: (() -> Void)? = nil

    var body: some View {
        HStack(alignment: .top, spacing: 8) {
            Image(systemName: tint == .red ? "xmark.octagon.fill" : "exclamationmark.triangle.fill")
            Text(text)
                .font(.callout)
            Spacer()
            if let action {
                Button(tint == .red ? "Retry" : "OK", action: action)
                    .controlSize(.small)
            }
        }
        .foregroundStyle(tint)
        .padding(.horizontal, 16)
        .padding(.vertical, 8)
        .background(tint.opacity(0.12))
    }
}
