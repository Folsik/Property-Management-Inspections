import SwiftUI

struct RegistrationView: View {
    @EnvironmentObject private var session: SessionModel
    @Environment(\.dismiss) private var dismiss
    @State private var phone = ""
    @State private var code = ""
    @State private var password = ""
    @State private var confirmation = ""
    @State private var step = 0
    @State private var isBusy = false
    @State private var error: String?
    @State private var developerCode: String?

    var body: some View {
        NavigationStack {
            Form {
                Section {
                    HStack {
                        ForEach(0..<3) { index in
                            Capsule()
                                .fill(index <= step ? Color.blue : Color.gray.opacity(0.2))
                                .frame(height: 5)
                        }
                    }
                    .padding(.vertical, 6)
                }
                if step == 0 {
                    Section("Phone verification") {
                        TextField("+44 7700 900123", text: $phone)
                            .keyboardType(.phonePad)
                            .textContentType(.telephoneNumber)
                        Text("We will send a six-digit verification code. Use your own phone number.")
                            .font(.footnote).foregroundStyle(.secondary)
                        Button("Send verification code") { Task { await requestCode() } }
                            .disabled(!validPhone || isBusy)
                    }
                } else if step == 1 {
                    Section("Verify phone") {
                        Text(phone).foregroundStyle(.secondary)
                        TextField("6-digit code", text: $code)
                            .keyboardType(.numberPad)
                            .textContentType(.oneTimeCode)
                        if let developerCode {
                            Text("Local development OTP: \(developerCode)")
                                .font(.footnote).foregroundStyle(.secondary)
                        }
                        Button("Verify and continue") { Task { await verify() } }
                            .disabled(code.count != 6 || isBusy)
                        Button("Change phone number") { step = 0; code = "" }
                    }
                } else {
                    Section("Create password") {
                        SecureField("Password", text: $password)
                            .textContentType(.newPassword)
                        SecureField("Confirm password", text: $confirmation)
                            .textContentType(.newPassword)
                        Text("Use a unique password with at least 15 characters. Common passwords are rejected.")
                            .font(.footnote).foregroundStyle(.secondary)
                        Button("Create account") { Task { await finish() } }
                            .disabled(password.count < 15 || confirmation != password || isBusy)
                    }
                }
                if let error {
                    Section { Text(error).foregroundStyle(.red) }
                }
                if isBusy {
                    Section { ProgressView("Processing") }
                }
            }
            .navigationTitle("Create account")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Close") { dismiss() }
                }
            }
        }
    }

    private var validPhone: Bool {
        let stripped = phone.replacingOccurrences(of: " ", with: "")
        return stripped.hasPrefix("+") && stripped.count >= 9 && stripped.count <= 16 &&
            stripped.dropFirst().allSatisfy(\.isNumber)
    }

    private func requestCode() async {
        isBusy = true
        error = nil
        defer { isBusy = false }
        phone = phone.replacingOccurrences(of: " ", with: "")
        do {
            developerCode = try await session.requestOTP(phone: phone)
            step = 1
        } catch {
            self.error = error.localizedDescription
        }
    }

    private func verify() async {
        isBusy = true
        error = nil
        defer { isBusy = false }
        do {
            try await session.verifyOTP(phone: phone, code: code)
            developerCode = nil
            step = 2
        } catch {
            self.error = error.localizedDescription
        }
    }

    private func finish() async {
        isBusy = true
        error = nil
        defer { isBusy = false }
        do {
            try await session.finishRegistration(password: password)
            password = ""
            confirmation = ""
            dismiss()
        } catch {
            self.error = error.localizedDescription
        }
    }
}
