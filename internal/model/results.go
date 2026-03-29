package model

type AntiPattern struct {
	Type     AntiPatternType
	File     string
	Line     int
	Method   string
	Class    string
	Severity Tier
}

type AssertionAnalysis struct {
	TotalAssertions        int
	BehavioralMethods      int // methods with output assertions (AST or text-fallback) and no verify calls
	StructuralMethods      int // methods with verify calls
	UnclassifiedMethods    int // methods with neither assertions nor verify calls
	StrengthDistribution   map[Strength]int
	TargetDistribution     map[Target]int
	ZeroAssertionMethods   []MethodRef
	OutputInteractionRatio float64
	ComposeTestFiles       int // files with Compose test imports (assertions not fully analyzed)
	UnclassifiedStatements int // statements the tool couldn't classify as assertions, verify, or stub calls
}

type StructuralIndicator struct {
	Type   StructuralType
	File   string
	Line   int
	Method string
	Detail string
}

type TautologyFlag struct {
	File             string
	Line             int
	Method           string
	StubIdentifier   string
	AssertIdentifier string
	SUTSimple        *bool
}

type DoublesAnalysis struct {
	Doubles         []TestDouble
	MockDensity     float64
	SetupOnlyRatio  float64
	FakeCount       int
	MostMockedTypes []TypeFrequency
}

type TestDouble struct {
	TypeName         string
	VariableName     string
	Framework        Framework
	Usage            Usage
	Placement        Placement
	PlacementSignals []PlacementSignal
}

// ScopeInfo describes the test scope of a file plus data points within local tests.
type ScopeInfo struct {
	Scope         TestScope
	IsRobolectric bool // local test uses Robolectric runner
	IsComposeTest bool // local test uses ComposeTestRule
	IsRoomTest    bool // local test uses Room in-memory DB
}

// ScopeDistribution counts test files by scope category.
type ScopeDistribution struct {
	Local    int
	Device   int
	Snapshot int
}

type FileAnalysis struct {
	File         ParsedTestFile
	AntiPatterns []AntiPattern
	Assertions   AssertionAnalysis
	Structural   []StructuralIndicator
	Tautologies  []TautologyFlag
	Doubles         DoublesAnalysis
	Scope           ScopeInfo
	SetupComplexity SetupComplexity

	// StructuralCouplingScore is a 0–100 composite score indicating the degree
	// of structural coupling in this test file. Higher = more coupled to
	// implementation structure. Computed from static signals only (no git).
	// Weights are initial estimates — not yet calibrated.
	StructuralCouplingScore float64
}

type ScanResult struct {
	Path              string
	Language          Language
	TotalTestFiles    int
	TotalTestMethods  int
	UnparseableFiles  int
	FileResults       []FileAnalysis
	ScopeDistribution ScopeDistribution

	// Aggregate assertion strength distribution across all files (Tier 2 signal).
	AggregateStrength map[Strength]int
}

type SetupComplexity struct {
	SetupStatements int
	AssertionCount  int
	Ratio           float64
}
