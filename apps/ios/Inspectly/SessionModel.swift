import Foundation
import Combine

@MainActor
final class SessionModel: ObservableObject {
    @Published private(set) var isSignedIn = false
    @Published private(set) var isBusy = false
    @Published private(set) var profile: UserProfile?
    @Published private(set) var organizations: [Organization] = []
    @Published private(set) var message: String?

    private let client = APIClient()
    private var didRestore = false

    func restore() async {
        guard !didRestore else { return }
        didRestore = true
        isBusy = true
        defer { isBusy = false }
        do {
            try await client.restore()
            profile = try await client.profile()
            isSignedIn = true
            await loadOrganizations()
        } catch APIError.noSession {
            isSignedIn = false
        } catch {
            // A network outage is not necessarily a revoked session. The
            // Keychain token stays intact unless the API confirms revocation.
            message = error.localizedDescription
        }
    }

    func signIn(identifier: String, password: String) async {
        isBusy = true
        message = nil
        defer { isBusy = false }
        do {
            try await client.signIn(identifier: identifier, password: password)
            profile = try await client.profile()
            isSignedIn = true
            await loadOrganizations()
        } catch {
            message = error.localizedDescription
        }
    }

    func loadOrganizations() async {
        guard isSignedIn else { return }
        do {
            organizations = try await client.organizations()
            message = nil
        } catch APIError.unauthorized {
            isSignedIn = false
            profile = nil
            organizations = []
            message = "Your device session has expired. Please sign in again."
        } catch {
            message = error.localizedDescription
        }
    }

    func signOut() async {
        isBusy = true
        defer { isBusy = false }
        do {
            try await client.signOut()
            isSignedIn = false
            profile = nil
            organizations = []
            message = nil
        } catch {
            message = error.localizedDescription
        }
    }
}
