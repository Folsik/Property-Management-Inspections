import SwiftUI

struct WorkspaceView: View {
    @StateObject private var model: WorkspaceModel
    @EnvironmentObject private var session: SessionModel
    let onSwitchWorkspace: () -> Void

    init(organization: Organization, api: APIClient, onSwitchWorkspace: @escaping () -> Void) {
        self.onSwitchWorkspace = onSwitchWorkspace
        _model = StateObject(wrappedValue: WorkspaceModel(organization: organization, api: api))
    }

    var body: some View {
        TabView {
            NavigationStack {
                overview
                    .navigationTitle("Overview")
            }
            .tabItem { Label("Home", systemImage: "house") }

            NavigationStack {
                PropertyListView(model: model)
            }
            .tabItem { Label("Properties", systemImage: "building.2") }

            NavigationStack {
                InspectionListView(model: model)
            }
            .tabItem { Label("Inspections", systemImage: "checklist") }

            NavigationStack {
                account
                    .navigationTitle("Account")
            }
            .tabItem { Label("Account", systemImage: "person.crop.circle") }
        }
        .tint(.blue)
        .task { if !model.hasLoaded { await model.load() } }
    }

    private var overview: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 20) {
                VStack(alignment: .leading, spacing: 6) {
                    Text(model.organization.name)
                        .font(.title2.bold())
                    Text("Property inspections workspace")
                        .foregroundStyle(.secondary)
                }

                HStack(spacing: 12) {
                    metric("Properties", count: model.properties.count, icon: "building.2")
                    metric("Inspections", count: model.inspections.count, icon: "checklist")
                }
                HStack(spacing: 12) {
                    metric("Open issues", count: model.inspections.reduce(0) { $0 + $1.openItems }, icon: "exclamationmark.circle")
                    metric("Completed", count: model.inspections.filter { $0.status == "completed" }.count, icon: "checkmark.seal")
                }

                Text("Recent inspections").font(.headline)
                if model.inspections.isEmpty {
                    ContentUnavailableView("No inspections yet", systemImage: "clipboard",
                        description: Text("Create an inspection in the Inspections tab."))
                } else {
                    ForEach(model.inspections.prefix(5)) { inspection in
                        NavigationLink {
                            InspectionDetailView(model: model, inspection: inspection)
                        } label: {
                            InspectionRow(inspection: inspection)
                        }
                        .buttonStyle(.plain)
                    }
                }
                if let message = model.message {
                    Text(message).font(.footnote).foregroundStyle(.red)
                }
            }
            .padding()
        }
        .background(Color(uiColor: .systemGroupedBackground))
        .refreshable { await model.load() }
        .toolbar {
            ToolbarItem(placement: .topBarTrailing) {
                Button("Refresh", systemImage: "arrow.clockwise") { Task { await model.load() } }
            }
        }
    }

    private func metric(_ title: String, count: Int, icon: String) -> some View {
        VStack(alignment: .leading, spacing: 12) {
            Image(systemName: icon).foregroundStyle(.blue)
            Text(count.formatted()).font(.largeTitle.bold())
            Text(title).font(.caption).foregroundStyle(.secondary)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding()
        .background(.background, in: RoundedRectangle(cornerRadius: 16))
        .accessibilityElement(children: .combine)
    }

    private var account: some View {
        Form {
            Section("Workspace") {
                LabeledContent("Organization", value: model.organization.name)
                LabeledContent("Role", value: model.organization.role.capitalized)
                LabeledContent("Country", value: model.organization.country)
            }
            Section("Your account") {
                LabeledContent("Name", value: session.profile?.fullName ?? "")
                LabeledContent("Phone", value: session.profile?.phone ?? "")
            }
            Section {
                Button("Sign out", role: .destructive) {
                    Task { await session.signOut() }
                }
                .disabled(session.isBusy)
            }
            if let message = session.message {
                Section { Text(message).foregroundStyle(.red) }
            }
        }
    }
}

struct PropertyListView: View {
    @ObservedObject var model: WorkspaceModel
    @State private var showCreate = false
    @State private var name = ""
    @State private var saving = false

    var body: some View {
        List {
            if let message = model.message {
                Section { Text(message).foregroundStyle(.red) }
            }
            if model.properties.isEmpty && !model.isLoading {
                ContentUnavailableView("No properties", systemImage: "building.2.crop.circle",
                    description: Text("Add your first property to create inspections."))
            }
            ForEach(model.properties) { property in
                VStack(alignment: .leading, spacing: 4) {
                    Text(property.name).font(.headline)
                    Text("Updated \(property.updatedAt.formatted(date: .abbreviated, time: .omitted))")
                        .font(.caption).foregroundStyle(.secondary)
                }
                .padding(.vertical, 5)
            }
        }
        .navigationTitle("Properties")
        .refreshable { await model.load() }
        .toolbar {
            if ["owner", "manager"].contains(model.organization.role) {
                ToolbarItem(placement: .topBarTrailing) {
                    Button("Add property", systemImage: "plus") { showCreate = true }
                }
            }
        }
        .sheet(isPresented: $showCreate) {
            NavigationStack {
                Form {
                    TextField("Property name", text: $name)
                        .textInputAutocapitalization(.words)
                }
                .navigationTitle("New property")
                .navigationBarTitleDisplayMode(.inline)
                .toolbar {
                    ToolbarItem(placement: .cancellationAction) {
                        Button("Cancel") { showCreate = false }
                    }
                    ToolbarItem(placement: .confirmationAction) {
                        Button("Save") {
                            saving = true
                            Task {
                                let saved = await model.createProperty(name: name)
                                saving = false
                                if saved { showCreate = false; name = "" }
                            }
                        }
                        .disabled(name.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty || saving)
                    }
                }
                if saving { ProgressView() }
                if let message = model.message {
                    Text(message).foregroundStyle(.red).font(.footnote).padding()
                }
            }
        }
    }
}

struct InspectionRow: View {
    let inspection: InspectionRecord

    var body: some View {
        HStack(spacing: 12) {
            Image(systemName: inspection.status == "completed" ? "checkmark.circle.fill" : "clipboard")
                .foregroundStyle(inspection.status == "completed" ? .green : .blue)
                .font(.title2)
            VStack(alignment: .leading, spacing: 4) {
                Text(inspection.propertyName).font(.headline)
                Text(inspection.displayType + " · " + inspection.status.replacingOccurrences(of: "_", with: " ").capitalized)
                    .font(.subheadline).foregroundStyle(.secondary)
            }
            Spacer()
            VStack(alignment: .trailing) {
                Text(inspection.openItems.formatted()).font(.headline)
                Text("open").font(.caption).foregroundStyle(.secondary)
            }
        }
        .padding(.vertical, 4)
    }
}

struct InspectionListView: View {
    @ObservedObject var model: WorkspaceModel
    @State private var showCreate = false
    @State private var propertyID = ""
    @State private var kind = "move_in"
    @State private var saving = false

    var body: some View {
        List {
            if let message = model.message { Text(message).foregroundStyle(.red) }
            if model.inspections.isEmpty && !model.isLoading {
                ContentUnavailableView("No inspections", systemImage: "checklist",
                    description: Text("Create an inspection for one of your properties."))
            }
            ForEach(model.inspections) { inspection in
                NavigationLink {
                    InspectionDetailView(model: model, inspection: inspection)
                } label: {
                    InspectionRow(inspection: inspection)
                }
            }
        }
        .navigationTitle("Inspections")
        .refreshable { await model.load() }
        .toolbar {
            ToolbarItem(placement: .topBarTrailing) {
                Button("New inspection", systemImage: "plus") {
                    propertyID = model.properties.first?.id ?? ""
                    showCreate = true
                }
                .disabled(model.properties.isEmpty)
            }
        }
        .sheet(isPresented: $showCreate) {
            NavigationStack {
                Form {
                    Picker("Property", selection: $propertyID) {
                        ForEach(model.properties) { property in
                            Text(property.name).tag(property.id)
                        }
                    }
                    Picker("Type", selection: $kind) {
                        Text("Move in").tag("move_in")
                        Text("Move out").tag("move_out")
                        Text("Turnover").tag("turnover")
                        Text("Maintenance").tag("maintenance")
                    }
                    if let message = model.message {
                        Text(message).foregroundStyle(.red)
                    }
                }
                .navigationTitle("New inspection")
                .navigationBarTitleDisplayMode(.inline)
                .toolbar {
                    ToolbarItem(placement: .cancellationAction) {
                        Button("Cancel") { showCreate = false }
                    }
                    ToolbarItem(placement: .confirmationAction) {
                        Button("Create") {
                            saving = true
                            Task {
                                let created = await model.createInspection(propertyID: propertyID, type: kind)
                                saving = false
                                if created { showCreate = false }
                            }
                        }
                        .disabled(propertyID.isEmpty || saving)
                    }
                }
            }
        }
    }
}

struct InspectionDetailView: View {
    @ObservedObject var model: WorkspaceModel
    let inspection: InspectionRecord
    @State private var showCreate = false
    @State private var title = ""
    @State private var notes = ""
    @State private var severity = "normal"
    @State private var pendingClientID = UUID()
    @State private var saving = false

    var body: some View {
        List {
            Section("Inspection") {
                LabeledContent("Property", value: inspection.propertyName)
                LabeledContent("Type", value: inspection.displayType)
                LabeledContent("Status", value: inspection.status.capitalized)
            }
            Section("Media") {
                NavigationLink {
                    InspectionMediaView(model: model, inspection: inspection)
                } label: {
                    Label("Photos, voice notes & reports", systemImage: "photo.on.rectangle.angled")
                }
            }
            Section("Inspection items") {
                if model.items.isEmpty { Text("No items yet").foregroundStyle(.secondary) }
                ForEach(model.items) { item in
                    VStack(alignment: .leading, spacing: 8) {
                        Text(item.title).font(.headline)
                        if !item.notes.isEmpty {
                            Text(item.notes).font(.subheadline).foregroundStyle(.secondary)
                        }
                        HStack {
                            Text(item.severity.capitalized)
                                .font(.caption).foregroundStyle(.secondary)
                            Spacer()
                            Menu {
                                ForEach(["open", "in_progress", "resolved"], id: \.self) { state in
                                    Button(state.replacingOccurrences(of: "_", with: " ").capitalized) {
                                        Task { _ = await model.setItemStatus(inspectionID: inspection.id, item: item, status: state) }
                                    }
                                }
                            } label: {
                                Label(item.status.replacingOccurrences(of: "_", with: " ").capitalized,
                                      systemImage: "chevron.up.chevron.down")
                                    .font(.caption)
                            }
                            .disabled(inspection.status != "draft" && inspection.status != "in_progress")
                        }
                    }
                    .padding(.vertical, 5)
                }
            }
            if let message = model.message {
                Section { Text(message).foregroundStyle(.red) }
            }
        }
        .navigationTitle("Inspection")
        .navigationBarTitleDisplayMode(.inline)
        .task { await model.loadItems(inspectionID: inspection.id) }
        .refreshable { await model.loadItems(inspectionID: inspection.id) }
        .toolbar {
            if inspection.status == "draft" || inspection.status == "in_progress" {
                ToolbarItem(placement: .topBarTrailing) {
                    Button("Add item", systemImage: "plus") {
                        pendingClientID = UUID()
                        showCreate = true
                    }
                }
            }
        }
        .sheet(isPresented: $showCreate) {
            NavigationStack {
                Form {
                    TextField("Issue title", text: $title)
                    TextField("Notes", text: $notes, axis: .vertical)
                        .lineLimit(3...8)
                    Picker("Severity", selection: $severity) {
                        ForEach(["low", "normal", "high", "critical"], id: \.self) { value in
                            Text(value.capitalized).tag(value)
                        }
                    }
                    if let message = model.message {
                        Text(message).foregroundStyle(.red)
                    }
                }
                .navigationTitle("Add item")
                .navigationBarTitleDisplayMode(.inline)
                .toolbar {
                    ToolbarItem(placement: .cancellationAction) {
                        Button("Cancel") { showCreate = false }
                    }
                    ToolbarItem(placement: .confirmationAction) {
                        Button("Save") {
                            saving = true
                            Task {
                                let result = await model.addItem(inspectionID: inspection.id, title: title,
                                                                 notes: notes, severity: severity, clientID: pendingClientID)
                                saving = false
                                if result { showCreate = false; title = ""; notes = ""; pendingClientID = UUID() }
                            }
                        }
                        .disabled(title.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty || saving)
                    }
                }
            }
        }
    }
}
