package fbp

import (
	"fmt"
	"math"
	"sort"
	"strconv"
)

const end_symbol rune = 4

/* The rule types inferred from the grammar are below. */
type pegRule uint8

const (
	ruleUnknown pegRule = iota
	rulestart
	ruleline
	ruleLineTerminator
	rulecomment
	ruleconnection
	rulebridge
	ruleleftlet
	ruleiip
	rulerightlet
	rulenode
	rulecomponent
	rulecompMeta
	ruleport
	ruleportWithIndex
	ruleanychar
	ruleiipchar
	rule_
	rule__
	rulePegText
	ruleAction0
	ruleAction1
	ruleAction2
	ruleAction3
	ruleAction4
	ruleAction5
	ruleAction6
	ruleAction7
	ruleAction8
	ruleAction9
	ruleAction10
	ruleAction11
	ruleAction12
	ruleAction13
	ruleAction14

	rulePre_
	rule_In_
	rule_Suf
)

var rul3s = [...]string{
	"Unknown",
	"start",
	"line",
	"LineTerminator",
	"comment",
	"connection",
	"bridge",
	"leftlet",
	"iip",
	"rightlet",
	"node",
	"component",
	"compMeta",
	"port",
	"portWithIndex",
	"anychar",
	"iipchar",
	"_",
	"__",
	"PegText",
	"Action0",
	"Action1",
	"Action2",
	"Action3",
	"Action4",
	"Action5",
	"Action6",
	"Action7",
	"Action8",
	"Action9",
	"Action10",
	"Action11",
	"Action12",
	"Action13",
	"Action14",

	"Pre_",
	"_In_",
	"_Suf",
}

type tokenTree interface {
	Print()
	PrintSyntax()
	PrintSyntaxTree(buffer string)
	Add(rule pegRule, begin, end, next, depth int)
	Expand(index int) tokenTree
	Tokens() <-chan token32
	AST() *node32
	Error() []token32
	trim(length int)
}

type node32 struct {
	token32
	up, next *node32
}

func (node *node32) print(depth int, buffer string) {
	for node != nil {
		for c := 0; c < depth; c++ {
			fmt.Printf(" ")
		}
		fmt.Printf("\x1B[34m%v\x1B[m %v\n", rul3s[node.pegRule], strconv.Quote(buffer[node.begin:node.end]))
		if node.up != nil {
			node.up.print(depth+1, buffer)
		}
		node = node.next
	}
}

func (ast *node32) Print(buffer string) {
	ast.print(0, buffer)
}

type element struct {
	node *node32
	down *element
}

/* ${@} bit structure for abstract syntax tree */
type token16 struct {
	pegRule
	begin, end, next int16
}

func (t *token16) isZero() bool {
	return t.pegRule == ruleUnknown && t.begin == 0 && t.end == 0 && t.next == 0
}

func (t *token16) isParentOf(u token16) bool {
	return t.begin <= u.begin && t.end >= u.end && t.next > u.next
}

func (t *token16) getToken32() token32 {
	return token32{pegRule: t.pegRule, begin: int32(t.begin), end: int32(t.end), next: int32(t.next)}
}

func (t *token16) String() string {
	return fmt.Sprintf("\x1B[34m%v\x1B[m %v %v %v", rul3s[t.pegRule], t.begin, t.end, t.next)
}

type tokens16 struct {
	tree    []token16
	ordered [][]token16
}

func (t *tokens16) trim(length int) {
	t.tree = t.tree[0:length]
}

func (t *tokens16) Print() {
	for _, token := range t.tree {
		fmt.Println(token.String())
	}
}

func (t *tokens16) Order() [][]token16 {
	if t.ordered != nil {
		return t.ordered
	}

	depths := make([]int16, 1, math.MaxInt16)
	for i, token := range t.tree {
		if token.pegRule == ruleUnknown {
			t.tree = t.tree[:i]
			break
		}
		depth := int(token.next)
		if length := len(depths); depth >= length {
			depths = depths[:depth+1]
		}
		depths[depth]++
	}
	depths = append(depths, 0)

	ordered, pool := make([][]token16, len(depths)), make([]token16, len(t.tree)+len(depths))
	for i, depth := range depths {
		depth++
		ordered[i], pool, depths[i] = pool[:depth], pool[depth:], 0
	}

	for i, token := range t.tree {
		depth := token.next
		token.next = int16(i)
		ordered[depth][depths[depth]] = token
		depths[depth]++
	}
	t.ordered = ordered
	return ordered
}

type state16 struct {
	token16
	depths []int16
	leaf   bool
}

func (t *tokens16) AST() *node32 {
	tokens := t.Tokens()
	stack := &element{node: &node32{token32: <-tokens}}
	for token := range tokens {
		if token.begin == token.end {
			continue
		}
		node := &node32{token32: token}
		for stack != nil && stack.node.begin >= token.begin && stack.node.end <= token.end {
			stack.node.next = node.up
			node.up = stack.node
			stack = stack.down
		}
		stack = &element{node: node, down: stack}
	}
	return stack.node
}

func (t *tokens16) PreOrder() (<-chan state16, [][]token16) {
	s, ordered := make(chan state16, 6), t.Order()
	go func() {
		var states [8]state16
		for i, _ := range states {
			states[i].depths = make([]int16, len(ordered))
		}
		depths, state, depth := make([]int16, len(ordered)), 0, 1
		write := func(t token16, leaf bool) {
			S := states[state]
			state, S.pegRule, S.begin, S.end, S.next, S.leaf = (state+1)%8, t.pegRule, t.begin, t.end, int16(depth), leaf
			copy(S.depths, depths)
			s <- S
		}

		states[state].token16 = ordered[0][0]
		depths[0]++
		state++
		a, b := ordered[depth-1][depths[depth-1]-1], ordered[depth][depths[depth]]
	depthFirstSearch:
		for {
			for {
				if i := depths[depth]; i > 0 {
					if c, j := ordered[depth][i-1], depths[depth-1]; a.isParentOf(c) &&
						(j < 2 || !ordered[depth-1][j-2].isParentOf(c)) {
						if c.end != b.begin {
							write(token16{pegRule: rule_In_, begin: c.end, end: b.begin}, true)
						}
						break
					}
				}

				if a.begin < b.begin {
					write(token16{pegRule: rulePre_, begin: a.begin, end: b.begin}, true)
				}
				break
			}

			next := depth + 1
			if c := ordered[next][depths[next]]; c.pegRule != ruleUnknown && b.isParentOf(c) {
				write(b, false)
				depths[depth]++
				depth, a, b = next, b, c
				continue
			}

			write(b, true)
			depths[depth]++
			c, parent := ordered[depth][depths[depth]], true
			for {
				if c.pegRule != ruleUnknown && a.isParentOf(c) {
					b = c
					continue depthFirstSearch
				} else if parent && b.end != a.end {
					write(token16{pegRule: rule_Suf, begin: b.end, end: a.end}, true)
				}

				depth--
				if depth > 0 {
					a, b, c = ordered[depth-1][depths[depth-1]-1], a, ordered[depth][depths[depth]]
					parent = a.isParentOf(b)
					continue
				}

				break depthFirstSearch
			}
		}

		close(s)
	}()
	return s, ordered
}

func (t *tokens16) PrintSyntax() {
	tokens, ordered := t.PreOrder()
	max := -1
	for token := range tokens {
		if !token.leaf {
			fmt.Printf("%v", token.begin)
			for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
				fmt.Printf(" \x1B[36m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
			}
			fmt.Printf(" \x1B[36m%v\x1B[m\n", rul3s[token.pegRule])
		} else if token.begin == token.end {
			fmt.Printf("%v", token.begin)
			for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
				fmt.Printf(" \x1B[31m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
			}
			fmt.Printf(" \x1B[31m%v\x1B[m\n", rul3s[token.pegRule])
		} else {
			for c, end := token.begin, token.end; c < end; c++ {
				if i := int(c); max+1 < i {
					for j := max; j < i; j++ {
						fmt.Printf("skip %v %v\n", j, token.String())
					}
					max = i
				} else if i := int(c); i <= max {
					for j := i; j <= max; j++ {
						fmt.Printf("dupe %v %v\n", j, token.String())
					}
				} else {
					max = int(c)
				}
				fmt.Printf("%v", c)
				for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
					fmt.Printf(" \x1B[34m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
				}
				fmt.Printf(" \x1B[34m%v\x1B[m\n", rul3s[token.pegRule])
			}
			fmt.Printf("\n")
		}
	}
}

func (t *tokens16) PrintSyntaxTree(buffer string) {
	tokens, _ := t.PreOrder()
	for token := range tokens {
		for c := 0; c < int(token.next); c++ {
			fmt.Printf(" ")
		}
		fmt.Printf("\x1B[34m%v\x1B[m %v\n", rul3s[token.pegRule], strconv.Quote(buffer[token.begin:token.end]))
	}
}

func (t *tokens16) Add(rule pegRule, begin, end, depth, index int) {
	t.tree[index] = token16{pegRule: rule, begin: int16(begin), end: int16(end), next: int16(depth)}
}

func (t *tokens16) Tokens() <-chan token32 {
	s := make(chan token32, 16)
	go func() {
		for _, v := range t.tree {
			s <- v.getToken32()
		}
		close(s)
	}()
	return s
}

func (t *tokens16) Error() []token32 {
	ordered := t.Order()
	length := len(ordered)
	tokens, length := make([]token32, length), length-1
	for i, _ := range tokens {
		o := ordered[length-i]
		if len(o) > 1 {
			tokens[i] = o[len(o)-2].getToken32()
		}
	}
	return tokens
}

/* ${@} bit structure for abstract syntax tree */
type token32 struct {
	pegRule
	begin, end, next int32
}

func (t *token32) isZero() bool {
	return t.pegRule == ruleUnknown && t.begin == 0 && t.end == 0 && t.next == 0
}

func (t *token32) isParentOf(u token32) bool {
	return t.begin <= u.begin && t.end >= u.end && t.next > u.next
}

func (t *token32) getToken32() token32 {
	return token32{pegRule: t.pegRule, begin: int32(t.begin), end: int32(t.end), next: int32(t.next)}
}

func (t *token32) String() string {
	return fmt.Sprintf("\x1B[34m%v\x1B[m %v %v %v", rul3s[t.pegRule], t.begin, t.end, t.next)
}

type tokens32 struct {
	tree    []token32
	ordered [][]token32
}

func (t *tokens32) trim(length int) {
	t.tree = t.tree[0:length]
}

func (t *tokens32) Print() {
	for _, token := range t.tree {
		fmt.Println(token.String())
	}
}

func (t *tokens32) Order() [][]token32 {
	if t.ordered != nil {
		return t.ordered
	}

	depths := make([]int32, 1, math.MaxInt16)
	for i, token := range t.tree {
		if token.pegRule == ruleUnknown {
			t.tree = t.tree[:i]
			break
		}
		depth := int(token.next)
		if length := len(depths); depth >= length {
			depths = depths[:depth+1]
		}
		depths[depth]++
	}
	depths = append(depths, 0)

	ordered, pool := make([][]token32, len(depths)), make([]token32, len(t.tree)+len(depths))
	for i, depth := range depths {
		depth++
		ordered[i], pool, depths[i] = pool[:depth], pool[depth:], 0
	}

	for i, token := range t.tree {
		depth := token.next
		token.next = int32(i)
		ordered[depth][depths[depth]] = token
		depths[depth]++
	}
	t.ordered = ordered
	return ordered
}

type state32 struct {
	token32
	depths []int32
	leaf   bool
}

func (t *tokens32) AST() *node32 {
	tokens := t.Tokens()
	stack := &element{node: &node32{token32: <-tokens}}
	for token := range tokens {
		if token.begin == token.end {
			continue
		}
		node := &node32{token32: token}
		for stack != nil && stack.node.begin >= token.begin && stack.node.end <= token.end {
			stack.node.next = node.up
			node.up = stack.node
			stack = stack.down
		}
		stack = &element{node: node, down: stack}
	}
	return stack.node
}

func (t *tokens32) PreOrder() (<-chan state32, [][]token32) {
	s, ordered := make(chan state32, 6), t.Order()
	go func() {
		var states [8]state32
		for i, _ := range states {
			states[i].depths = make([]int32, len(ordered))
		}
		depths, state, depth := make([]int32, len(ordered)), 0, 1
		write := func(t token32, leaf bool) {
			S := states[state]
			state, S.pegRule, S.begin, S.end, S.next, S.leaf = (state+1)%8, t.pegRule, t.begin, t.end, int32(depth), leaf
			copy(S.depths, depths)
			s <- S
		}

		states[state].token32 = ordered[0][0]
		depths[0]++
		state++
		a, b := ordered[depth-1][depths[depth-1]-1], ordered[depth][depths[depth]]
	depthFirstSearch:
		for {
			for {
				if i := depths[depth]; i > 0 {
					if c, j := ordered[depth][i-1], depths[depth-1]; a.isParentOf(c) &&
						(j < 2 || !ordered[depth-1][j-2].isParentOf(c)) {
						if c.end != b.begin {
							write(token32{pegRule: rule_In_, begin: c.end, end: b.begin}, true)
						}
						break
					}
				}

				if a.begin < b.begin {
					write(token32{pegRule: rulePre_, begin: a.begin, end: b.begin}, true)
				}
				break
			}

			next := depth + 1
			if c := ordered[next][depths[next]]; c.pegRule != ruleUnknown && b.isParentOf(c) {
				write(b, false)
				depths[depth]++
				depth, a, b = next, b, c
				continue
			}

			write(b, true)
			depths[depth]++
			c, parent := ordered[depth][depths[depth]], true
			for {
				if c.pegRule != ruleUnknown && a.isParentOf(c) {
					b = c
					continue depthFirstSearch
				} else if parent && b.end != a.end {
					write(token32{pegRule: rule_Suf, begin: b.end, end: a.end}, true)
				}

				depth--
				if depth > 0 {
					a, b, c = ordered[depth-1][depths[depth-1]-1], a, ordered[depth][depths[depth]]
					parent = a.isParentOf(b)
					continue
				}

				break depthFirstSearch
			}
		}

		close(s)
	}()
	return s, ordered
}

func (t *tokens32) PrintSyntax() {
	tokens, ordered := t.PreOrder()
	max := -1
	for token := range tokens {
		if !token.leaf {
			fmt.Printf("%v", token.begin)
			for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
				fmt.Printf(" \x1B[36m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
			}
			fmt.Printf(" \x1B[36m%v\x1B[m\n", rul3s[token.pegRule])
		} else if token.begin == token.end {
			fmt.Printf("%v", token.begin)
			for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
				fmt.Printf(" \x1B[31m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
			}
			fmt.Printf(" \x1B[31m%v\x1B[m\n", rul3s[token.pegRule])
		} else {
			for c, end := token.begin, token.end; c < end; c++ {
				if i := int(c); max+1 < i {
					for j := max; j < i; j++ {
						fmt.Printf("skip %v %v\n", j, token.String())
					}
					max = i
				} else if i := int(c); i <= max {
					for j := i; j <= max; j++ {
						fmt.Printf("dupe %v %v\n", j, token.String())
					}
				} else {
					max = int(c)
				}
				fmt.Printf("%v", c)
				for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
					fmt.Printf(" \x1B[34m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
				}
				fmt.Printf(" \x1B[34m%v\x1B[m\n", rul3s[token.pegRule])
			}
			fmt.Printf("\n")
		}
	}
}

func (t *tokens32) PrintSyntaxTree(buffer string) {
	tokens, _ := t.PreOrder()
	for token := range tokens {
		for c := 0; c < int(token.next); c++ {
			fmt.Printf(" ")
		}
		fmt.Printf("\x1B[34m%v\x1B[m %v\n", rul3s[token.pegRule], strconv.Quote(buffer[token.begin:token.end]))
	}
}

func (t *tokens32) Add(rule pegRule, begin, end, depth, index int) {
	t.tree[index] = token32{pegRule: rule, begin: int32(begin), end: int32(end), next: int32(depth)}
}

func (t *tokens32) Tokens() <-chan token32 {
	s := make(chan token32, 16)
	go func() {
		for _, v := range t.tree {
			s <- v.getToken32()
		}
		close(s)
	}()
	return s
}

func (t *tokens32) Error() []token32 {
	ordered := t.Order()
	length := len(ordered)
	tokens, length := make([]token32, length), length-1
	for i, _ := range tokens {
		o := ordered[length-i]
		if len(o) > 1 {
			tokens[i] = o[len(o)-2].getToken32()
		}
	}
	return tokens
}

func (t *tokens16) Expand(index int) tokenTree {
	tree := t.tree
	if index >= len(tree) {
		expanded := make([]token32, 2*len(tree))
		for i, v := range tree {
			expanded[i] = v.getToken32()
		}
		return &tokens32{tree: expanded}
	}
	return nil
}

func (t *tokens32) Expand(index int) tokenTree {
	tree := t.tree
	if index >= len(tree) {
		expanded := make([]token32, 2*len(tree))
		copy(expanded, tree)
		t.tree = expanded
	}
	return nil
}

type Fbp struct {
	BaseFbp

	Buffer string
	buffer []rune
	rules  [35]func() bool
	Parse  func(rule ...int) error
	Reset  func()
	tokenTree
}

type textPosition struct {
	line, symbol int
}

type textPositionMap map[int]textPosition

func translatePositions(buffer string, positions []int) textPositionMap {
	length, translations, j, line, symbol := len(positions), make(textPositionMap, len(positions)), 0, 1, 0
	sort.Ints(positions)

search:
	for i, c := range buffer[0:] {
		if c == '\n' {
			line, symbol = line+1, 0
		} else {
			symbol++
		}
		if i == positions[j] {
			translations[positions[j]] = textPosition{line, symbol}
			for j++; j < length; j++ {
				if i != positions[j] {
					continue search
				}
			}
			break search
		}
	}

	return translations
}

type parseError struct {
	p *Fbp
}

func (e *parseError) Error() string {
	tokens, error := e.p.tokenTree.Error(), "\n"
	positions, p := make([]int, 2*len(tokens)), 0
	for _, token := range tokens {
		positions[p], p = int(token.begin), p+1
		positions[p], p = int(token.end), p+1
	}
	translations := translatePositions(e.p.Buffer, positions)
	for _, token := range tokens {
		begin, end := int(token.begin), int(token.end)
		error += fmt.Sprintf("parse error near \x1B[34m%v\x1B[m (line %v symbol %v - line %v symbol %v):\n%v\n",
			rul3s[token.pegRule],
			translations[begin].line, translations[begin].symbol,
			translations[end].line, translations[end].symbol,
			/*strconv.Quote(*/ e.p.Buffer[begin:end] /*)*/)
	}

	return error
}

func (p *Fbp) PrintSyntaxTree() {
	p.tokenTree.PrintSyntaxTree(p.Buffer)
}

func (p *Fbp) Highlighter() {
	p.tokenTree.PrintSyntax()
}

func (p *Fbp) Execute() {
	buffer, begin, end := p.Buffer, 0, 0
	for token := range p.tokenTree.Tokens() {
		switch token.pegRule {
		case rulePegText:
			begin, end = int(token.begin), int(token.end)
		case ruleAction0:
			p.createInport(buffer[begin:end])
		case ruleAction1:
			p.createOutport(buffer[begin:end])
		case ruleAction2:
			p.inPort = p.port
			p.inPortIndex = p.index
		case ruleAction3:
			p.outPort = p.port
			p.outPortIndex = p.index
		case ruleAction4:
			p.createMiddlet()
		case ruleAction5:
			p.createLeftlet()
		case ruleAction6:
			p.createRightlet()
		case ruleAction7:
			p.iip = buffer[begin:end]
		case ruleAction8:
			p.nodeProcessName = buffer[begin:end]
		case ruleAction9:
			p.createNode()
		case ruleAction10:
			p.nodeComponentName = buffer[begin:end]
		case ruleAction11:
			p.nodeMeta = buffer[begin:end]
		case ruleAction12:
			p.port = buffer[begin:end]
		case ruleAction13:
			p.port = buffer[begin:end]
		case ruleAction14:
			p.index = buffer[begin:end]

		}
	}
}

func (p *Fbp) Init() {
	p.buffer = []rune(p.Buffer)
	if len(p.buffer) == 0 || p.buffer[len(p.buffer)-1] != end_symbol {
		p.buffer = append(p.buffer, end_symbol)
	}

	var tree tokenTree = &tokens16{tree: make([]token16, math.MaxInt16)}
	position, depth, tokenIndex, buffer, rules := 0, 0, 0, p.buffer, p.rules

	p.Parse = func(rule ...int) error {
		r := 1
		if len(rule) > 0 {
			r = rule[0]
		}
		matches := p.rules[r]()
		p.tokenTree = tree
		if matches {
			p.tokenTree.trim(tokenIndex)
			return nil
		}
		return &parseError{p}
	}

	p.Reset = func() {
		position, tokenIndex, depth = 0, 0, 0
	}

	add := func(rule pegRule, begin int) {
		if t := tree.Expand(tokenIndex); t != nil {
			tree = t
		}
		tree.Add(rule, begin, position, depth, tokenIndex)
		tokenIndex++
	}

	matchDot := func() bool {
		if buffer[position] != end_symbol {
			position++
			return true
		}
		return false
	}

	/*matchChar := func(c byte) bool {
		if buffer[position] == c {
			position++
			return true
		}
		return false
	}*/

	/*matchRange := func(lower byte, upper byte) bool {
		if c := buffer[position]; c >= lower && c <= upper {
			position++
			return true
		}
		return false
	}*/

	rules = [...]func() bool{
		nil,
		/* 0 start <- <(line* _ !.)> */
		func() bool {
			position0, tokenIndex0, depth0 := position, tokenIndex, depth
			{
				position1 := position
				depth++
			l2:
				{
					position3, tokenIndex3, depth3 := position, tokenIndex, depth
					{
						position4 := position
						depth++
						{
							position5, tokenIndex5, depth5 := position, tokenIndex, depth
							if !rules[rule_]() {
								goto l6
							}
							{
								position7, tokenIndex7, depth7 := position, tokenIndex, depth
								if buffer[position] != rune('e') {
									goto l8
								}
								position++
								goto l7
							l8:
								position, tokenIndex, depth = position7, tokenIndex7, depth7
								if buffer[position] != rune('E') {
									goto l6
								}
								position++
							}
						l7:
							{
								position9, tokenIndex9, depth9 := position, tokenIndex, depth
								if buffer[position] != rune('x') {
									goto l10
								}
								position++
								goto l9
							l10:
								position, tokenIndex, depth = position9, tokenIndex9, depth9
								if buffer[position] != rune('X') {
									goto l6
								}
								position++
							}
						l9:
							{
								position11, tokenIndex11, depth11 := position, tokenIndex, depth
								if buffer[position] != rune('p') {
									goto l12
								}
								position++
								goto l11
							l12:
								position, tokenIndex, depth = position11, tokenIndex11, depth11
								if buffer[position] != rune('P') {
									goto l6
								}
								position++
							}
						l11:
							{
								position13, tokenIndex13, depth13 := position, tokenIndex, depth
								if buffer[position] != rune('o') {
									goto l14
								}
								position++
								goto l13
							l14:
								position, tokenIndex, depth = position13, tokenIndex13, depth13
								if buffer[position] != rune('O') {
									goto l6
								}
								position++
							}
						l13:
							{
								position15, tokenIndex15, depth15 := position, tokenIndex, depth
								if buffer[position] != rune('r') {
									goto l16
								}
								position++
								goto l15
							l16:
								position, tokenIndex, depth = position15, tokenIndex15, depth15
								if buffer[position] != rune('R') {
									goto l6
								}
								position++
							}
						l15:
							{
								position17, tokenIndex17, depth17 := position, tokenIndex, depth
								if buffer[position] != rune('t') {
									goto l18
								}
								position++
								goto l17
							l18:
								position, tokenIndex, depth = position17, tokenIndex17, depth17
								if buffer[position] != rune('T') {
									goto l6
								}
								position++
							}
						l17:
							if buffer[position] != rune('=') {
								goto l6
							}
							position++
							{
								position21, tokenIndex21, depth21 := position, tokenIndex, depth
								if c := buffer[position]; c < rune('A') || c > rune('Z') {
									goto l22
								}
								position++
								goto l21
							l22:
								position, tokenIndex, depth = position21, tokenIndex21, depth21
								if c := buffer[position]; c < rune('a') || c > rune('z') {
									goto l23
								}
								position++
								goto l21
							l23:
								position, tokenIndex, depth = position21, tokenIndex21, depth21
								if buffer[position] != rune('.') {
									goto l24
								}
								position++
								goto l21
							l24:
								position, tokenIndex, depth = position21, tokenIndex21, depth21
								if c := buffer[position]; c < rune('0') || c > rune('9') {
									goto l25
								}
								position++
								goto l21
							l25:
								position, tokenIndex, depth = position21, tokenIndex21, depth21
								if buffer[position] != rune('_') {
									goto l6
								}
								position++
							}
						l21:
						l19:
							{
								position20, tokenIndex20, depth20 := position, tokenIndex, depth
								{
									position26, tokenIndex26, depth26 := position, tokenIndex, depth
									if c := buffer[position]; c < rune('A') || c > rune('Z') {
										goto l27
									}
									position++
									goto l26
								l27:
									position, tokenIndex, depth = position26, tokenIndex26, depth26
									if c := buffer[position]; c < rune('a') || c > rune('z') {
										goto l28
									}
									position++
									goto l26
								l28:
									position, tokenIndex, depth = position26, tokenIndex26, depth26
									if buffer[position] != rune('.') {
										goto l29
									}
									position++
									goto l26
								l29:
									position, tokenIndex, depth = position26, tokenIndex26, depth26
									if c := buffer[position]; c < rune('0') || c > rune('9') {
										goto l30
									}
									position++
									goto l26
								l30:
									position, tokenIndex, depth = position26, tokenIndex26, depth26
									if buffer[position] != rune('_') {
										goto l20
									}
									position++
								}
							l26:
								goto l19
							l20:
								position, tokenIndex, depth = position20, tokenIndex20, depth20
							}
							if buffer[position] != rune(':') {
								goto l6
							}
							position++
							{
								position33, tokenIndex33, depth33 := position, tokenIndex, depth
								if c := buffer[position]; c < rune('A') || c > rune('Z') {
									goto l34
								}
								position++
								goto l33
							l34:
								position, tokenIndex, depth = position33, tokenIndex33, depth33
								if c := buffer[position]; c < rune('0') || c > rune('9') {
									goto l35
								}
								position++
								goto l33
							l35:
								position, tokenIndex, depth = position33, tokenIndex33, depth33
								if buffer[position] != rune('_') {
									goto l6
								}
								position++
							}
						l33:
						l31:
							{
								position32, tokenIndex32, depth32 := position, tokenIndex, depth
								{
									position36, tokenIndex36, depth36 := position, tokenIndex, depth
									if c := buffer[position]; c < rune('A') || c > rune('Z') {
										goto l37
									}
									position++
									goto l36
								l37:
									position, tokenIndex, depth = position36, tokenIndex36, depth36
									if c := buffer[position]; c < rune('0') || c > rune('9') {
										goto l38
									}
									position++
									goto l36
								l38:
									position, tokenIndex, depth = position36, tokenIndex36, depth36
									if buffer[position] != rune('_') {
										goto l32
									}
									position++
								}
							l36:
								goto l31
							l32:
								position, tokenIndex, depth = position32, tokenIndex32, depth32
							}
							if !rules[rule_]() {
								goto l6
							}
							{
								position39, tokenIndex39, depth39 := position, tokenIndex, depth
								if !rules[ruleLineTerminator]() {
									goto l39
								}
								goto l40
							l39:
								position, tokenIndex, depth = position39, tokenIndex39, depth39
							}
						l40:
							goto l5
						l6:
							position, tokenIndex, depth = position5, tokenIndex5, depth5
							if !rules[rule_]() {
								goto l41
							}
							{
								position42, tokenIndex42, depth42 := position, tokenIndex, depth
								if buffer[position] != rune('i') {
									goto l43
								}
								position++
								goto l42
							l43:
								position, tokenIndex, depth = position42, tokenIndex42, depth42
								if buffer[position] != rune('I') {
									goto l41
								}
								position++
							}
						l42:
							{
								position44, tokenIndex44, depth44 := position, tokenIndex, depth
								if buffer[position] != rune('n') {
									goto l45
								}
								position++
								goto l44
							l45:
								position, tokenIndex, depth = position44, tokenIndex44, depth44
								if buffer[position] != rune('N') {
									goto l41
								}
								position++
							}
						l44:
							{
								position46, tokenIndex46, depth46 := position, tokenIndex, depth
								if buffer[position] != rune('p') {
									goto l47
								}
								position++
								goto l46
							l47:
								position, tokenIndex, depth = position46, tokenIndex46, depth46
								if buffer[position] != rune('P') {
									goto l41
								}
								position++
							}
						l46:
							{
								position48, tokenIndex48, depth48 := position, tokenIndex, depth
								if buffer[position] != rune('o') {
									goto l49
								}
								position++
								goto l48
							l49:
								position, tokenIndex, depth = position48, tokenIndex48, depth48
								if buffer[position] != rune('O') {
									goto l41
								}
								position++
							}
						l48:
							{
								position50, tokenIndex50, depth50 := position, tokenIndex, depth
								if buffer[position] != rune('r') {
									goto l51
								}
								position++
								goto l50
							l51:
								position, tokenIndex, depth = position50, tokenIndex50, depth50
								if buffer[position] != rune('R') {
									goto l41
								}
								position++
							}
						l50:
							{
								position52, tokenIndex52, depth52 := position, tokenIndex, depth
								if buffer[position] != rune('t') {
									goto l53
								}
								position++
								goto l52
							l53:
								position, tokenIndex, depth = position52, tokenIndex52, depth52
								if buffer[position] != rune('T') {
									goto l41
								}
								position++
							}
						l52:
							if buffer[position] != rune('=') {
								goto l41
							}
							position++
							{
								position54 := position
								depth++
								{
									position57, tokenIndex57, depth57 := position, tokenIndex, depth
									if c := buffer[position]; c < rune('A') || c > rune('Z') {
										goto l58
									}
									position++
									goto l57
								l58:
									position, tokenIndex, depth = position57, tokenIndex57, depth57
									if c := buffer[position]; c < rune('a') || c > rune('z') {
										goto l59
									}
									position++
									goto l57
								l59:
									position, tokenIndex, depth = position57, tokenIndex57, depth57
									if c := buffer[position]; c < rune('0') || c > rune('9') {
										goto l60
									}
									position++
									goto l57
								l60:
									position, tokenIndex, depth = position57, tokenIndex57, depth57
									if buffer[position] != rune('_') {
										goto l41
									}
									position++
								}
							l57:
							l55:
								{
									position56, tokenIndex56, depth56 := position, tokenIndex, depth
									{
										position61, tokenIndex61, depth61 := position, tokenIndex, depth
										if c := buffer[position]; c < rune('A') || c > rune('Z') {
											goto l62
										}
										position++
										goto l61
									l62:
										position, tokenIndex, depth = position61, tokenIndex61, depth61
										if c := buffer[position]; c < rune('a') || c > rune('z') {
											goto l63
										}
										position++
										goto l61
									l63:
										position, tokenIndex, depth = position61, tokenIndex61, depth61
										if c := buffer[position]; c < rune('0') || c > rune('9') {
											goto l64
										}
										position++
										goto l61
									l64:
										position, tokenIndex, depth = position61, tokenIndex61, depth61
										if buffer[position] != rune('_') {
											goto l56
										}
										position++
									}
								l61:
									goto l55
								l56:
									position, tokenIndex, depth = position56, tokenIndex56, depth56
								}
								if buffer[position] != rune('.') {
									goto l41
								}
								position++
								{
									position67, tokenIndex67, depth67 := position, tokenIndex, depth
									if c := buffer[position]; c < rune('A') || c > rune('Z') {
										goto l68
									}
									position++
									goto l67
								l68:
									position, tokenIndex, depth = position67, tokenIndex67, depth67
									if c := buffer[position]; c < rune('0') || c > rune('9') {
										goto l69
									}
									position++
									goto l67
								l69:
									position, tokenIndex, depth = position67, tokenIndex67, depth67
									if buffer[position] != rune('_') {
										goto l41
									}
									position++
								}
							l67:
							l65:
								{
									position66, tokenIndex66, depth66 := position, tokenIndex, depth
									{
										position70, tokenIndex70, depth70 := position, tokenIndex, depth
										if c := buffer[position]; c < rune('A') || c > rune('Z') {
											goto l71
										}
										position++
										goto l70
									l71:
										position, tokenIndex, depth = position70, tokenIndex70, depth70
										if c := buffer[position]; c < rune('0') || c > rune('9') {
											goto l72
										}
										position++
										goto l70
									l72:
										position, tokenIndex, depth = position70, tokenIndex70, depth70
										if buffer[position] != rune('_') {
											goto l66
										}
										position++
									}
								l70:
									goto l65
								l66:
									position, tokenIndex, depth = position66, tokenIndex66, depth66
								}
								if buffer[position] != rune(':') {
									goto l41
								}
								position++
								{
									position75, tokenIndex75, depth75 := position, tokenIndex, depth
									if c := buffer[position]; c < rune('A') || c > rune('Z') {
										goto l76
									}
									position++
									goto l75
								l76:
									position, tokenIndex, depth = position75, tokenIndex75, depth75
									if c := buffer[position]; c < rune('0') || c > rune('9') {
										goto l77
									}
									position++
									goto l75
								l77:
									position, tokenIndex, depth = position75, tokenIndex75, depth75
									if buffer[position] != rune('_') {
										goto l41
									}
									position++
								}
							l75:
							l73:
								{
									position74, tokenIndex74, depth74 := position, tokenIndex, depth
									{
										position78, tokenIndex78, depth78 := position, tokenIndex, depth
										if c := buffer[position]; c < rune('A') || c > rune('Z') {
											goto l79
										}
										position++
										goto l78
									l79:
										position, tokenIndex, depth = position78, tokenIndex78, depth78
										if c := buffer[position]; c < rune('0') || c > rune('9') {
											goto l80
										}
										position++
										goto l78
									l80:
										position, tokenIndex, depth = position78, tokenIndex78, depth78
										if buffer[position] != rune('_') {
											goto l74
										}
										position++
									}
								l78:
									goto l73
								l74:
									position, tokenIndex, depth = position74, tokenIndex74, depth74
								}
								depth--
								add(rulePegText, position54)
							}
							if !rules[rule_]() {
								goto l41
							}
							{
								position81, tokenIndex81, depth81 := position, tokenIndex, depth
								if !rules[ruleLineTerminator]() {
									goto l81
								}
								goto l82
							l81:
								position, tokenIndex, depth = position81, tokenIndex81, depth81
							}
						l82:
							{
								add(ruleAction0, position)
							}
							goto l5
						l41:
							position, tokenIndex, depth = position5, tokenIndex5, depth5
							if !rules[rule_]() {
								goto l84
							}
							{
								position85, tokenIndex85, depth85 := position, tokenIndex, depth
								if buffer[position] != rune('o') {
									goto l86
								}
								position++
								goto l85
							l86:
								position, tokenIndex, depth = position85, tokenIndex85, depth85
								if buffer[position] != rune('O') {
									goto l84
								}
								position++
							}
						l85:
							{
								position87, tokenIndex87, depth87 := position, tokenIndex, depth
								if buffer[position] != rune('u') {
									goto l88
								}
								position++
								goto l87
							l88:
								position, tokenIndex, depth = position87, tokenIndex87, depth87
								if buffer[position] != rune('U') {
									goto l84
								}
								position++
							}
						l87:
							{
								position89, tokenIndex89, depth89 := position, tokenIndex, depth
								if buffer[position] != rune('t') {
									goto l90
								}
								position++
								goto l89
							l90:
								position, tokenIndex, depth = position89, tokenIndex89, depth89
								if buffer[position] != rune('T') {
									goto l84
								}
								position++
							}
						l89:
							{
								position91, tokenIndex91, depth91 := position, tokenIndex, depth
								if buffer[position] != rune('p') {
									goto l92
								}
								position++
								goto l91
							l92:
								position, tokenIndex, depth = position91, tokenIndex91, depth91
								if buffer[position] != rune('P') {
									goto l84
								}
								position++
							}
						l91:
							{
								position93, tokenIndex93, depth93 := position, tokenIndex, depth
								if buffer[position] != rune('o') {
									goto l94
								}
								position++
								goto l93
							l94:
								position, tokenIndex, depth = position93, tokenIndex93, depth93
								if buffer[position] != rune('O') {
									goto l84
								}
								position++
							}
						l93:
							{
								position95, tokenIndex95, depth95 := position, tokenIndex, depth
								if buffer[position] != rune('r') {
									goto l96
								}
								position++
								goto l95
							l96:
								position, tokenIndex, depth = position95, tokenIndex95, depth95
								if buffer[position] != rune('R') {
									goto l84
								}
								position++
							}
						l95:
							{
								position97, tokenIndex97, depth97 := position, tokenIndex, depth
								if buffer[position] != rune('t') {
									goto l98
								}
								position++
								goto l97
							l98:
								position, tokenIndex, depth = position97, tokenIndex97, depth97
								if buffer[position] != rune('T') {
									goto l84
								}
								position++
							}
						l97:
							if buffer[position] != rune('=') {
								goto l84
							}
							position++
							{
								position99 := position
								depth++
								{
									position102, tokenIndex102, depth102 := position, tokenIndex, depth
									if c := buffer[position]; c < rune('A') || c > rune('Z') {
										goto l103
									}
									position++
									goto l102
								l103:
									position, tokenIndex, depth = position102, tokenIndex102, depth102
									if c := buffer[position]; c < rune('a') || c > rune('z') {
										goto l104
									}
									position++
									goto l102
								l104:
									position, tokenIndex, depth = position102, tokenIndex102, depth102
									if c := buffer[position]; c < rune('0') || c > rune('9') {
										goto l105
									}
									position++
									goto l102
								l105:
									position, tokenIndex, depth = position102, tokenIndex102, depth102
									if buffer[position] != rune('_') {
										goto l84
									}
									position++
								}
							l102:
							l100:
								{
									position101, tokenIndex101, depth101 := position, tokenIndex, depth
									{
										position106, tokenIndex106, depth106 := position, tokenIndex, depth
										if c := buffer[position]; c < rune('A') || c > rune('Z') {
											goto l107
										}
										position++
										goto l106
									l107:
										position, tokenIndex, depth = position106, tokenIndex106, depth106
										if c := buffer[position]; c < rune('a') || c > rune('z') {
											goto l108
										}
										position++
										goto l106
									l108:
										position, tokenIndex, depth = position106, tokenIndex106, depth106
										if c := buffer[position]; c < rune('0') || c > rune('9') {
											goto l109
										}
										position++
										goto l106
									l109:
										position, tokenIndex, depth = position106, tokenIndex106, depth106
										if buffer[position] != rune('_') {
											goto l101
										}
										position++
									}
								l106:
									goto l100
								l101:
									position, tokenIndex, depth = position101, tokenIndex101, depth101
								}
								if buffer[position] != rune('.') {
									goto l84
								}
								position++
								{
									position112, tokenIndex112, depth112 := position, tokenIndex, depth
									if c := buffer[position]; c < rune('A') || c > rune('Z') {
										goto l113
									}
									position++
									goto l112
								l113:
									position, tokenIndex, depth = position112, tokenIndex112, depth112
									if c := buffer[position]; c < rune('0') || c > rune('9') {
										goto l114
									}
									position++
									goto l112
								l114:
									position, tokenIndex, depth = position112, tokenIndex112, depth112
									if buffer[position] != rune('_') {
										goto l84
									}
									position++
								}
							l112:
							l110:
								{
									position111, tokenIndex111, depth111 := position, tokenIndex, depth
									{
										position115, tokenIndex115, depth115 := position, tokenIndex, depth
										if c := buffer[position]; c < rune('A') || c > rune('Z') {
											goto l116
										}
										position++
										goto l115
									l116:
										position, tokenIndex, depth = position115, tokenIndex115, depth115
										if c := buffer[position]; c < rune('0') || c > rune('9') {
											goto l117
										}
										position++
										goto l115
									l117:
										position, tokenIndex, depth = position115, tokenIndex115, depth115
										if buffer[position] != rune('_') {
											goto l111
										}
										position++
									}
								l115:
									goto l110
								l111:
									position, tokenIndex, depth = position111, tokenIndex111, depth111
								}
								if buffer[position] != rune(':') {
									goto l84
								}
								position++
								{
									position120, tokenIndex120, depth120 := position, tokenIndex, depth
									if c := buffer[position]; c < rune('A') || c > rune('Z') {
										goto l121
									}
									position++
									goto l120
								l121:
									position, tokenIndex, depth = position120, tokenIndex120, depth120
									if c := buffer[position]; c < rune('0') || c > rune('9') {
										goto l122
									}
									position++
									goto l120
								l122:
									position, tokenIndex, depth = position120, tokenIndex120, depth120
									if buffer[position] != rune('_') {
										goto l84
									}
									position++
								}
							l120:
							l118:
								{
									position119, tokenIndex119, depth119 := position, tokenIndex, depth
									{
										position123, tokenIndex123, depth123 := position, tokenIndex, depth
										if c := buffer[position]; c < rune('A') || c > rune('Z') {
											goto l124
										}
										position++
										goto l123
									l124:
										position, tokenIndex, depth = position123, tokenIndex123, depth123
										if c := buffer[position]; c < rune('0') || c > rune('9') {
											goto l125
										}
										position++
										goto l123
									l125:
										position, tokenIndex, depth = position123, tokenIndex123, depth123
										if buffer[position] != rune('_') {
											goto l119
										}
										position++
									}
								l123:
									goto l118
								l119:
									position, tokenIndex, depth = position119, tokenIndex119, depth119
								}
								depth--
								add(rulePegText, position99)
							}
							if !rules[rule_]() {
								goto l84
							}
							{
								position126, tokenIndex126, depth126 := position, tokenIndex, depth
								if !rules[ruleLineTerminator]() {
									goto l126
								}
								goto l127
							l126:
								position, tokenIndex, depth = position126, tokenIndex126, depth126
							}
						l127:
							{
								add(ruleAction1, position)
							}
							goto l5
						l84:
							position, tokenIndex, depth = position5, tokenIndex5, depth5
							if !rules[rulecomment]() {
								goto l129
							}
							{
								position130, tokenIndex130, depth130 := position, tokenIndex, depth
								{
									position132, tokenIndex132, depth132 := position, tokenIndex, depth
									if buffer[position] != rune('\n') {
										goto l133
									}
									position++
									goto l132
								l133:
									position, tokenIndex, depth = position132, tokenIndex132, depth132
									if buffer[position] != rune('\r') {
										goto l130
									}
									position++
								}
							l132:
								goto l131
							l130:
								position, tokenIndex, depth = position130, tokenIndex130, depth130
							}
						l131:
							goto l5
						l129:
							position, tokenIndex, depth = position5, tokenIndex5, depth5
							if !rules[rule_]() {
								goto l134
							}
							{
								position135, tokenIndex135, depth135 := position, tokenIndex, depth
								if buffer[position] != rune('\n') {
									goto l136
								}
								position++
								goto l135
							l136:
								position, tokenIndex, depth = position135, tokenIndex135, depth135
								if buffer[position] != rune('\r') {
									goto l134
								}
								position++
							}
						l135:
							goto l5
						l134:
							position, tokenIndex, depth = position5, tokenIndex5, depth5
							if !rules[rule_]() {
								goto l3
							}
							if !rules[ruleconnection]() {
								goto l3
							}
							if !rules[rule_]() {
								goto l3
							}
							{
								position137, tokenIndex137, depth137 := position, tokenIndex, depth
								if !rules[ruleLineTerminator]() {
									goto l137
								}
								goto l138
							l137:
								position, tokenIndex, depth = position137, tokenIndex137, depth137
							}
						l138:
						}
					l5:
						depth--
						add(ruleline, position4)
					}
					goto l2
				l3:
					position, tokenIndex, depth = position3, tokenIndex3, depth3
				}
				if !rules[rule_]() {
					goto l0
				}
				{
					position139, tokenIndex139, depth139 := position, tokenIndex, depth
					if !matchDot() {
						goto l139
					}
					goto l0
				l139:
					position, tokenIndex, depth = position139, tokenIndex139, depth139
				}
				depth--
				add(rulestart, position1)
			}
			return true
		l0:
			position, tokenIndex, depth = position0, tokenIndex0, depth0
			return false
		},
		/* 1 line <- <((_ (('e' / 'E') ('x' / 'X') ('p' / 'P') ('o' / 'O') ('r' / 'R') ('t' / 'T') '=') ([A-Z] / [a-z] / '.' / [0-9] / '_')+ ':' ([A-Z] / [0-9] / '_')+ _ LineTerminator?) / (_ (('i' / 'I') ('n' / 'N') ('p' / 'P') ('o' / 'O') ('r' / 'R') ('t' / 'T') '=') <(([A-Z] / [a-z] / [0-9] / '_')+ '.' ([A-Z] / [0-9] / '_')+ ':' ([A-Z] / [0-9] / '_')+)> _ LineTerminator? Action0) / (_ (('o' / 'O') ('u' / 'U') ('t' / 'T') ('p' / 'P') ('o' / 'O') ('r' / 'R') ('t' / 'T') '=') <(([A-Z] / [a-z] / [0-9] / '_')+ '.' ([A-Z] / [0-9] / '_')+ ':' ([A-Z] / [0-9] / '_')+)> _ LineTerminator? Action1) / (comment ('\n' / '\r')?) / (_ ('\n' / '\r')) / (_ connection _ LineTerminator?))> */
		nil,
		/* 2 LineTerminator <- <(_ ','? comment? ('\n' / '\r')?)> */
		func() bool {
			position141, tokenIndex141, depth141 := position, tokenIndex, depth
			{
				position142 := position
				depth++
				if !rules[rule_]() {
					goto l141
				}
				{
					position143, tokenIndex143, depth143 := position, tokenIndex, depth
					if buffer[position] != rune(',') {
						goto l143
					}
					position++
					goto l144
				l143:
					position, tokenIndex, depth = position143, tokenIndex143, depth143
				}
			l144:
				{
					position145, tokenIndex145, depth145 := position, tokenIndex, depth
					if !rules[rulecomment]() {
						goto l145
					}
					goto l146
				l145:
					position, tokenIndex, depth = position145, tokenIndex145, depth145
				}
			l146:
				{
					position147, tokenIndex147, depth147 := position, tokenIndex, depth
					{
						position149, tokenIndex149, depth149 := position, tokenIndex, depth
						if buffer[position] != rune('\n') {
							goto l150
						}
						position++
						goto l149
					l150:
						position, tokenIndex, depth = position149, tokenIndex149, depth149
						if buffer[position] != rune('\r') {
							goto l147
						}
						position++
					}
				l149:
					goto l148
				l147:
					position, tokenIndex, depth = position147, tokenIndex147, depth147
				}
			l148:
				depth--
				add(ruleLineTerminator, position142)
			}
			return true
		l141:
			position, tokenIndex, depth = position141, tokenIndex141, depth141
			return false
		},
		/* 3 comment <- <(_ '#' anychar*)> */
		func() bool {
			position151, tokenIndex151, depth151 := position, tokenIndex, depth
			{
				position152 := position
				depth++
				if !rules[rule_]() {
					goto l151
				}
				if buffer[position] != rune('#') {
					goto l151
				}
				position++
			l153:
				{
					position154, tokenIndex154, depth154 := position, tokenIndex, depth
					{
						position155 := position
						depth++
						{
							position156, tokenIndex156, depth156 := position, tokenIndex, depth
							{
								position157, tokenIndex157, depth157 := position, tokenIndex, depth
								if buffer[position] != rune('\n') {
									goto l158
								}
								position++
								goto l157
							l158:
								position, tokenIndex, depth = position157, tokenIndex157, depth157
								if buffer[position] != rune('\r') {
									goto l156
								}
								position++
							}
						l157:
							goto l154
						l156:
							position, tokenIndex, depth = position156, tokenIndex156, depth156
						}
						if !matchDot() {
							goto l154
						}
						depth--
						add(ruleanychar, position155)
					}
					goto l153
				l154:
					position, tokenIndex, depth = position154, tokenIndex154, depth154
				}
				depth--
				add(rulecomment, position152)
			}
			return true
		l151:
			position, tokenIndex, depth = position151, tokenIndex151, depth151
			return false
		},
		/* 4 connection <- <((bridge _ ('-' '>') _ connection) / bridge)> */
		func() bool {
			position159, tokenIndex159, depth159 := position, tokenIndex, depth
			{
				position160 := position
				depth++
				{
					position161, tokenIndex161, depth161 := position, tokenIndex, depth
					if !rules[rulebridge]() {
						goto l162
					}
					if !rules[rule_]() {
						goto l162
					}
					if buffer[position] != rune('-') {
						goto l162
					}
					position++
					if buffer[position] != rune('>') {
						goto l162
					}
					position++
					if !rules[rule_]() {
						goto l162
					}
					if !rules[ruleconnection]() {
						goto l162
					}
					goto l161
				l162:
					position, tokenIndex, depth = position161, tokenIndex161, depth161
					if !rules[rulebridge]() {
						goto l159
					}
				}
			l161:
				depth--
				add(ruleconnection, position160)
			}
			return true
		l159:
			position, tokenIndex, depth = position159, tokenIndex159, depth159
			return false
		},
		/* 5 bridge <- <((port _ Action2 node _ port Action3 Action4) / iip / (leftlet Action5) / (rightlet Action6))> */
		func() bool {
			position163, tokenIndex163, depth163 := position, tokenIndex, depth
			{
				position164 := position
				depth++
				{
					position165, tokenIndex165, depth165 := position, tokenIndex, depth
					if !rules[ruleport]() {
						goto l166
					}
					if !rules[rule_]() {
						goto l166
					}
					{
						add(ruleAction2, position)
					}
					if !rules[rulenode]() {
						goto l166
					}
					if !rules[rule_]() {
						goto l166
					}
					if !rules[ruleport]() {
						goto l166
					}
					{
						add(ruleAction3, position)
					}
					{
						add(ruleAction4, position)
					}
					goto l165
				l166:
					position, tokenIndex, depth = position165, tokenIndex165, depth165
					{
						position171 := position
						depth++
						if buffer[position] != rune('\'') {
							goto l170
						}
						position++
						{
							position172 := position
							depth++
						l173:
							{
								position174, tokenIndex174, depth174 := position, tokenIndex, depth
								{
									position175 := position
									depth++
									{
										position176, tokenIndex176, depth176 := position, tokenIndex, depth
										if buffer[position] != rune('\\') {
											goto l177
										}
										position++
										if buffer[position] != rune('\'') {
											goto l177
										}
										position++
										goto l176
									l177:
										position, tokenIndex, depth = position176, tokenIndex176, depth176
										{
											position178, tokenIndex178, depth178 := position, tokenIndex, depth
											if buffer[position] != rune('\'') {
												goto l178
											}
											position++
											goto l174
										l178:
											position, tokenIndex, depth = position178, tokenIndex178, depth178
										}
										if !matchDot() {
											goto l174
										}
									}
								l176:
									depth--
									add(ruleiipchar, position175)
								}
								goto l173
							l174:
								position, tokenIndex, depth = position174, tokenIndex174, depth174
							}
							depth--
							add(rulePegText, position172)
						}
						if buffer[position] != rune('\'') {
							goto l170
						}
						position++
						{
							add(ruleAction7, position)
						}
						depth--
						add(ruleiip, position171)
					}
					goto l165
				l170:
					position, tokenIndex, depth = position165, tokenIndex165, depth165
					{
						position181 := position
						depth++
						{
							position182, tokenIndex182, depth182 := position, tokenIndex, depth
							if !rules[rulenode]() {
								goto l183
							}
							if !rules[rule_]() {
								goto l183
							}
							if !rules[ruleportWithIndex]() {
								goto l183
							}
							goto l182
						l183:
							position, tokenIndex, depth = position182, tokenIndex182, depth182
							if !rules[rulenode]() {
								goto l180
							}
							if !rules[rule_]() {
								goto l180
							}
							if !rules[ruleport]() {
								goto l180
							}
						}
					l182:
						depth--
						add(ruleleftlet, position181)
					}
					{
						add(ruleAction5, position)
					}
					goto l165
				l180:
					position, tokenIndex, depth = position165, tokenIndex165, depth165
					{
						position185 := position
						depth++
						{
							position186, tokenIndex186, depth186 := position, tokenIndex, depth
							if !rules[ruleportWithIndex]() {
								goto l187
							}
							if !rules[rule_]() {
								goto l187
							}
							if !rules[rulenode]() {
								goto l187
							}
							goto l186
						l187:
							position, tokenIndex, depth = position186, tokenIndex186, depth186
							if !rules[ruleport]() {
								goto l163
							}
							if !rules[rule_]() {
								goto l163
							}
							if !rules[rulenode]() {
								goto l163
							}
						}
					l186:
						depth--
						add(rulerightlet, position185)
					}
					{
						add(ruleAction6, position)
					}
				}
			l165:
				depth--
				add(rulebridge, position164)
			}
			return true
		l163:
			position, tokenIndex, depth = position163, tokenIndex163, depth163
			return false
		},
		/* 6 leftlet <- <((node _ portWithIndex) / (node _ port))> */
		nil,
		/* 7 iip <- <('\'' <iipchar*> '\'' Action7)> */
		nil,
		/* 8 rightlet <- <((portWithIndex _ node) / (port _ node))> */
		nil,
		/* 9 node <- <(<([a-z] / [A-Z] / [0-9] / '_')+> Action8 component? Action9)> */
		func() bool {
			position192, tokenIndex192, depth192 := position, tokenIndex, depth
			{
				position193 := position
				depth++
				{
					position194 := position
					depth++
					{
						position197, tokenIndex197, depth197 := position, tokenIndex, depth
						if c := buffer[position]; c < rune('a') || c > rune('z') {
							goto l198
						}
						position++
						goto l197
					l198:
						position, tokenIndex, depth = position197, tokenIndex197, depth197
						if c := buffer[position]; c < rune('A') || c > rune('Z') {
							goto l199
						}
						position++
						goto l197
					l199:
						position, tokenIndex, depth = position197, tokenIndex197, depth197
						if c := buffer[position]; c < rune('0') || c > rune('9') {
							goto l200
						}
						position++
						goto l197
					l200:
						position, tokenIndex, depth = position197, tokenIndex197, depth197
						if buffer[position] != rune('_') {
							goto l192
						}
						position++
					}
				l197:
				l195:
					{
						position196, tokenIndex196, depth196 := position, tokenIndex, depth
						{
							position201, tokenIndex201, depth201 := position, tokenIndex, depth
							if c := buffer[position]; c < rune('a') || c > rune('z') {
								goto l202
							}
							position++
							goto l201
						l202:
							position, tokenIndex, depth = position201, tokenIndex201, depth201
							if c := buffer[position]; c < rune('A') || c > rune('Z') {
								goto l203
							}
							position++
							goto l201
						l203:
							position, tokenIndex, depth = position201, tokenIndex201, depth201
							if c := buffer[position]; c < rune('0') || c > rune('9') {
								goto l204
							}
							position++
							goto l201
						l204:
							position, tokenIndex, depth = position201, tokenIndex201, depth201
							if buffer[position] != rune('_') {
								goto l196
							}
							position++
						}
					l201:
						goto l195
					l196:
						position, tokenIndex, depth = position196, tokenIndex196, depth196
					}
					depth--
					add(rulePegText, position194)
				}
				{
					add(ruleAction8, position)
				}
				{
					position206, tokenIndex206, depth206 := position, tokenIndex, depth
					{
						position208 := position
						depth++
						if buffer[position] != rune('(') {
							goto l206
						}
						position++
						{
							position209 := position
							depth++
						l210:
							{
								position211, tokenIndex211, depth211 := position, tokenIndex, depth
								{
									position212, tokenIndex212, depth212 := position, tokenIndex, depth
									if c := buffer[position]; c < rune('a') || c > rune('z') {
										goto l213
									}
									position++
									goto l212
								l213:
									position, tokenIndex, depth = position212, tokenIndex212, depth212
									if c := buffer[position]; c < rune('A') || c > rune('Z') {
										goto l214
									}
									position++
									goto l212
								l214:
									position, tokenIndex, depth = position212, tokenIndex212, depth212
									if buffer[position] != rune('/') {
										goto l215
									}
									position++
									goto l212
								l215:
									position, tokenIndex, depth = position212, tokenIndex212, depth212
									if buffer[position] != rune('-') {
										goto l216
									}
									position++
									goto l212
								l216:
									position, tokenIndex, depth = position212, tokenIndex212, depth212
									if c := buffer[position]; c < rune('0') || c > rune('9') {
										goto l217
									}
									position++
									goto l212
								l217:
									position, tokenIndex, depth = position212, tokenIndex212, depth212
									if buffer[position] != rune('_') {
										goto l211
									}
									position++
								}
							l212:
								goto l210
							l211:
								position, tokenIndex, depth = position211, tokenIndex211, depth211
							}
							depth--
							add(rulePegText, position209)
						}
						{
							add(ruleAction10, position)
						}
						{
							position219, tokenIndex219, depth219 := position, tokenIndex, depth
							{
								position221 := position
								depth++
								if buffer[position] != rune(':') {
									goto l219
								}
								position++
								{
									position222 := position
									depth++
									{
										position225, tokenIndex225, depth225 := position, tokenIndex, depth
										if c := buffer[position]; c < rune('a') || c > rune('z') {
											goto l226
										}
										position++
										goto l225
									l226:
										position, tokenIndex, depth = position225, tokenIndex225, depth225
										if c := buffer[position]; c < rune('A') || c > rune('Z') {
											goto l227
										}
										position++
										goto l225
									l227:
										position, tokenIndex, depth = position225, tokenIndex225, depth225
										if buffer[position] != rune('/') {
											goto l228
										}
										position++
										goto l225
									l228:
										position, tokenIndex, depth = position225, tokenIndex225, depth225
										if buffer[position] != rune('=') {
											goto l229
										}
										position++
										goto l225
									l229:
										position, tokenIndex, depth = position225, tokenIndex225, depth225
										if buffer[position] != rune('_') {
											goto l230
										}
										position++
										goto l225
									l230:
										position, tokenIndex, depth = position225, tokenIndex225, depth225
										if buffer[position] != rune(',') {
											goto l231
										}
										position++
										goto l225
									l231:
										position, tokenIndex, depth = position225, tokenIndex225, depth225
										if c := buffer[position]; c < rune('0') || c > rune('9') {
											goto l219
										}
										position++
									}
								l225:
								l223:
									{
										position224, tokenIndex224, depth224 := position, tokenIndex, depth
										{
											position232, tokenIndex232, depth232 := position, tokenIndex, depth
											if c := buffer[position]; c < rune('a') || c > rune('z') {
												goto l233
											}
											position++
											goto l232
										l233:
											position, tokenIndex, depth = position232, tokenIndex232, depth232
											if c := buffer[position]; c < rune('A') || c > rune('Z') {
												goto l234
											}
											position++
											goto l232
										l234:
											position, tokenIndex, depth = position232, tokenIndex232, depth232
											if buffer[position] != rune('/') {
												goto l235
											}
											position++
											goto l232
										l235:
											position, tokenIndex, depth = position232, tokenIndex232, depth232
											if buffer[position] != rune('=') {
												goto l236
											}
											position++
											goto l232
										l236:
											position, tokenIndex, depth = position232, tokenIndex232, depth232
											if buffer[position] != rune('_') {
												goto l237
											}
											position++
											goto l232
										l237:
											position, tokenIndex, depth = position232, tokenIndex232, depth232
											if buffer[position] != rune(',') {
												goto l238
											}
											position++
											goto l232
										l238:
											position, tokenIndex, depth = position232, tokenIndex232, depth232
											if c := buffer[position]; c < rune('0') || c > rune('9') {
												goto l224
											}
											position++
										}
									l232:
										goto l223
									l224:
										position, tokenIndex, depth = position224, tokenIndex224, depth224
									}
									depth--
									add(rulePegText, position222)
								}
								{
									add(ruleAction11, position)
								}
								depth--
								add(rulecompMeta, position221)
							}
							goto l220
						l219:
							position, tokenIndex, depth = position219, tokenIndex219, depth219
						}
					l220:
						if buffer[position] != rune(')') {
							goto l206
						}
						position++
						depth--
						add(rulecomponent, position208)
					}
					goto l207
				l206:
					position, tokenIndex, depth = position206, tokenIndex206, depth206
				}
			l207:
				{
					add(ruleAction9, position)
				}
				depth--
				add(rulenode, position193)
			}
			return true
		l192:
			position, tokenIndex, depth = position192, tokenIndex192, depth192
			return false
		},
		/* 10 component <- <('(' <([a-z] / [A-Z] / '/' / '-' / [0-9] / '_')*> Action10 compMeta? ')')> */
		nil,
		/* 11 compMeta <- <(':' <([a-z] / [A-Z] / '/' / '=' / '_' / ',' / [0-9])+> Action11)> */
		nil,
		/* 12 port <- <(<([A-Z] / '.' / [0-9] / '_')+> __ Action12)> */
		func() bool {
			position243, tokenIndex243, depth243 := position, tokenIndex, depth
			{
				position244 := position
				depth++
				{
					position245 := position
					depth++
					{
						position248, tokenIndex248, depth248 := position, tokenIndex, depth
						if c := buffer[position]; c < rune('A') || c > rune('Z') {
							goto l249
						}
						position++
						goto l248
					l249:
						position, tokenIndex, depth = position248, tokenIndex248, depth248
						if buffer[position] != rune('.') {
							goto l250
						}
						position++
						goto l248
					l250:
						position, tokenIndex, depth = position248, tokenIndex248, depth248
						if c := buffer[position]; c < rune('0') || c > rune('9') {
							goto l251
						}
						position++
						goto l248
					l251:
						position, tokenIndex, depth = position248, tokenIndex248, depth248
						if buffer[position] != rune('_') {
							goto l243
						}
						position++
					}
				l248:
				l246:
					{
						position247, tokenIndex247, depth247 := position, tokenIndex, depth
						{
							position252, tokenIndex252, depth252 := position, tokenIndex, depth
							if c := buffer[position]; c < rune('A') || c > rune('Z') {
								goto l253
							}
							position++
							goto l252
						l253:
							position, tokenIndex, depth = position252, tokenIndex252, depth252
							if buffer[position] != rune('.') {
								goto l254
							}
							position++
							goto l252
						l254:
							position, tokenIndex, depth = position252, tokenIndex252, depth252
							if c := buffer[position]; c < rune('0') || c > rune('9') {
								goto l255
							}
							position++
							goto l252
						l255:
							position, tokenIndex, depth = position252, tokenIndex252, depth252
							if buffer[position] != rune('_') {
								goto l247
							}
							position++
						}
					l252:
						goto l246
					l247:
						position, tokenIndex, depth = position247, tokenIndex247, depth247
					}
					depth--
					add(rulePegText, position245)
				}
				if !rules[rule__]() {
					goto l243
				}
				{
					add(ruleAction12, position)
				}
				depth--
				add(ruleport, position244)
			}
			return true
		l243:
			position, tokenIndex, depth = position243, tokenIndex243, depth243
			return false
		},
		/* 13 portWithIndex <- <(<([A-Z] / '.' / [0-9] / '_')+> Action13 '[' <[0-9]+> Action14 ']' __)> */
		func() bool {
			position257, tokenIndex257, depth257 := position, tokenIndex, depth
			{
				position258 := position
				depth++
				{
					position259 := position
					depth++
					{
						position262, tokenIndex262, depth262 := position, tokenIndex, depth
						if c := buffer[position]; c < rune('A') || c > rune('Z') {
							goto l263
						}
						position++
						goto l262
					l263:
						position, tokenIndex, depth = position262, tokenIndex262, depth262
						if buffer[position] != rune('.') {
							goto l264
						}
						position++
						goto l262
					l264:
						position, tokenIndex, depth = position262, tokenIndex262, depth262
						if c := buffer[position]; c < rune('0') || c > rune('9') {
							goto l265
						}
						position++
						goto l262
					l265:
						position, tokenIndex, depth = position262, tokenIndex262, depth262
						if buffer[position] != rune('_') {
							goto l257
						}
						position++
					}
				l262:
				l260:
					{
						position261, tokenIndex261, depth261 := position, tokenIndex, depth
						{
							position266, tokenIndex266, depth266 := position, tokenIndex, depth
							if c := buffer[position]; c < rune('A') || c > rune('Z') {
								goto l267
							}
							position++
							goto l266
						l267:
							position, tokenIndex, depth = position266, tokenIndex266, depth266
							if buffer[position] != rune('.') {
								goto l268
							}
							position++
							goto l266
						l268:
							position, tokenIndex, depth = position266, tokenIndex266, depth266
							if c := buffer[position]; c < rune('0') || c > rune('9') {
								goto l269
							}
							position++
							goto l266
						l269:
							position, tokenIndex, depth = position266, tokenIndex266, depth266
							if buffer[position] != rune('_') {
								goto l261
							}
							position++
						}
					l266:
						goto l260
					l261:
						position, tokenIndex, depth = position261, tokenIndex261, depth261
					}
					depth--
					add(rulePegText, position259)
				}
				{
					add(ruleAction13, position)
				}
				if buffer[position] != rune('[') {
					goto l257
				}
				position++
				{
					position271 := position
					depth++
					if c := buffer[position]; c < rune('0') || c > rune('9') {
						goto l257
					}
					position++
				l272:
					{
						position273, tokenIndex273, depth273 := position, tokenIndex, depth
						if c := buffer[position]; c < rune('0') || c > rune('9') {
							goto l273
						}
						position++
						goto l272
					l273:
						position, tokenIndex, depth = position273, tokenIndex273, depth273
					}
					depth--
					add(rulePegText, position271)
				}
				{
					add(ruleAction14, position)
				}
				if buffer[position] != rune(']') {
					goto l257
				}
				position++
				if !rules[rule__]() {
					goto l257
				}
				depth--
				add(ruleportWithIndex, position258)
			}
			return true
		l257:
			position, tokenIndex, depth = position257, tokenIndex257, depth257
			return false
		},
		/* 14 anychar <- <(!('\n' / '\r') .)> */
		nil,
		/* 15 iipchar <- <(('\\' '\'') / (!'\'' .))> */
		nil,
		/* 16 _ <- <(' ' / '\t')*> */
		func() bool {
			{
				position278 := position
				depth++
			l279:
				{
					position280, tokenIndex280, depth280 := position, tokenIndex, depth
					{
						position281, tokenIndex281, depth281 := position, tokenIndex, depth
						if buffer[position] != rune(' ') {
							goto l282
						}
						position++
						goto l281
					l282:
						position, tokenIndex, depth = position281, tokenIndex281, depth281
						if buffer[position] != rune('\t') {
							goto l280
						}
						position++
					}
				l281:
					goto l279
				l280:
					position, tokenIndex, depth = position280, tokenIndex280, depth280
				}
				depth--
				add(rule_, position278)
			}
			return true
		},
		/* 17 __ <- <(' ' / '\t')+> */
		func() bool {
			position283, tokenIndex283, depth283 := position, tokenIndex, depth
			{
				position284 := position
				depth++
				{
					position287, tokenIndex287, depth287 := position, tokenIndex, depth
					if buffer[position] != rune(' ') {
						goto l288
					}
					position++
					goto l287
				l288:
					position, tokenIndex, depth = position287, tokenIndex287, depth287
					if buffer[position] != rune('\t') {
						goto l283
					}
					position++
				}
			l287:
			l285:
				{
					position286, tokenIndex286, depth286 := position, tokenIndex, depth
					{
						position289, tokenIndex289, depth289 := position, tokenIndex, depth
						if buffer[position] != rune(' ') {
							goto l290
						}
						position++
						goto l289
					l290:
						position, tokenIndex, depth = position289, tokenIndex289, depth289
						if buffer[position] != rune('\t') {
							goto l286
						}
						position++
					}
				l289:
					goto l285
				l286:
					position, tokenIndex, depth = position286, tokenIndex286, depth286
				}
				depth--
				add(rule__, position284)
			}
			return true
		l283:
			position, tokenIndex, depth = position283, tokenIndex283, depth283
			return false
		},
		nil,
		/* 20 Action0 <- <{ p.createInport(buffer[begin:end]) }> */
		nil,
		/* 21 Action1 <- <{ p.createOutport(buffer[begin:end]) }> */
		nil,
		/* 22 Action2 <- <{ p.inPort = p.port; p.inPortIndex = p.index }> */
		nil,
		/* 23 Action3 <- <{ p.outPort = p.port; p.outPortIndex = p.index }> */
		nil,
		/* 24 Action4 <- <{ p.createMiddlet() }> */
		nil,
		/* 25 Action5 <- <{ p.createLeftlet() }> */
		nil,
		/* 26 Action6 <- <{ p.createRightlet() }> */
		nil,
		/* 27 Action7 <- <{ p.iip = buffer[begin:end] }> */
		nil,
		/* 28 Action8 <- <{ p.nodeProcessName = buffer[begin:end] }> */
		nil,
		/* 29 Action9 <- <{ p.createNode() }> */
		nil,
		/* 30 Action10 <- <{ p.nodeComponentName = buffer[begin:end] }> */
		nil,
		/* 31 Action11 <- <{ p.nodeMeta = buffer[begin:end] }> */
		nil,
		/* 32 Action12 <- <{ p.port = buffer[begin:end] }> */
		nil,
		/* 33 Action13 <- <{ p.port = buffer[begin:end] }> */
		nil,
		/* 34 Action14 <- <{ p.index = buffer[begin:end] }> */
		nil,
	}
	p.rules = rules
}
