import XCTest
@testable import MyApp

class SleepPatternTests: XCTestCase {
    func testWithTaskSleep() async {
        try? await Task.sleep(nanoseconds: 1_000_000)
        XCTAssertTrue(true)
    }

    func testWithUsleep() {
        usleep(1000)
        XCTAssertTrue(true)
    }

    func testWithSleep() {
        sleep(1)
        XCTAssertTrue(true)
    }

    func testWithDispatchAfter() {
        DispatchQueue.main.asyncAfter(deadline: .now() + 1.0) {
            self.doSomething()
        }
        XCTAssertTrue(true)
    }

    func testWithAsyncExpectation() {
        let expectation = expectation(description: "async")
        sut.fetchData { result in
            XCTAssertNotNil(result)
            expectation.fulfill()
        }
        waitForExpectations(timeout: 5)
    }
}
