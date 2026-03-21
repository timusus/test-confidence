import XCTest
@testable import MyApp

// A manual mock implementing the UserRepository protocol
class MockUserRepository: UserRepository {
    var fetchUsersCalled = false
    var fetchUsersCallCount = 0
    var saveUserCalled = false
    var saveUserLastArgument: User?
    var stubbedUsers: [User] = []

    func fetchUsers() -> [User] {
        fetchUsersCalled = true
        fetchUsersCallCount += 1
        return stubbedUsers
    }

    func saveUser(_ user: User) {
        saveUserCalled = true
        saveUserLastArgument = user
    }
}

class UserServiceTests: XCTestCase {
    var sut: UserService!
    var mockRepository: MockUserRepository!
    var fakeAnalytics: FakeAnalyticsTracker!

    override func setUp() {
        super.setUp()
        mockRepository = MockUserRepository()
        fakeAnalytics = FakeAnalyticsTracker()
        sut = UserService(repository: mockRepository, analytics: fakeAnalytics)
    }

    func testFetchUsersCallsRepository() {
        mockRepository.stubbedUsers = [User(name: "Alice")]
        let users = sut.fetchUsers()
        XCTAssertTrue(mockRepository.fetchUsersCalled)
        XCTAssertEqual(users.count, 1)
    }

    func testSaveUserPassesArgument() {
        let user = User(name: "Bob")
        sut.saveUser(user)
        XCTAssertTrue(mockRepository.saveUserCalled)
        XCTAssertEqual(mockRepository.saveUserLastArgument?.name, "Bob")
    }
}
