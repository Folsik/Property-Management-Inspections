import SwiftUI

@main
struct InspectlyApp: App {
    @StateObject private var session = SessionModel()

    var body: some Scene {
        WindowGroup {
            RootView()
                .environmentObject(session)
                .task { await session.restore() }
        }
    }
}
