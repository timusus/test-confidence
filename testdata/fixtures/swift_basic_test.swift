import XCTest
@testable import MyApp

class UserViewModelTests: XCTestCase {
    var sut: UserViewModel!
    var mockRepository: MockUserRepository!

    override func setUp() {
        super.setUp()
        mockRepository = MockUserRepository()
        sut = UserViewModel(repository: mockRepository)
    }

    override func tearDown() {
        sut = nil
        super.tearDown()
    }

    func testLoadUsers() {
        let users = sut.loadUsers()
        XCTAssertEqual(users.count, 3)
        XCTAssertTrue(users.first?.name == "Alice")
        XCTAssertNotNil(users.first)
    }

    func testDeleteUser() {
        let result = sut.deleteUser(id: "123")
        XCTAssertTrue(result)
    }

    func testAsyncFetch() async throws {
        let result = try await sut.fetchData()
        XCTAssertEqual(result.count, 5)
    }
}
