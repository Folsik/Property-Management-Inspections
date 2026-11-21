import Foundation
import Combine

@MainActor
final class WorkspaceModel: ObservableObject {
    let organization: Organization
    private let api: APIClient

    @Published private(set) var properties: [PropertyRecord] = []
    @Published private(set) var inspections: [InspectionRecord] = []
    @Published private(set) var items: [InspectionItemRecord] = []
    @Published private(set) var media: [InspectionMedia] = []
    @Published private(set) var isLoading = false
    @Published private(set) var message: String?
    @Published private(set) var hasLoaded = false

    init(organization: Organization, api: APIClient) {
        self.organization = organization
        self.api = api
    }

    func load() async {
        guard !isLoading else { return }
        isLoading = true
        defer { isLoading = false }
        do {
            async let properties = api.properties(in: organization.id)
            async let inspections = api.inspections(in: organization.id)
            let (newProperties, newInspections) = try await (properties, inspections)
            self.properties = newProperties
            self.inspections = newInspections
            message = nil
            hasLoaded = true
        } catch {
            message = error.localizedDescription
        }
    }

    func loadItems(inspectionID: String) async {
        do {
            items = try await api.items(in: organization.id, inspectionID: inspectionID)
            message = nil
        } catch {
            message = error.localizedDescription
        }
    }

    func createProperty(name: String) async -> Bool {
        do {
            try await api.createProperty(in: organization.id, named: name)
            await load()
            return true
        } catch {
            message = error.localizedDescription
            return false
        }
    }

    func createInspection(propertyID: String, type: String) async -> Bool {
        do {
            try await api.createInspection(in: organization.id, propertyID: propertyID, type: type)
            await load()
            return true
        } catch {
            message = error.localizedDescription
            return false
        }
    }

    func addItem(inspectionID: String, title: String, notes: String, severity: String, clientID: UUID) async -> Bool {
        do {
            try await api.createItem(in: organization.id, inspectionID: inspectionID, title: title, notes: notes,
                                     severity: severity, clientID: clientID)
            await loadItems(inspectionID: inspectionID)
            await load()
            return true
        } catch {
            message = error.localizedDescription
            return false
        }
    }

    func setItemStatus(inspectionID: String, item: InspectionItemRecord, status: String) async -> Bool {
        do {
            try await api.updateItem(in: organization.id, inspectionID: inspectionID, itemID: item.id,
                                     version: item.version, status: status)
            await loadItems(inspectionID: inspectionID)
            await load()
            return true
        } catch {
            // Preserve the original server version. A 409 must never silently overwrite.
            message = error.localizedDescription + " Refresh and review any changes made by another inspector."
            await loadItems(inspectionID: inspectionID)
            return false
        }
    }

    func loadMedia(inspectionID: String) async {
        do {
            media = try await api.media(in: organization.id, inspectionID: inspectionID)
            message = nil
        } catch {
            message = error.localizedDescription
        }
    }

    func uploadMedia(inspectionID: String, file: URL, mime: String) async -> Bool {
        do {
            try await api.uploadMedia(in: organization.id, inspectionID: inspectionID, file: file, mime: mime)
            await loadMedia(inspectionID: inspectionID)
            return true
        } catch {
            message = error.localizedDescription
            return false
        }
    }

    func transcribe(inspectionID: String, mediaID: String) async -> Bool {
        do {
            try await api.requestTranscription(in: organization.id, inspectionID: inspectionID, mediaID: mediaID)
            message = nil
            return true
        } catch {
            message = error.localizedDescription
            return false
        }
    }

    func report(inspectionID: String, mediaID: String) async throws -> Data {
        try await api.generatedReport(in: organization.id, inspectionID: inspectionID, mediaID: mediaID)
    }

    func clearMessage() { message = nil }
}
