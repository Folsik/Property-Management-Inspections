import SwiftUI

struct RootView: View {
    @EnvironmentObject private var session: SessionModel
    @State private var identifier = ""
    @State private var password = ""

    var body: some View {
        NavigationStack {
            Group {
                if session.isSignedIn {
                    home
                } else {
                    signIn
                }
            }
            .navigationTitle("Inspectly")
        }
        .tint(.blue)
    }

    private var signIn: some View {
        Form {
            Section("Sign in") {
                TextField("Email or phone number", text: $identifier)
                    .textInputAutocapitalization(.never)
                    .keyboardType(.emailAddress)
                    .textContentType(.username)
                    .autocorrectionDisabled()
                SecureField("Password", text: $password)
                    .textContentType(.password)
                Button {
                    Task {
                        await session.signIn(identifier: identifier, password: password)
                        if session.isSignedIn { password = "" }
                    }
                } label: {
                    if session.isBusy {
                        ProgressView()
                    } else {
                        Text("Sign in")
                    }
                }
                .disabled(session.isBusy || identifier.isEmpty || password.isEmpty)
            }
            Section {
                Text("Use the account you created in the web application. Device credentials are protected by iOS Keychain.")
                    .font(.footnote)
                    .foregroundStyle(.secondary)
            }
            if let error = session.message {
                Section {
                    Text(error).foregroundStyle(.red)
                }
            }
        }
    }

    private var home: some View {
        List {
            Section("Account") {
                Text(session.profile?.fullName.isEmpty == false ? session.profile!.fullName : "Signed in")
                if let phone = session.profile?.phone { Text(phone).foregroundStyle(.secondary) }
            }
            Section("Organizations") {
                ForEach(session.organizations) { org in
                    VStack(alignment: .leading) {
                        Text(org.name).font(.headline)
                        Text(org.role.capitalized).font(.caption).foregroundStyle(.secondary)
                    }
                }
                if session.organizations.isEmpty {
                    Text("No organizations yet. Create one in the web application.")
                        .foregroundStyle(.secondary)
                }
                Button("Refresh") { Task { await session.loadOrganizations() } }
            }
            Section {
                Button("Sign out", role: .destructive) { Task { await session.signOut() } }
            }
            if let error = session.message {
                Section {
                    Text(error).foregroundStyle(.red)
                }
            }
        }
        .refreshable { await session.loadOrganizations() }
    }
}
