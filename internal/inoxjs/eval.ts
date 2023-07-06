import * as ast from './ast.ts'
import { Context, newContext } from './context.ts'
import { NO_LOCAL_SCOPE, UNKNOWN_NODE_TYPE, fmtGlobalVarIsNotDeclared, fmtVarIsNotDeclared } from './eval_error.ts'
import {newGlobalState, GlobalState} from './global_state.ts'
import { Defer, isError } from './utils.ts'
import { Integer, NIL, Str, Value } from './value.ts'

class TreeWalkState {

    localScopeStack: Map<string, Value>[] = []
    returnValue: Value|undefined
    iterationChange = IterationChange.NoIterationChange

    constructor(readonly global: GlobalState){

    }

    get currentLocalScope(): Map<string, Value> {
        if (this.localScopeStack.length == 0) {
            throw new Error('no local scope')
        }
        return this.localScopeStack[this.localScopeStack.length-1]
    }

    pushScope() {
        this.localScopeStack.push(new Map<string, Value>())
    }

    popScope() {
        if(this.localScopeStack.length == 0){
            throw new Error(NO_LOCAL_SCOPE)
        }
        this.localScopeStack.pop()
    }
}

enum IterationChange {
    NoIterationChange, BreakIteration, ContinueIteration, PruneWalk
}

export function newTreeWalkState(){
    const globalState = newGlobalState(newContext())
    return new TreeWalkState(globalState)
}

export function treeWalkEval(node: Readonly<ast.Node>, state: TreeWalkState): Value | Error {
    const defer = new Defer()

    try {
        switch(node.type){
            case ast.NodeType.Chunk: {
                state.localScopeStack = [];
                state.pushScope()
                defer.add(() => state.popScope())

                state.returnValue = undefined
                defer.add(() => {
        			state.returnValue = undefined
                    state.iterationChange = IterationChange.NoIterationChange
                })


                if(node.statements.length == 1){
                    const res = treeWalkEval(node.statements[0], state)
                    if(isError(res)){
                        return res
                    }

                    if(state.returnValue != undefined) {
                        return state.returnValue
                    }

                    return res
                }

                for(const stmt of node.statements){
                    const res = treeWalkEval(stmt, state)
                    if(isError(res)){
                        return res
                    }

                    if(state.returnValue != undefined) {
                        return state.returnValue
                    }
                }

                return NIL
            }
            case ast.NodeType.Variable: {
                const value = state.currentLocalScope.get(node.varName)
                if(value === undefined){
                    throw new Error(fmtVarIsNotDeclared(node.varName))
                }
                return value
            }
              
            case ast.NodeType.GlobalVariable: {
                const value = state.global.globals.get(node.globalVarName)
                if(value === undefined){
                    throw new Error(fmtGlobalVarIsNotDeclared(node.globalVarName))
                }
                return value
            }
            case ast.NodeType.IdentifierLiteral: {
                let value = state.global.globals.get(node.name)
                if(value === undefined){
                    value = state.currentLocalScope.get(node.name)
                }
               
                if(value === undefined){
                    throw new Error(fmtVarIsNotDeclared(node.name))
                }

                return value
            }
            case ast.NodeType.MemberExpression:
            case ast.NodeType.IdentifierMemberExpression:
            case ast.NodeType.IndexExpression:
            case ast.NodeType.SliceExpression:
                break
            case ast.NodeType.IntegerLiteral: case ast.NodeType.QuotedStringLiteral:
                return evalSimpleValueLiteral(node)
            }
            return new Error(UNKNOWN_NODE_TYPE)
    } catch(err){
        if( !(err instanceof Error)) {
            throw err
        }
        return err
    } finally {
        defer.execute()
    }
  
}


function evalSimpleValueLiteral(n: ast.SimpleValueLiteral): Value {
    switch(n.type){
    case ast.NodeType.IntegerLiteral:
        return new Integer(n.intValue)
    case ast.NodeType.QuotedStringLiteral:
        return new Str(n.quotedStringValue)
    }
}