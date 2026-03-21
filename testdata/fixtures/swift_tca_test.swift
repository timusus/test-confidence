import ComposableArchitecture
import Testing
@testable import MyApp

struct FeatureTests {
    @Test func sendAction() async {
        let store = TestStore(initialState: Feature.State()) {
            Feature()
        }
        await store.send(.buttonTapped) {
            $0.count = 1
        }
        await store.receive(.response(.success("ok"))) {
            $0.message = "ok"
        }
    }

    @Test func equality() {
        #expect(1 == 1)
    }

    @Test func comparison() {
        #expect(result.count > 0)
    }

    @Test func booleanCheck() {
        #expect(isValid)
    }

    @Test func throwsCheck() {
        #expect(throws: MyError.self) {
            try riskyOperation()
        }
    }

    @Test func requireUnwrap() throws {
        let value = try #require(optionalValue)
        #expect(value == 42)
    }
}
