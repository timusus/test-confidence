package ios

var StrongAssertions = []string{"XCTAssertEqual", "XCTAssertIdentical", "XCTAssertGreaterThan", "XCTAssertLessThan"}
var MediumAssertions = []string{"XCTAssertTrue", "XCTAssertFalse"}
var WeakAssertions = []string{"XCTAssertNil", "XCTAssertNotNil"}

var ExceptionAssertions = []string{"XCTAssertThrowsError", "XCTAssertNoThrow"}

// Manual mock patterns: properties with these suffixes indicate mock tracking.
var MockPropertySuffixes = []string{"Called", "CallCount", "LastArgument", "ReceivedArguments", "ReceivedInvocations"}

var ThirdPartyPkgs = []string{
	"CoreData", "Network", "StoreKit", "Alamofire", "Moya",
	"Firebase", "Realm", "URLSession",
}

var BoundaryTypeDefaults = []string{"*API", "*Client", "*DataSource", "*NetworkManager"}
var InternalTypeDefaults = []string{
	"*UseCase", "*ViewModel", "*Presenter", "*Interactor", "*Coordinator",
	"*Helper", "*Manager", "*Store", "*Handler", "*Router", "*Navigator", "*Service",
}

var BoundaryPackageDefaults = []string{"Data/", "Network/", "API/", "Persistence/", "Remote/", "Services/"}
var InternalPackageDefaults = []string{"Domain/", "UseCase/", "Feature/", "UI/", "Presentation/", "Scenes/"}
