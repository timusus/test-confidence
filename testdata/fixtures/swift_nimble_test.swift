import XCTest
import Nimble

class UserServiceTests: XCTestCase {
    func testFetchUserReturnsCorrectName() {
        let service = UserService()
        let user = service.fetchUser()
        expect(user.name).to(equal("Alice"))
    }

    func testFetchUserReturnsNonNilAge() {
        let service = UserService()
        let user = service.fetchUser()
        expect(user.age).toNot(beNil())
    }

    func testUserListContainsAlice() {
        let service = UserService()
        let users = service.fetchUsers()
        expect(users).to(contain("Alice"))
    }

    func testFetchUserEventually() {
        let service = UserService()
        expect(service.asyncUser).toEventually(equal("Bob"))
    }

    func testUserIsActive() {
        let service = UserService()
        let user = service.fetchUser()
        expect(user.isActive).to(beTrue())
    }

    func testUserListIsNotEmpty() {
        let service = UserService()
        let users = service.fetchUsers()
        expect(users).notTo(beEmpty())
    }

    func testUserCount() {
        let service = UserService()
        let users = service.fetchUsers()
        expect(users).to(haveCount(3))
    }

    func testUserNameStartsWithA() {
        let service = UserService()
        let user = service.fetchUser()
        expect(user.name).to(beginWith("A"))
    }
}
