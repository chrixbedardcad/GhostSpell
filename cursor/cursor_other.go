//go:build !windows

package cursor

type noopIndicator struct{}

func New(_ []byte) Indicator          { return &noopIndicator{} }
func (n *noopIndicator) Show()        {}
func (n *noopIndicator) Success()     {}
func (n *noopIndicator) Error()       {}
func (n *noopIndicator) Hide()        {}
func (n *noopIndicator) Close()       {}
