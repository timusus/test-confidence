import XCTest
@testable import MyApp

class AsyncTests: XCTestCase {
    var sut: DataLoader!

    override func setUpWithError() throws {
        sut = DataLoader()
    }

    override func tearDownWithError() throws {
        sut = nil
    }

    func testAsyncDataLoad() async throws {
        let data = try await sut.loadData()
        XCTAssertFalse(data.isEmpty)
        XCTAssertEqual(data.count, 10)
    }

    func testAsyncWithExpectation() {
        let expectation = expectation(description: "Data loaded")
        sut.loadData { result in
            XCTAssertNotNil(result)
            expectation.fulfill()
        }
        waitForExpectations(timeout: 5.0)
    }

    func testConcurrentLoad() async throws {
        async let first = sut.loadData()
        async let second = sut.loadData()
        let results = try await [first, second]
        XCTAssertEqual(results.count, 2)
    }
}
