import XCTest
@testable import MyApp

// RxTest is not XCTestCase — tests the indirect inheritance pattern
class ObservableRelayBindTest: RxTest {}

extension ObservableRelayBindTest {
    func testBindToPublishRelay() {
        let relay = PublishRelay<Int>()
        XCTAssertNotNil(relay)
    }

    func testBindToBehaviorRelay() {
        let relay = BehaviorRelay<Int>(value: 0)
        XCTAssertEqual(relay.value, 0)
    }
}

// Extension on a class defined elsewhere — no class declaration in this file
extension SomeOtherTest {
    func testFromExtensionOnly() {
        XCTAssertTrue(true)
    }
}

// A class with BaseTCATestCase — tests indirect inheritance
class ReducerTests: BaseTCATestCase {
    func testInitialState() {
        XCTAssertNotNil(store)
    }
}
