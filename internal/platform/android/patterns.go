package android

var StrongAssertions = []string{"assertEquals", "isEqualTo", "shouldBe", "contains", "hasSize", "isEmpty", "containsExactly"}
var MediumAssertions = []string{"assertTrue", "assertFalse", "assertIsDisplayed", "assertExists", "performClick"}
var WeakAssertions = []string{"assertNotNull", "assertNull", "isNotNull", "isNotEmpty"}

var VerifyFunctions = []string{"verify", "coVerify"}
var StubFunctions = []string{"every", "coEvery"}
var MockitoVerify = []string{"verify"}
var MockitoStub = []string{"whenever", "given"}

var ExceptionAssertions = []string{"assertThrows", "shouldThrow", "assertFailsWith"}

var MockAnnotations = []string{"MockK", "Mock", "SpyK"}
var MockCreators = []string{"mockk", "mock", "spyk"}

var ThirdPartyPkgs = []string{
	"retrofit2", "okhttp3", "androidx.room", "com.google.firebase",
	"kotlinx.coroutines", "com.squareup", "io.ktor", "javax.inject",
	"androidx.navigation", "androidx.media3", "androidx.datastore", "androidx.work",
}

var BoundaryTypeDefaults = []string{
	"*Api", "*Client", "*DataSource", "*Dao", "*RemoteSource",
	"*Manager", "*Saver", "*Recorder", "*Handle",
	// Android framework types that are always boundaries
	"NavController", "NavOptions", "NavHostController",
	"SavedStateHandle",
	"LifecycleOwner", "Lifecycle",
	"SnackbarHostState",
	"Context", "Application",
	"SharedPreferences",
	"ContentResolver",
	"PackageManager",
	"ConnectivityManager",
	"WorkManager",
	// AndroidX framework types
	"Player",       // androidx.media3
	"ExoPlayer",
	"MediaSession",
	"MediaController",
	"DataStore",
	"WorkRequest",
	"NotificationCompat",
}
var InternalTypeDefaults = []string{
	"*UseCase", "*ViewModel", "*Presenter", "*Interactor",
	"*State",      // UI state classes
	"*Navigation", // navigation coordinators
	"*Analytics",  // analytics trackers
}

// VerbPrefixes are prefixes that identify use-case/command-style classes.
// Classes matching these are auto-classified as internal without config.
// e.g., GetUser, UpdateProfile, RemoveItem, TogglePlayback
var VerbPrefixes = []string{
	"Get", "Set", "Update", "Delete", "Remove", "Add", "Create",
	"Toggle", "Follow", "Unfollow", "Map", "Convert", "Transform",
	"Fetch", "Load", "Save", "Search", "Filter", "Sort",
	"Subscribe", "Unsubscribe", "Observe",
}

var BoundaryPackageDefaults = []string{"data/", "network/", "api/", "persistence/", "remote/", "local/", "cache/"}
var InternalPackageDefaults = []string{"domain/", "usecase/", "feature/", "ui/", "presentation/"}
