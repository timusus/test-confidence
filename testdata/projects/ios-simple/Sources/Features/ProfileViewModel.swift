import Foundation

class ProfileViewModel: ObservableObject {
    @Published var name: String = ""
    @Published var email: String = ""
    
    func loadProfile() {
        // load from API
        name = "Alice"
        email = "alice@example.com"
    }
    
    func saveProfile() {
        // save to API
    }
}
