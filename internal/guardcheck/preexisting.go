package guardcheck

import (
	"go/ast"
)

// Identity is what makes two conditionals the same guard across a move.
//
// The reviewer is scoped to a diff by LINE, which is right for an edit and
// wrong for a move. Splitting one file into eight presents every relocated line
// as added, so the reviewer re-analyses the old file's whole conditional
// surface and reports its long-standing holes as if the split had dug them.
// Measured on #210: 136 guards and 42 survivors against origin/main on
// 2026-09-11, where the diagnosis in #232 recorded 125 and 39 against an
// earlier main. That diagnosis also recorded 94.5% coverage on both sides of
// the split and the same 27 zero-count blocks before and after under different
// file names; those are its figures, not this package's.
//
// File and line cannot tell a move from a rewrite. Package, enclosing function
// and condition text can, as far as text goes — and the limits of "as far as
// text goes" are the point of this comment, because everything the reviewer
// stops failing on rests on them.
//
// What it distinguishes:
//
//   - the same condition in another file of the same package: one identity,
//     which is the move this exists for
//   - the same condition text in another function: two identities. `if err
//     != nil` appears hundreds of times here, and an identity of text alone
//     would let any of them certify any other
//   - the same method name on another receiver type: two identities, because
//     Func carries the receiver
//   - any change to the condition's text beyond whitespace: two identities,
//     so a rewritten guard is judged as new
//
// What it CANNOT distinguish, and must be read before trusting a PRE-EXISTING
// line:
//
//   - two conditions with identical text in one function. Nothing here can say
//     which of them is which, so PairSurvivors compares how many survived on
//     each side rather than pairing by name, and calls the excess introduced.
//     Where a function holds several identical guards, "pre-existing" means
//     "one of these was already unobserved", not "this one was".
//   - a condition whose text is unchanged but whose meaning is not, because a
//     name it reads now holds something else. `if ok {` after the call above it
//     was swapped is a new guard wearing an old face, and this calls it
//     pre-existing.
//   - a guard inside a func literal assigned at package level, which has no
//     enclosing declaration and so falls back to package and text alone.
//   - a condition that moved to another package: Pkg is part of the identity,
//     so that reads as introduced. That is the loud direction and the correct
//     one — the suite that would observe it is a different suite, and the
//     reviewer's own docs record a guard reported SURVIVED because the test
//     covering it lived one package away.
type Identity struct {
	Pkg  string
	Func string
	Cond string
}

// Identity is the guard's identity across a move.
func (g Guard) Identity() Identity {
	return Identity{Pkg: g.Pkg, Func: g.Func, Cond: g.Cond}
}

// CountByIdentity counts guards per identity.
//
// It bounds the base-tree search: the reviewer neutralises base candidates for
// an identity only until it has found as many survivors there as the change has
// here, because that is all the pairing can spend.
func CountByIdentity(gs []Guard) map[Identity]int {
	out := make(map[Identity]int, len(gs))
	for _, g := range gs {
		out[g.Identity()]++
	}
	return out
}

// PairSurvivors splits survivors into the ones the base tree already had and
// the ones the change introduced.
//
// baseSurvivors says, per identity, how many conditions with that identity also
// survived neutralisation in the base tree — measured there, not inferred from
// the fact that the text matches. Each survivor here consumes one, and a
// survivor that finds none left is introduced.
//
// The excess fails. Three identical guards where the base had two survivors
// means one is new, and exempting the whole identity because a member of it was
// old is how a new hole hides behind an old one. Which of the three is the new
// one is not knowable from text; that it exists is.
//
// The empty case is the failing one by construction: with nothing known about
// the base, every survivor is introduced. A reviewer that stops failing is
// worse than one that fails noisily, so the exemption is earned per guard or
// not at all.
func PairSurvivors(survivors []Guard, baseSurvivors map[Identity]int) (pre, introduced []Guard) {
	budget := make(map[Identity]int, len(baseSurvivors))
	for k, v := range baseSurvivors {
		budget[k] = v
	}
	for _, g := range survivors {
		k := g.Identity()
		if budget[k] > 0 {
			budget[k]--
			pre = append(pre, g)
			continue
		}
		introduced = append(introduced, g)
	}
	return pre, introduced
}

// funcName renders a declaration's name, receiver type included.
func funcName(d *ast.FuncDecl) string {
	if d.Recv == nil || len(d.Recv.List) == 0 {
		return d.Name.Name
	}
	return "(" + receiverType(d.Recv.List[0].Type) + ")." + d.Name.Name
}

// receiverType renders a receiver's type well enough to tell two apart.
//
// Not a full type printer. A receiver is a name, a pointer to one, or a generic
// instantiation of one; anything else returns "?" and costs a match, which is
// the loud direction.
func receiverType(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return "*" + receiverType(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		// `func (s *Store[T]) ...` — the parameter is not part of the name.
		return receiverType(t.X)
	case *ast.IndexListExpr:
		return receiverType(t.X)
	}
	return "?"
}
