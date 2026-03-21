package parse

import (
	"os"
	"strings"
	"testing"

	"github.com/timusus/test-confidence/internal/model"
)

func TestParseKotlin(t *testing.T) {
	src, err := os.ReadFile("../../testdata/fixtures/hello.kt")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	tree, err := ParseKotlin(src)
	if err != nil {
		t.Fatalf("failed to parse Kotlin: %v", err)
	}
	defer tree.Close()

	root := tree.RootNode()

	if root.Type() != "source_file" {
		t.Errorf("expected root node type 'source_file', got %q", root.Type())
	}

	if root.NamedChildCount() == 0 {
		t.Error("expected at least one named child node")
	}
}

func TestParseKotlinTestFile(t *testing.T) {
	src, err := os.ReadFile("../../testdata/fixtures/kotlin_basic_test.kt")
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := ParseTestFile("../../testdata/fixtures/kotlin_basic_test.kt", src, model.Kotlin)
	if err != nil {
		t.Fatal(err)
	}
	defer parsed.Close()

	// Basic structure
	if len(parsed.Classes) != 1 {
		t.Fatalf("expected 1 class, got %d", len(parsed.Classes))
	}

	cls := parsed.Classes[0]
	if cls.Name != "UserViewModelTest" {
		t.Errorf("expected class name 'UserViewModelTest', got %q", cls.Name)
	}

	// 3 test methods (setUp is not a @Test method)
	if len(cls.Methods) != 3 {
		t.Errorf("expected 3 test methods, got %d", len(cls.Methods))
	}

	// Imports include mockk
	hasMockK := false
	for _, imp := range parsed.Imports {
		if strings.Contains(imp.Path, "io.mockk") {
			hasMockK = true
			break
		}
	}
	if !hasMockK {
		t.Error("expected mockk import")
	}

	// @MockK property detected with type name
	hasMockProp := false
	for _, prop := range cls.Properties {
		if prop.TypeName == "UserRepository" {
			hasMockProp = true
			hasMockAnnotation := false
			for _, ann := range prop.Annotations {
				if ann.Name == "MockK" {
					hasMockAnnotation = true
				}
			}
			if !hasMockAnnotation {
				t.Error("expected @MockK annotation on UserRepository property")
			}
		}
	}
	if !hasMockProp {
		t.Error("expected property with type UserRepository")
	}

	// Setup blocks from @Before
	if len(cls.SetupBlocks) == 0 {
		t.Error("expected setup blocks from @Before method")
	}

	// @Ignore annotation detected (search by name, not index)
	foundIgnored := false
	for _, m := range cls.Methods {
		for _, ann := range m.Annotations {
			if ann.Name == "Ignore" {
				foundIgnored = true
			}
		}
	}
	if !foundIgnored {
		t.Error("expected @Ignore annotation on a method")
	}

	// Methods have statements
	for _, m := range cls.Methods {
		if m.Name == "should load users" {
			if len(m.Statements) == 0 {
				t.Error("expected statements in 'should load users' method")
			}
		}
	}
}
