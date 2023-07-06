import { assertEquals } from "https://deno.land/std@0.193.0/testing/asserts.ts";

import  * as ast from "./ast.ts"
import { newTreeWalkState, treeWalkEval } from "./eval.ts"
import { Integer, NIL, Str } from "./value.ts";


function setup(){
    return {
        state: newTreeWalkState(),
    }
}

Deno.test({
    name: "treeWalkEval",
    async fn(t){
        await t.step("empty", () => {
            const {state} = setup()
        
            const node: ast.Chunk = {
                type: ast.NodeType.Chunk,
                statements: []
            }
    
            
            const res = treeWalkEval(node, state)
            assertEquals(NIL, res)
        })

        await t.step("integer literal", () => {
            const {state} = setup()
        
            const node: ast.Chunk = {
                type: ast.NodeType.Chunk,
                statements: [
                    {
                        type: ast.NodeType.IntegerLiteral,
                        intValue: 1n,
                    }
                ]
            }
            
            const res = treeWalkEval(node, state)
            assertEquals(new Integer(1n), res)
        })

        await t.step("quoted string literal", () => {
            const {state} = setup()
        
            const node: ast.Chunk = {
                type: ast.NodeType.Chunk,
                statements: [
                    {
                        type: ast.NodeType.QuotedStringLiteral,
                        quotedStringValue: "a",
                        raw: "" //ignored
                    }
                ]
            }
            
            const res = treeWalkEval(node, state)
            assertEquals(new Str("a"), res)
        })
    }
})