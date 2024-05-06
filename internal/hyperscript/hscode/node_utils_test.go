package hscode_test

import (
	"context"
	"testing"

	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/hyperscript/hsparse"
	"github.com/stretchr/testify/assert"
)

func TestGetTokenAtCursor(t *testing.T) {

	result, _, _ := hsparse.ParseHyperScriptProgram(context.Background(), "on click toggle .red on me")

	token, ok := hscode.GetTokenAtCursor(0, result.Tokens)
	if !assert.True(t, ok) {
		return
	}

	assert.Equal(t, hscode.Token{
		Type:   hscode.IDENTIFIER,
		Value:  "on",
		Start:  0,
		End:    2,
		Line:   1,
		Column: 1,
	}, token)

	token, ok = hscode.GetTokenAtCursor(2, result.Tokens)
	if !assert.True(t, ok) {
		return
	}

	assert.Equal(t, hscode.Token{
		Type:   hscode.IDENTIFIER,
		Value:  "on",
		Start:  0,
		End:    2,
		Line:   1,
		Column: 1,
	}, token)
}

func TestGetClosestTokenOnCursorLeftSide(t *testing.T) {

	result, _, _ := hsparse.ParseHyperScriptProgram(context.Background(), "on click toggle .red on me")

	_, ok := hscode.GetClosestTokenOnCursorLeftSide(0, result.Tokens)
	if !assert.False(t, ok) {
		return
	}

	token, ok := hscode.GetClosestTokenOnCursorLeftSide(1, result.Tokens)
	if !assert.True(t, ok) {
		return
	}

	onToken := hscode.Token{
		Type:   hscode.IDENTIFIER,
		Value:  "on",
		Start:  0,
		End:    2,
		Line:   1,
		Column: 1,
	}

	assert.Equal(t, onToken, token)

	token, ok = hscode.GetClosestTokenOnCursorLeftSide(2, result.Tokens)
	if !assert.True(t, ok) {
		return
	}

	assert.Equal(t, onToken, token)

	token, ok = hscode.GetTokenAtCursor(3, result.Tokens)
	if !assert.True(t, ok) {
		return
	}

	assert.Equal(t, hscode.Token{
		Type:   hscode.WHITESPACE,
		Value:  " ",
		Start:  2,
		End:    3,
		Line:   1,
		Column: 3,
	}, token)
}

// func TestGetNodeAtCursor(t *testing.T) {

// 	result, _, _ := hsparse.ParseHyperScriptSlow("on click toggle .red on me")

// 	node, _, _ := hscode.GetNodeAtCursor(0, result.Node)
// 	if !assert.NotZero(t, node) {
// 		return
// 	}

// 	assert.Equal(t, hscode.OnFeature, node.Type)

// 	node, _, _ = hscode.GetNodeAtCursor(2, result.Node)
// 	if !assert.NotZero(t, node) {
// 		return
// 	}

// 	assert.Equal(t, hscode.OnFeature, node.Type)
// }
