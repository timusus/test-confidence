package analyze

import (
	"sort"
	"strings"

	"github.com/timusus/test-confidence/internal/model"
	"github.com/timusus/test-confidence/internal/parse"
)

// AnalyzeDoubles detects test doubles in a parsed test file, classifies their
// usage (SetupOnly, Verification, Both, FakeUsage), and computes aggregate
// metrics like mock density and setup-only ratio.
func AnalyzeDoubles(file *model.ParsedTestFile) model.DoublesAnalysis {
	var doubles []model.TestDouble

	for _, cls := range file.Classes {
		// 1. Detect doubles from class properties
		doubles = append(doubles, detectPropertyDoubles(cls)...)

		// 2. Detect inline mockk<Type>() in method bodies
		for _, method := range cls.Methods {
			doubles = append(doubles, detectInlineMocks(method)...)
		}

		// 3. Classify usage by scanning ALL methods
		classifyUsage(doubles, cls)
	}

	// Compute aggregate metrics
	analysis := model.DoublesAnalysis{
		Doubles:         doubles,
		MockDensity:     computeMockDensity(file),
		MostMockedTypes: computeMostMockedTypes(doubles),
	}

	// Count fakes and compute setup-only ratio
	var nonFakeCount, setupOnlyCount int
	for _, d := range doubles {
		if d.Usage == model.FakeUsage {
			analysis.FakeCount++
		} else {
			nonFakeCount++
			if d.Usage == model.SetupOnly {
				setupOnlyCount++
			}
		}
	}
	if nonFakeCount > 0 {
		analysis.SetupOnlyRatio = float64(setupOnlyCount) / float64(nonFakeCount)
	}

	return analysis
}

// detectPropertyDoubles extracts test doubles from class-level property declarations.
// Detects @MockK/@Mock/@SpyK annotated properties, Mock-prefixed manual mocks,
// and hand-written fakes (Fake*/Test*/Stub* types or fake*/test*/stub* variables).
func detectPropertyDoubles(cls model.TestClass) []model.TestDouble {
	var doubles []model.TestDouble

	for _, prop := range cls.Properties {
		// Check for mock annotations
		framework, isMock := detectMockAnnotation(prop.Annotations)
		if isMock {
			doubles = append(doubles, model.TestDouble{
				TypeName:     prop.TypeName,
				VariableName: prop.Name,
				Framework:    framework,
				Placement:    model.UnknownPlacement,
			})
			continue
		}

		// Check for Swift manual mock types (e.g., MockUserRepository)
		if strings.HasPrefix(prop.TypeName, "Mock") || strings.HasPrefix(prop.Name, "mock") {
			if !isFake(prop.Name, prop.TypeName) {
				doubles = append(doubles, model.TestDouble{
					TypeName:     prop.TypeName,
					VariableName: prop.Name,
					Framework:    model.FrameworkManualMock,
					Placement:    model.UnknownPlacement,
				})
				continue
			}
		}

		// Check for inline mockk/mock/spyk in the initializer (e.g., `val activity: Activity = mockk { }`)
		if prop.HasInitializer && prop.InitText != "" {
			if strings.Contains(prop.InitText, "mockk") {
				doubles = append(doubles, model.TestDouble{
					TypeName:     prop.TypeName,
					VariableName: prop.Name,
					Framework:    model.FrameworkMockK,
					Placement:    model.UnknownPlacement,
				})
				continue
			}
			if strings.Contains(prop.InitText, "spyk") {
				doubles = append(doubles, model.TestDouble{
					TypeName:     prop.TypeName,
					VariableName: prop.Name,
					Framework:    model.FrameworkMockK,
					Placement:    model.UnknownPlacement,
				})
				continue
			}
			if strings.Contains(prop.InitText, "mock(") || strings.Contains(prop.InitText, "mock<") {
				doubles = append(doubles, model.TestDouble{
					TypeName:     prop.TypeName,
					VariableName: prop.Name,
					Framework:    model.FrameworkMockito,
					Placement:    model.UnknownPlacement,
				})
				continue
			}
		}

		// Check for Fake type or variable name
		if isFake(prop.Name, prop.TypeName) {
			typeName := prop.TypeName
			if typeName == "" {
				// Try to extract from initializer — for `val fakeDb = FakeDatabase()`
				// the property has no declared type, but we can look at the
				// variable name to infer it's a fake.
				typeName = prop.Name
			}
			doubles = append(doubles, model.TestDouble{
				TypeName:     typeName,
				VariableName: prop.Name,
				Framework:    model.FrameworkFake,
				Usage:        model.FakeUsage,
				Placement:    model.UnknownPlacement,
			})
		}
	}

	return doubles
}

// detectMockAnnotation checks if any annotation indicates a mock framework.
func detectMockAnnotation(annotations []model.Annotation) (model.Framework, bool) {
	for _, ann := range annotations {
		switch ann.Name {
		case "MockK", "SpyK":
			return model.FrameworkMockK, true
		case "Mock", "Spy", "InjectMocks":
			return model.FrameworkMockito, true
		}
	}
	return 0, false
}

// isFake returns true if the variable name or type name suggests a hand-written
// fake. Matches Fake, Test, and Stub prefixes in type names, and fake*, test*,
// stub* variable name patterns (case-insensitive first letter).
func isFake(varName, typeName string) bool {
	return isFakeVarName(varName) || isFakeTypeName(typeName)
}

// isFakeTypeName checks if a type name starts with Fake, Test, or Stub prefix.
func isFakeTypeName(typeName string) bool {
	return strings.HasPrefix(typeName, "Fake") ||
		strings.HasPrefix(typeName, "Test") ||
		strings.HasPrefix(typeName, "Stub")
}

// isFakeVarName checks if a variable name suggests a fake (fake*, test*, stub*
// followed by an uppercase letter, indicating a type suffix like testRepository).
func isFakeVarName(varName string) bool {
	for _, prefix := range []string{"fake", "test", "stub"} {
		if strings.HasPrefix(varName, prefix) && len(varName) > len(prefix) {
			// Require next char to be uppercase to avoid matching
			// "testing", "stubborn", etc.
			next := varName[len(prefix)]
			if next >= 'A' && next <= 'Z' {
				return true
			}
		}
	}
	return false
}

// detectInlineMocks finds inline mockk<Type>() or mock(Type::class) calls
// within method statements.
func detectInlineMocks(method model.TestMethod) []model.TestDouble {
	var doubles []model.TestDouble

	for _, stmt := range method.Statements {
		if stmt.Type() != "property_declaration" {
			continue
		}

		text := stmt.Text()
		varName := extractVarName(stmt)
		if varName == "" {
			continue
		}

		// mockk<Type>()
		if strings.Contains(text, "mockk<") {
			typeName := extractGenericType(text, "mockk<")
			doubles = append(doubles, model.TestDouble{
				TypeName:     typeName,
				VariableName: varName,
				Framework:    model.FrameworkMockK,
				Placement:    model.UnknownPlacement,
			})
			continue
		}

		// mock(Type::class) — Mockito style
		if strings.Contains(text, "mock(") {
			typeName := extractMockitoType(text)
			doubles = append(doubles, model.TestDouble{
				TypeName:     typeName,
				VariableName: varName,
				Framework:    model.FrameworkMockito,
				Placement:    model.UnknownPlacement,
			})
			continue
		}

		// Hand-written fakes instantiated inline: val testRepo = TestUserRepository()
		typeName := extractInlineCallType(stmt)
		if isFake(varName, typeName) {
			if typeName == "" {
				typeName = varName
			}
			doubles = append(doubles, model.TestDouble{
				TypeName:     typeName,
				VariableName: varName,
				Framework:    model.FrameworkFake,
				Usage:        model.FakeUsage,
				Placement:    model.UnknownPlacement,
			})
		}
	}

	return doubles
}

// extractInlineCallType extracts the type name from a call_expression child
// of a property_declaration AST node (e.g., `val x = FakeRepo()` -> "FakeRepo").
func extractInlineCallType(stmt model.ASTNode) string {
	for _, child := range stmt.Children() {
		if child.Type() == "call_expression" {
			for _, cc := range child.Children() {
				if cc.Type() == "simple_identifier" {
					return cc.Text()
				}
			}
		}
	}
	return ""
}

// extractVarName extracts the variable name from a property_declaration AST node.
// Handles both Kotlin (variable_declaration > simple_identifier) and
// Swift (pattern > simple_identifier) AST structures.
func extractVarName(node model.ASTNode) string {
	for _, child := range node.Children() {
		if child.Type() == "variable_declaration" {
			for _, vc := range child.Children() {
				if vc.Type() == "simple_identifier" {
					return vc.Text()
				}
			}
		}
		// Swift: property_declaration > pattern > simple_identifier
		if child.Type() == "pattern" {
			for _, pc := range child.Children() {
				if pc.Type() == "simple_identifier" {
					return pc.Text()
				}
			}
		}
	}
	return ""
}

// extractGenericType extracts the type from a pattern like "mockk<Logger>()".
func extractGenericType(text, prefix string) string {
	idx := strings.Index(text, prefix)
	if idx < 0 {
		return ""
	}
	rest := text[idx+len(prefix):]
	end := strings.Index(rest, ">")
	if end < 0 {
		return ""
	}
	return rest[:end]
}

// extractMockitoType extracts the type from "mock(Type::class)".
func extractMockitoType(text string) string {
	idx := strings.Index(text, "mock(")
	if idx < 0 {
		return ""
	}
	rest := text[idx+5:]
	end := strings.Index(rest, "::class")
	if end < 0 {
		return ""
	}
	return rest[:end]
}

// classifyUsage scans all test methods in a class to determine each double's
// usage pattern (SetupOnly, Verification, Both). Fakes are left as FakeUsage.
func classifyUsage(doubles []model.TestDouble, cls model.TestClass) {
	type usageInfo struct {
		hasStub   bool
		hasVerify bool
	}

	usage := map[string]*usageInfo{}
	for i := range doubles {
		if doubles[i].Usage != model.FakeUsage {
			usage[doubles[i].VariableName] = &usageInfo{}
		}
	}

	// Scan all methods
	for _, method := range cls.Methods {
		for _, stmt := range method.Statements {
			if parse.IsStubCall(stmt) {
				receiver, _ := parse.ExtractCallTarget(stmt)
				if info, ok := usage[receiver]; ok {
					info.hasStub = true
				}
			}
			if parse.IsVerifyCall(stmt) {
				receiver, _ := parse.ExtractCallTarget(stmt)
				if info, ok := usage[receiver]; ok {
					info.hasVerify = true
				}
			}
		}
	}

	// Apply classification
	for i := range doubles {
		if doubles[i].Usage == model.FakeUsage {
			continue
		}
		info := usage[doubles[i].VariableName]
		if info == nil {
			continue
		}
		switch {
		case info.hasStub && info.hasVerify:
			doubles[i].Usage = model.Both
		case info.hasStub:
			doubles[i].Usage = model.SetupOnly
		case info.hasVerify:
			doubles[i].Usage = model.Verification
		}
	}
}

// computeMockDensity calculates the ratio of mock-related constructs to total
// statements across all methods plus mock properties.
func computeMockDensity(file *model.ParsedTestFile) float64 {
	var mockCount, totalCount int

	for _, cls := range file.Classes {
		// Count mock-annotated properties
		for _, prop := range cls.Properties {
			_, isMock := detectMockAnnotation(prop.Annotations)
			if isMock {
				mockCount++
			}
		}
		totalCount += len(cls.Properties)

		for _, method := range cls.Methods {
			for _, stmt := range method.Statements {
				totalCount++
				if parse.IsStubCall(stmt) || parse.IsVerifyCall(stmt) {
					mockCount++
				} else if stmt.Type() == "property_declaration" {
					text := stmt.Text()
					if strings.Contains(text, "mockk<") || strings.Contains(text, "mock(") {
						mockCount++
					}
				}
			}
		}
	}

	if totalCount == 0 {
		return 0
	}
	return float64(mockCount) / float64(totalCount)
}

// computeMostMockedTypes aggregates doubles by TypeName and returns them
// sorted by count descending.
func computeMostMockedTypes(doubles []model.TestDouble) []model.TypeFrequency {
	counts := map[string]int{}
	for _, d := range doubles {
		if d.TypeName != "" && d.Usage != model.FakeUsage {
			counts[d.TypeName]++
		}
	}

	result := make([]model.TypeFrequency, 0, len(counts))
	for typeName, count := range counts {
		result = append(result, model.TypeFrequency{TypeName: typeName, Count: count})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})

	return result
}
