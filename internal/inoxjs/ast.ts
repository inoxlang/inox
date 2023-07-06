export interface NodeSpan {
    start: number
    end: number
}

export type Node = 
    | Chunk | Variable | GlobalVariable | IdentifierLiteral | MemberExpression | IdentifierMemberExpression 
    | IndexExpression | SliceExpression | IntegerLiteral | QuotedStringLiteral

export type SimpleValueLiteral = IntegerLiteral | QuotedStringLiteral

export enum NodeType {
    Chunk, Variable, GlobalVariable, IdentifierLiteral, MemberExpression, IdentifierMemberExpression, IndexExpression, SliceExpression,
    IntegerLiteral, QuotedStringLiteral
}

export interface Chunk {
    type: NodeType.Chunk
    span?: NodeSpan
    statements: Node[]
}

export interface Variable {
    type: NodeType.Variable
    span?: NodeSpan
    varName: string
}

export interface GlobalVariable {
    type: NodeType.GlobalVariable
    span?: NodeSpan
    globalVarName: string
}

export interface IdentifierLiteral {
    type: NodeType.IdentifierLiteral
    span?: NodeSpan
    name: string
}

export interface MemberExpression {
    type: NodeType.MemberExpression
    span?: NodeSpan
    left: Node
    propertyName: IdentifierLiteral
    optional: boolean
}

export interface IdentifierMemberExpression {
    type: NodeType.IdentifierMemberExpression
    span?: NodeSpan
    left: IdentifierLiteral
    propertyNames: IdentifierLiteral[]
}

export interface IndexExpression {
    type: NodeType.IndexExpression
    span?: NodeSpan
    indexed: Node
    index: Node
}

export interface SliceExpression {
    type: NodeType.SliceExpression
    span?: NodeSpan
    indexed: Node
    index: Node
}

export interface IntegerLiteral {
    type: NodeType.IntegerLiteral
    span?: NodeSpan
    intValue: bigint
}


export interface QuotedStringLiteral {
    type: NodeType.QuotedStringLiteral
    raw: string
    quotedStringValue: string
}
