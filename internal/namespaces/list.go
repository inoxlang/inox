package namespaces

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globalnames"
	"github.com/inoxlang/inox/internal/namespaces/log_ns"
)

func AddNamespacesTo(m map[string]core.Value) {
	m[globalnames.LOG_NS] = log_ns.NAMESPACE
}
