package parse

import (
	"context"
	"fmt"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/kotlin"
)

// ParseKotlin parses Kotlin source code and returns the resulting tree-sitter tree.
func ParseKotlin(src []byte) (*sitter.Tree, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(kotlin.GetLanguage())

	tree, err := parser.ParseCtx(context.Background(), nil, src)
	if err != nil {
		return nil, fmt.Errorf("tree-sitter parse: %w", err)
	}

	return tree, nil
}
