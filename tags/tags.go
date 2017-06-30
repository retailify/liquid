// Package tags defines the standard Liquid tags.
package tags

import (
	"io"

	"github.com/osteele/liquid/chunks"
	"github.com/osteele/liquid/expressions"
	"github.com/osteele/liquid/generics"
)

// DefineStandardTags defines the standard Liquid tags.
func DefineStandardTags() {
	// The parser only recognize the comment and raw tags if they've been defined,
	// but it ignores any syntax specified here.
	loopTags := []string{"break", "continue", "cycle"}
	chunks.DefineTag("break", breakTag)
	chunks.DefineTag("continue", continueTag)
	chunks.DefineStartTag("capture").Parser(captureTagParser)
	chunks.DefineStartTag("case").Branch("when").Parser(caseTagParser)
	chunks.DefineStartTag("comment")
	chunks.DefineStartTag("for").Governs(loopTags).Parser(loopTagParser)
	chunks.DefineStartTag("if").Branch("else").Branch("elsif").Parser(ifTagParser(true))
	chunks.DefineStartTag("raw")
	chunks.DefineStartTag("tablerow").Governs(loopTags)
	chunks.DefineStartTag("unless").SameSyntaxAs("if").Parser(ifTagParser(false))
}

func captureTagParser(node chunks.ASTControlTag) (func(io.Writer, chunks.RenderContext) error, error) {
	// TODO verify syntax
	varname := node.Parameters
	return func(w io.Writer, ctx chunks.RenderContext) error {
		s, err := ctx.InnerString()
		if err != nil {
			return err
		}
		ctx.Set(varname, s)
		return nil
	}, nil
}

func caseTagParser(node chunks.ASTControlTag) (func(io.Writer, chunks.RenderContext) error, error) {
	// TODO parse error on non-empty node.Body
	// TODO case can include an else
	expr, err := expressions.Parse(node.Parameters)
	if err != nil {
		return nil, err
	}
	type caseRec struct {
		expr expressions.Expression
		node *chunks.ASTControlTag
	}
	cases := []caseRec{}
	for _, branch := range node.Branches {
		bfn, err := expressions.Parse(branch.Parameters)
		if err != nil {
			return nil, err
		}
		cases = append(cases, caseRec{bfn, branch})
	}
	return func(w io.Writer, ctx chunks.RenderContext) error {
		value, err := ctx.Evaluate(expr)
		if err != nil {
			return err
		}
		for _, branch := range cases {
			b, err := ctx.Evaluate(branch.expr)
			if err != nil {
				return err
			}
			if generics.Equal(value, b) {
				return ctx.RenderBranch(w, branch.node)
			}
		}
		return nil
	}, nil
}

func ifTagParser(polarity bool) func(chunks.ASTControlTag) (func(io.Writer, chunks.RenderContext) error, error) {
	return func(node chunks.ASTControlTag) (func(io.Writer, chunks.RenderContext) error, error) {
		type branchRec struct {
			test expressions.Expression
			body *chunks.ASTControlTag
		}
		expr, err := expressions.Parse(node.Parameters)
		if err != nil {
			return nil, err
		}
		if !polarity {
			expr = expressions.Negate(expr)
		}
		branches := []branchRec{
			{expr, &node},
		}
		for _, c := range node.Branches {
			test := expressions.Constant(true)
			switch c.Name {
			case "else":
			// TODO parse error if this isn't the last branch
			case "elsif":
				t, err := expressions.Parse(c.Parameters)
				if err != nil {
					return nil, err
				}
				test = t
			default:
			}
			branches = append(branches, branchRec{test, c})
		}
		return func(w io.Writer, ctx chunks.RenderContext) error {
			for _, b := range branches {
				value, err := ctx.Evaluate(b.test)
				if err != nil {
					return err
				}
				if value != nil && value != false {
					return ctx.RenderBranch(w, b.body)
				}
			}
			return nil
		}, nil
	}
}
