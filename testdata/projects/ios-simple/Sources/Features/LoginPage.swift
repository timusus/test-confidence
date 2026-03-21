import SwiftUI

struct LoginPage: View {
    var body: some View {
        VStack {
            TextField("Email", text: .constant(""))
            SecureField("Password", text: .constant(""))
        }
    }
}
