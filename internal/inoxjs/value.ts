export abstract class Value {
    abstract equal(other: Value, alreadyCompared: Map<Value, Value>): boolean
}


export class NilT extends Value {
    equal(other: Value, alreadyCompared: Map<Value, Value>): boolean {
        return other instanceof NilT
    }
}

export const NIL = new NilT()

export class Integer extends Value {
    constructor(readonly value: bigint){
        super()
    }
    equal(other: Value, _: Map<Value, Value>): boolean {
        return (other instanceof Integer) && this.value == other.value
    }
}

export class Str extends Value {
    constructor(readonly value: string){
        super()
    }
    equal(other: Value, _: Map<Value, Value>): boolean {
        return (other instanceof Str) && this.value == other.value
    }
}


export class Path extends Value {

    constructor(readonly value: string){
        //TODO: check
        super()
    }

    equal(other: Value, _: Map<Value, Value>): boolean {
        return (other instanceof Path) && this.value == other.value
    }
}