import SwiftUI

struct RootView: View {
    @EnvironmentObject private var session: SessionModel
    @State private var identifier = ""
    @State private var password = ""
    @State private var showRegistration = false
    @State private var showOrganizationCreation = false
    @State private var newOrganizationName = ""
    @State private var newOrganizationCountry = "GB"
    @State private var newOrganizationKind = "company"
    @State private var isCreatingOrganization = false

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
                Button("Create an account") { showRegistration = true }
                Text("Phone verification is required for registration.")
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
        .sheet(isPresented: $showRegistration) {
            RegistrationView()
                .environmentObject(session)
        }
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
                Button {
                    showOrganizationCreation = true
                    newOrganizationName = ""
                } label: {
                    Label("Create organization", systemImage: "plus.circle.fill")
                }
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
        .sheet(isPresented: $showOrganizationCreation) {
            NavigationStack {
                Form {
                    TextField("Organization name", text: $newOrganizationName)
                        .textInputAutocapitalization(.words)
                    TextField("Country code (ISO 3166-1, e.g. GB)", text: $newOrganizationCountry)
                        .textInputAutocapitalization(.characters)
                        .autocorrectionDisabled()
                    Picker("Type", selection: $newOrganizationKind) {
                        Text("Company").tag("company")
                        Text("Individual").tag("individual")
                    }
                    if let message = session.message {
                        Text(message).foregroundStyle(.red)
                    }
                }
                .navigationTitle("New organization")
                .navigationBarTitleDisplayMode(.inline)
                .toolbar {
                    ToolbarItem(placement: .cancellationAction) {
                        Button("Cancel") { showOrganizationCreation = false }
                    }
                    ToolbarItem(placement: .confirmationAction) {
                        Button("Create") {
                            isCreatingOrganization = true
                            Task {
                                let created = await session.createOrganization(name: newOrganizationName, country: newOrganizationCountry.uppercased(), kind: newOrganizationKind)
                                isCreatingOrganization = false
                                if created { showOrganizationCreation = false }
                            }
                        }
                        .disabled(newOrganizationName.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty || newOrganizationCountry.count != 2 || isCreatingOrganization)
                    }
                }
            }
        }
        .toolbar {
            ToolbarItem(placement: .topBarTrailing) {
                Button("Refresh", systemImage: "arrow.clockwise") {
                    Task { await session.loadOrganizations() }
                }
            }
        }
    }
}
