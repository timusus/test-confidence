package model

import sitter "github.com/smacker/go-tree-sitter"

// ASTNode wraps a tree-sitter node with its source bytes.
type ASTNode struct {
	Node   *sitter.Node
	Source []byte
}

func (n ASTNode) Text() string {
	return string(n.Source[n.Node.StartByte():n.Node.EndByte()])
}

func (n ASTNode) Type() string {
	return n.Node.Type()
}

func (n ASTNode) Children() []ASTNode {
	count := int(n.Node.NamedChildCount())
	children := make([]ASTNode, count)
	for i := 0; i < count; i++ {
		children[i] = ASTNode{Node: n.Node.NamedChild(i), Source: n.Source}
	}
	return children
}

type Import struct {
	Path string
	Line int
}

type Annotation struct {
	Name string
	Args string
	Line int
}

type Property struct {
	Name           string
	TypeName       string
	Annotations    []Annotation
	HasInitializer bool
	InitText       string // Short prefix of the initializer expression (e.g., "mockk<", "mock(")
	Line           int
}

type MethodRef struct {
	File   string
	Class  string
	Method string
	Line   int
}

type ParsedTestFile struct {
	Path     string
	Language Language
	Source   []byte
	Imports  []Import
	Classes  []TestClass
	Tree     *sitter.Tree // retains tree-sitter tree so ASTNode pointers remain valid
}

// Close releases the underlying tree-sitter tree. Must be called when the
// ParsedTestFile is no longer needed.
func (p *ParsedTestFile) Close() {
	if p.Tree != nil {
		p.Tree.Close()
		p.Tree = nil
	}
}

type TestClass struct {
	Name        string
	Annotations []Annotation
	Methods     []TestMethod
	Properties  []Property
	SetupBlocks []ASTNode
}

type TestMethod struct {
	Name        string
	Annotations []Annotation
	Statements  []ASTNode
	LineStart   int
	LineEnd     int
}

type ParsedProductionFile struct {
	Path     string
	Language Language
	Source   []byte
	Methods  []ProductionMethod
	Tree     *sitter.Tree
}

// Close releases the underlying tree-sitter tree. Must be called when the
// ParsedProductionFile is no longer needed.
func (p *ParsedProductionFile) Close() {
	if p.Tree != nil {
		p.Tree.Close()
		p.Tree = nil
	}
}

type ProductionMethod struct {
	Name         string
	Body         []ASTNode
	IsSingleExpr bool
	BranchCount  int
	CallCount    int
	LineStart    int
	LineEnd      int
}

type ProjectContext struct {
	Language        Language
	DIBoundaryTypes map[string]bool
	FakeTypes       map[string]bool
	TypePackages    map[string]string
	ThirdPartyPkgs  []string
}

type TypeFrequency struct {
	TypeName string
	Count    int
}

type PlacementSignal struct {
	Signal string    // e.g., "import_origin", "di_module", "package_path", "type_name", "stubbing_pattern"
	Value  string    // e.g., "retrofit2.ApiClient", "boundary"
	Result Placement
}
