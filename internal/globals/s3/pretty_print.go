package internal

import (
	"fmt"
	"io"

	core "github.com/inoxlang/inox/internal/core"
)

func (b *Bucket) PrettyPrint(w io.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%T(...)", b)
}

func (r *GetObjectResponse) PrettyPrint(w io.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%T(...)", r)
}

func (r *PutObjectResponse) PrettyPrint(w io.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%T(...)", r)
}

func (r *GetBucketPolicyResponse) PrettyPrint(w io.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%T(%s)", r, r.s)
}

func (i *ObjectInfo) PrettyPrint(w io.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%T(key: %s, ...)", i, i.Key)
}
