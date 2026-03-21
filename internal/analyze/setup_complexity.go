package analyze

import (
	"github.com/timusus/test-confidence/internal/model"
	"github.com/timusus/test-confidence/internal/parse"
)

// mockAnnotations are annotation names that declare mock/spy properties.
var mockAnnotations = map[string]bool{
	"MockK":       true,
	"SpyK":        true,
	"Mock":        true,
	"Spy":         true,
	"InjectMocks": true,
}

// ruleAnnotations are annotation names that declare test rule properties.
var ruleAnnotations = map[string]bool{
	"Rule":         true,
	"ClassRule":    true,
	"JvmField":    true,
	"get:Rule":     true,
	"get:ClassRule": true,
}

// AnalyzeSetupComplexity counts setup statements (setup blocks, property
// initializations, stubs, mock annotations, rule declarations) and computes
// the setup-to-assertion ratio per file. This is a Tier 2 signal — accurate
// at extremes (ratio > 5:1 strongly indicates setup-heavy tests) but
// ambiguous at moderate ratios.
func AnalyzeSetupComplexity(file *model.ParsedTestFile, assertions model.AssertionAnalysis) model.SetupComplexity {
	setupCount := 0

	for _, cls := range file.Classes {
		// Count @MockK / @Mock / @SpyK annotated properties
		for _, prop := range cls.Properties {
			counted := false
			for _, ann := range prop.Annotations {
				if mockAnnotations[ann.Name] {
					setupCount++
					counted = true
					break
				}
			}
			if counted {
				continue
			}
			// Count @Rule declarations as setup infrastructure
			for _, ann := range prop.Annotations {
				if ruleAnnotations[ann.Name] {
					setupCount++
					counted = true
					break
				}
			}
			if counted {
				continue
			}
			// Count class-level property initializations as setup
			// e.g. val repo = FakeUserRepository(), let sut = makeSUT()
			if prop.HasInitializer {
				setupCount++
			}
		}

		// Count statements in @Before/setUp setup blocks
		for _, block := range cls.SetupBlocks {
			setupCount += countSetupBlockStatements(block)
		}

		// Count stub calls and inline mock declarations in test methods
		for _, method := range cls.Methods {
			for _, stmt := range method.Statements {
				if parse.IsStubCall(stmt) {
					setupCount++
					continue
				}
				// Inline mockk<Type>() or mock(Type::class) declarations
				if stmt.Type() == "property_declaration" {
					text := stmt.Text()
					if containsMockCreation(text) {
						setupCount++
					}
				}
			}
		}
	}

	assertionCount := assertions.TotalAssertions

	var ratio float64
	if assertionCount > 0 {
		ratio = float64(setupCount) / float64(assertionCount)
	}

	return model.SetupComplexity{
		SetupStatements: setupCount,
		AssertionCount:  assertionCount,
		Ratio:           ratio,
	}
}

// countSetupBlockStatements counts the number of statements in a setup block
// (function_body node from @Before/setUp methods). Each direct child statement
// counts as one setup statement.
func countSetupBlockStatements(block model.ASTNode) int {
	count := 0
	n := block.Node
	for i := 0; i < int(n.NamedChildCount()); i++ {
		child := n.NamedChild(i)
		if child.Type() == "statements" {
			// function_body > statements > individual statements
			count += int(child.NamedChildCount())
		} else {
			// Direct children that are statements (e.g. in Swift)
			count++
		}
	}
	return count
}

// containsMockCreation checks if a property declaration text contains an inline
// mock creation call (mockk<...>() or mock(...::class)).
func containsMockCreation(text string) bool {
	for i := 0; i < len(text)-5; i++ {
		if text[i:i+6] == "mockk<" {
			return true
		}
	}
	for i := 0; i < len(text)-4; i++ {
		if text[i:i+5] == "mock(" {
			return true
		}
	}
	return false
}
