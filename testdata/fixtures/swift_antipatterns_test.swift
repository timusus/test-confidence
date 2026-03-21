import XCTest
@testable import MyApp

class AntiPatternTests: XCTestCase {
    func testEmpty() {
    }

    func testIgnoredWithSkip() throws {
        throw XCTSkip("Not implemented yet")
    }

    func testWithSleep() {
        Thread.sleep(forTimeInterval: 1.0)
        XCTAssertTrue(true)
    }

    func testConditionalLogic() {
        if ProcessInfo.processInfo.environment["CI"] != nil {
            XCTAssertTrue(true)
        }
    }

    func testWithThrows() {
        XCTAssertThrowsError(try sut.dangerousOperation()) { error in
            XCTAssertEqual(error as? MyError, MyError.expected)
        }
    }

    func testWithNoThrow() {
        XCTAssertNoThrow(try sut.safeOperation())
    }

    func testAssertionRoulette() {
        XCTAssertEqual(result.a, 1)
        XCTAssertEqual(result.b, 2)
        XCTAssertEqual(result.c, 3)
        XCTAssertEqual(result.d, 4)
        XCTAssertEqual(result.e, 5)
        XCTAssertEqual(result.f, 6)
        XCTAssertEqual(result.g, 7)
        XCTAssertEqual(result.h, 8)
        XCTAssertEqual(result.i, 9)
        XCTAssertEqual(result.j, 10)
        XCTAssertEqual(result.k, 11)
    }
}
