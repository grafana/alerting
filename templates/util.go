package templates

import (
	"fmt"
	"sort"
	tmpltext "text/template"
	"text/template/parse"
)

// FindTopLevelTemplates returns the top-level definitions of the given template.
func FindTopLevelTemplates(tmpl *tmpltext.Template) ([]string, error) {
	// We need to find the names of all defined templates and subtract
	// the names of all executed templates to find the set of templates that
	// should be tested.
	definedTmpls := make([]*tmpltext.Template, 0)
	for _, def := range tmpl.Templates() {
		// Check defined templates for an empty outer wrapper template.
		// This can happen if the template filename does not match the name of any template definition.
		// Remove if it exists.
		if def.Name() == tmpl.ParseName && parse.IsEmptyTree(def.Root) {
			continue
		}
		definedTmpls = append(definedTmpls, def)
	}

	if len(definedTmpls) == 0 {
		return nil, nil
	}

	executedTmpls := make(map[string]struct{})
	for _, t := range definedTmpls {
		err := checkTmpl(t, executedTmpls)
		if err != nil {
			return nil, err
		}
	}

	results := make([]string, 0, len(definedTmpls))
	for _, t := range definedTmpls {
		name := t.Name()
		if _, ok := executedTmpls[name]; !ok {
			results = append(results, name)
		}
	}

	// Stable ordering.
	sort.Strings(results)
	return results, nil
}

func checkTmpl(tmpl *tmpltext.Template, executedTmpls map[string]struct{}) error {
	tr := tmpl.Tree
	if tr == nil {
		return fmt.Errorf("template %s has nil parse tree", tmpl.Name())
	}
	checkListNode(tr.Root, executedTmpls)

	return nil
}

func checkBranchNode(node *parse.BranchNode, executedTmpls map[string]struct{}) {
	if node.List != nil {
		checkListNode(node.List, executedTmpls)
	}
	if node.ElseList != nil {
		checkListNode(node.ElseList, executedTmpls)
	}
}

func checkListNode(node *parse.ListNode, executedTmpls map[string]struct{}) {
	for _, n := range node.Nodes {
		checkNode(n, executedTmpls)
	}
}

func checkNode(node parse.Node, executedTmpls map[string]struct{}) {
	switch node.Type() {
	case parse.NodeAction:
		// check if we need to do something here
	case parse.NodeCommand:
		// check if we need to do something here
	case parse.NodeIf:
		n := node.(*parse.IfNode)
		checkBranchNode(&n.BranchNode, executedTmpls)
	case parse.NodeList:
		n := node.(*parse.ListNode)
		checkListNode(n, executedTmpls)
	case parse.NodeRange:
		n := node.(*parse.RangeNode)
		checkBranchNode(&n.BranchNode, executedTmpls)
	case parse.NodeTemplate:
		n := node.(*parse.TemplateNode)
		executedTmpls[n.Name] = struct{}{}
	case parse.NodeWith:
		n := node.(*parse.WithNode)
		checkBranchNode(&n.BranchNode, executedTmpls)
	}
}
