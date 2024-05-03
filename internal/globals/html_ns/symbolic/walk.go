package html_ns

type TraversalAction int

const (
	ContinueTraversal TraversalAction = iota
	PruneTraversal
	StopTraversal
)

type VisitFn func(node *HTMLNode) (TraversalAction, error)

func (n *HTMLNode) Walk(preVisit, postVisit VisitFn) error {
	_, err := n.walk(preVisit, postVisit)
	return err
}

func (n *HTMLNode) walk(preVisit, postVisit VisitFn) (TraversalAction, error) {

	action, err := preVisit(n)
	if err != nil {
		return StopTraversal, err
	}

	switch action {
	case StopTraversal:
		return StopTraversal, nil
	case PruneTraversal:
		return ContinueTraversal, nil
	}

	for _, child := range n.requiredChildren {
		action, err := child.walk(preVisit, postVisit)

		if err != nil {
			return StopTraversal, err
		}

		if action == StopTraversal {
			return StopTraversal, nil
		}
	}

	if postVisit != nil {
		action, err = postVisit(n)
		if err != nil {
			return StopTraversal, err
		}

		if action == StopTraversal {
			return StopTraversal, nil
		}
	}

	return ContinueTraversal, nil
}
