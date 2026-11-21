import SwiftUI

struct RootView: View {
    @EnvironmentObject private var session: SessionModel
    @State private var identifier = ""
    @State private var password = ""

    var body: some View {
        Group {
            if session.isSignedIn {
                NavigationStack {
                    organizationPicker
                }
            } else {
                NavigationStack {
                    signIn
                }
            }
        }
        .tint(.blue)
    }

    private var signIn: some View {
        Form {
            Section {
                VStack(alignment: .leading, spacing: 8) {
                    Image(systemName: "checkmark.shield.fill")
                        .font(.system(size: 48))
                        .foregroundStyle(.blue)
                        .padding(.bottom, 6)
                    Text("Inspections, simplified")
                        .font(.title2.bold())
                    Text("Securely manage properties, inspections and issues from your iPhone.")
                        .foregroundStyle(.secondary)
                }
                .padding(.vertical, 12)
            }
            Section("Sign in") {
                TextField("Email or phone number", text: $identifier)
                    .textInputAutocapitalization(.never)
                    .keyboardType(.emailAddress)
                    .textContentType(.username)
                    .autocorrectionDisabled()
                SecureField("Password", text: $password)
                    .textContentType(.password)
                    .onSubmit { Task { await signInNow() } }
                Button {
                    Task { await signInNow() }
                } label: {
                    HStack {
                        Spacer()
                        if session.isBusy { ProgressView() }
                        else { Text("Sign in").fontWeight(.semibold) }
                        Spacer()
                    }
                }
                .disabled(session.isBusy || identifier.isEmpty || password.isEmpty)
            }
            Section {
                Text("Use an existing account created in the web application. Phone OTP registration will be added in the next iOS increment.")
                    .font(.footnote)
                    .foregroundStyle(.secondary)
            }
            if let error = session.message {
                Section {
                    Text(error).foregroundStyle(.red)
                }
            }
        }
        .navigationTitle("Inspectly")
    }

    private func signInNow() async {
        await session.signIn(identifier: identifier, password: password)
        if session.isSignedIn { password = "" }
    }

    private var organizationPicker: some View {
        List {
            Section {
                Text("Choose a workspace")
                    .font(.title2.bold())
                    .listRowBackground(Color.clear)
            }
            Section("Organizations") {
                ForEach(session.organizations) { org in
                    NavigationLink {
                        WorkspaceView(organization: org, api: session.api)
                            .navigationBarBackButtonHidden(false)
                    } label: {
                        HStack(spacing: 12) {
                            Image(systemName: org.kind == "individual" ? "person.crop.square" : "building.2")
                                .font(.title2)
                                .foregroundStyle(.blue)
                                .frame(width: 42, height: 42)
                                .background(.blue.opacity(0.08), in: RoundedRectangle(cornerRadius: 10))
                            VStack(alignment: .leading) {
                                Text(org.name).font(.headline)
                                Text(org.role.capitalized + " · " + org.country)
                                    .font(.caption).foregroundStyle(.secondary)
                            }
                        }
                        .padding(.vertical, 4)
                    }
                }
                if session.organizations.isEmpty {
                    ContentUnavailableView("No organizations", systemImage: "building.2",
                        description: Text("Create an organization in the web application first."))
                }
            }
            if let message = session.message {
                Section { Text(message).foregroundStyle(.red) }
            }
            Section {
                Button("Sign out", role: .destructive) {
                    Task { await session.signOut() }
                }
            }
        }
        .navigationTitle("Workspaces")
        .refreshable { await session.loadOrganizations() }
        .toolbar {
            ToolbarItem(placement: .topBarTrailing) {
                Button("Refresh", systemImage: "arrow.clockwise") {
                    Task { await session.loadOrganizations() }
                }
            }
        }
    }
}
