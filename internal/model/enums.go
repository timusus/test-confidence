package model

type Language int

const (
	Kotlin Language = iota
	Swift
)

func (l Language) String() string {
	switch l {
	case Kotlin:
		return "Kotlin"
	case Swift:
		return "Swift"
	default:
		return "Unknown"
	}
}

type Platform int

const (
	Android Platform = iota
	IOS
)

func (p Platform) String() string {
	switch p {
	case Android:
		return "Android"
	case IOS:
		return "iOS"
	default:
		return "Unknown"
	}
}

type Tier int

const (
	Tier1 Tier = 1 // >90% accuracy — facts
	Tier2 Tier = 2 // 70-90% — indicators
	Tier3 Tier = 3 // 50-70% — observations with caveats
)

func (t Tier) String() string {
	switch t {
	case Tier1:
		return "Tier1"
	case Tier2:
		return "Tier2"
	case Tier3:
		return "Tier3"
	default:
		return "Unknown"
	}
}

type Strength int

const (
	Strong Strength = iota // assertEquals, isEqualTo, shouldBe, contains, hasSize
	Medium                 // assertTrue, assertFalse, assertIsDisplayed
	Weak                   // assertNotNull, assertNull
)

func (s Strength) String() string {
	switch s {
	case Strong:
		return "Strong"
	case Medium:
		return "Medium"
	case Weak:
		return "Weak"
	default:
		return "Unknown"
	}
}

type Target int

const (
	Output      Target = iota // assertions on SUT return values, state, UI
	Interaction               // verify calls — NOT counted in TotalAssertions
	Exception                 // assertThrows, shouldThrow
)

func (t Target) String() string {
	switch t {
	case Output:
		return "Output"
	case Interaction:
		return "Interaction"
	case Exception:
		return "Exception"
	default:
		return "Unknown"
	}
}

type Usage int

const (
	SetupOnly    Usage = iota // stubbed but never verified
	Verification              // verified but never stubbed
	Both                      // stubbed and verified
	FakeUsage                 // Fake* class instantiation
)

func (u Usage) String() string {
	switch u {
	case SetupOnly:
		return "SetupOnly"
	case Verification:
		return "Verification"
	case Both:
		return "Both"
	case FakeUsage:
		return "FakeUsage"
	default:
		return "Unknown"
	}
}

type Placement int

const (
	Boundary Placement = iota
	Internal
	UnknownPlacement
)

func (p Placement) String() string {
	switch p {
	case Boundary:
		return "Boundary"
	case Internal:
		return "Internal"
	case UnknownPlacement:
		return "UnknownPlacement"
	default:
		return "Unknown"
	}
}

type Framework int

const (
	FrameworkMockK Framework = iota
	FrameworkMockito
	FrameworkManualMock
	FrameworkFake
)

func (f Framework) String() string {
	switch f {
	case FrameworkMockK:
		return "MockK"
	case FrameworkMockito:
		return "Mockito"
	case FrameworkManualMock:
		return "ManualMock"
	case FrameworkFake:
		return "Fake"
	default:
		return "Unknown"
	}
}

type AntiPatternType int

const (
	ThreadSleep AntiPatternType = iota
	UnsafeDelay
	EmptyTest
	IgnoredTest
	ConditionalLogic
	ReflectionUsage
	GodTestClass
	AssertionRoulette
	RelaxedMockPolicy
)

func (a AntiPatternType) String() string {
	switch a {
	case ThreadSleep:
		return "ThreadSleep"
	case UnsafeDelay:
		return "UnsafeDelay"
	case EmptyTest:
		return "EmptyTest"
	case IgnoredTest:
		return "IgnoredTest"
	case ConditionalLogic:
		return "ConditionalLogic"
	case ReflectionUsage:
		return "ReflectionUsage"
	case GodTestClass:
		return "GodTestClass"
	case AssertionRoulette:
		return "AssertionRoulette"
	case RelaxedMockPolicy:
		return "RelaxedMockPolicy"
	default:
		return "Unknown"
	}
}

type StructuralType int

const (
	VerifyOrdering StructuralType = iota
	ArgumentCaptor
	StubAndVerify
	VerifyOnlyTest
	PropertyAccessVerify // verify(mock).property.called(N) — counting getter access
)

func (s StructuralType) String() string {
	switch s {
	case VerifyOrdering:
		return "VerifyOrdering"
	case ArgumentCaptor:
		return "ArgumentCaptor"
	case StubAndVerify:
		return "StubAndVerify"
	case VerifyOnlyTest:
		return "VerifyOnlyTest"
	case PropertyAccessVerify:
		return "PropertyAccessVerify"
	default:
		return "Unknown"
	}
}

type TestScope int

const (
	ScopeLocal    TestScope = iota // host-side tests (src/test/, no device required)
	ScopeDevice                    // on-device or emulator tests (androidTest, XCUITest)
	ScopeSnapshot                  // screenshot/snapshot tests (Paparazzi, Roborazzi, SnapshotTesting)
)

func (s TestScope) String() string {
	switch s {
	case ScopeLocal:
		return "Local"
	case ScopeDevice:
		return "Device"
	case ScopeSnapshot:
		return "Snapshot"
	default:
		return "Unknown"
	}
}
